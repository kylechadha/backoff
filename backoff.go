package backoff

import (
	"errors"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/inconshreveable/log15"
)

var ErrShutdown = errors.New("TokenRefresher closing")

type TokenRefresher interface {
	GetToken() (string, error)
	Refresh()
	Close() error
}

type TokenRetriever interface {
	RetrieveToken() (token string, expiresIn time.Duration, err error)
}

// tokenRefresher manages refreshing tokens with an exponential backoff
// strategy until the token is expired, then switches to a constant
// backoff strategy.
//
// tokenRefresher also provides a mechanism to force a refresh.
type tokenRefresher struct {
	logger        log15.Logger
	retriever     TokenRetriever
	refreshBuffer time.Duration

	closeOnce sync.Once
	done      chan struct{}
	force     chan struct{}

	mu    sync.RWMutex
	token string
}

// NewTokenRefresher creates a new default tokenRefresher.
func NewTokenRefresher(logger log15.Logger, refreshBuffer time.Duration, retriever TokenRetriever) TokenRefresher {
	m := tokenRefresher{
		logger:        logger,
		retriever:     retriever,
		refreshBuffer: refreshBuffer,
		done:          make(chan struct{}),
		force:         make(chan struct{}),
	}
	go m.refresher()

	return &m
}

// GetToken returns the stored token. If the token is invalid or expired and
// in the process of being refreshed, GetToken will block.
func (m *tokenRefresher) GetToken() (token string, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.token == "" {
		// This error is only returned when explicit cancellation occurs
		// due to service shutdown.
		err = errors.New("Token is invalid or expired")
	}
	return m.token, err
}

// Refresh requests the refresher goroutine update the token. If the refresher
// is currently updating the token, this is a no-op.
func (m *tokenRefresher) Refresh() {
	select {
	case m.force <- struct{}{}:
	default:
	}
}

// Close closes the done chan, signaling service shutdown. It can be called
// more than once.
func (m *tokenRefresher) Close() error {
	m.closeOnce.Do(func() {
		close(m.done)
	})
	return nil
}

// refresher initiates a refresh in the following three cases:
// 1) On service startup
// 2) When the existing token is about to expire
// 3) When Refresh() is called
func (m *tokenRefresher) refresher() {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Crit("Panic occurred in refresher goroutine. Restarting", "err", r)
			go m.refresher()
		}
	}()

	expWithBuffer, err := m.refresh(true)
	if err == ErrShutdown {
		return
	}
	timer := time.NewTimer(expWithBuffer)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			expWithBuffer, err := m.refresh(false)
			if err == ErrShutdown {
				return
			}
			timer.Reset(expWithBuffer)

		case <-m.force:
			expWithBuffer, err := m.refresh(true)
			if err == ErrShutdown {
				return
			}
			if !timer.Stop() {
				// If the timer hasn't been hit, we need to drain the timer chan
				// before we reset it.
				<-timer.C
			}
			timer.Reset(expWithBuffer)

		case <-m.done:
			return
		}
	}
}

// refresh manages refreshing the token. It manages token state, locking the
// token and setting the stored token to an empty string if it's no longer
// valid. This is considered to be before the first try if a force refresh
// occurred, and once the refresh buffer has elapsed in a normal timed refresh.
//
// Tokens are initially attempted to be refreshed with an exponential backoff
// strategy, which continues for refreshBuffer amount of time or until the refresh
// is successful. After refreshBuffer has elapsed, a constant backoff strategy
// is used until the refresh is successful.
//
// refresh supports explicit cancellation during shutdown. Note that if shutdown
// occurs and the token was invalid, the token will be unlocked and will remain
// set to an empty string, causing GetToken to return an error.
func (m *tokenRefresher) refresh(force bool) (expiresIn time.Duration, err error) {
	var token string
	if force {
		m.mu.Lock()
		defer m.mu.Unlock()

		token, expiresIn, err = m.retriever.RetrieveToken()
		m.setToken(token)
		if err == nil {
			return expiresIn - m.refreshBuffer, nil
		}
		m.logger.Crit("Force refresh failed", "err", err)
	}

	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = m.refreshBuffer - time.Second // Prevent a race condition between the token expiring and acquiring the lock.
	token, expiresIn, err = m.refreshInner(eb, m.done)
	if err != nil {
		if err == ErrShutdown {
			return 0, err
		}
		if !force {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.setToken(token)
			m.logger.Crit("Could not refresh token within refresh buffer. Stored token is now expired", "err", err)
		}

		token, expiresIn, err = m.refreshInner(backoff.NewConstantBackOff(1*time.Minute), m.done) // TODO: Probably want the interval to be configurable
		if err == ErrShutdown {
			return 0, err
		}
	}

	m.setToken(token)
	return expiresIn - m.refreshBuffer, nil
}

// refreshInner calls the Retriever whenever the backoff ticker ticks. If
// an exponential strategy is used and the max elapsed time is hit, note
// that the ticker channel will be closed and the last error returned by
// the Retriever will be returned from this function.
//
// refreshInner also supports explicit cancellation via signaling on the
// done chan.
func (m *tokenRefresher) refreshInner(b backoff.BackOff, done <-chan struct{}) (token string, expiresIn time.Duration, err error) {
	ticker := backoff.NewTicker(b)

Loop:
	for {
		select {
		case _, ok := <-ticker.C:
			if !ok { // Max elapsed time has been hit (only applies to the exponential backoff strategy).
				break Loop
			}
			token, expiresIn, rErr := m.retriever.RetrieveToken()
			if rErr != nil {
				err = rErr
				m.logger.Error("Failed to refresh token. Retrying...", "err", err)
				continue
			}

			ticker.Stop()
			return token, expiresIn, nil
		case <-done:
			ticker.Stop()
			return "", 0, ErrShutdown
		}
	}
	return "", 0, err
}

// setToken sets the status of the cached token.
func (m *tokenRefresher) setToken(token string) {
	m.token = token
}
