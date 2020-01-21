package backoff

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/inconshreveable/log15"
)

var mockRetrieverErr = errors.New("There was an error")

type mockRetriever struct {
	called int

	numFails       int
	permanentFail  bool
	incrementToken bool
	token          string
	expiresIn      time.Duration
}

func (r *mockRetriever) RetrieveToken() (token string, expiresIn time.Duration, err error) {
	r.called++
	if r.numFails > 0 || r.permanentFail {
		r.numFails--
		return "", 0, mockRetrieverErr
	}

	token = r.token
	if r.incrementToken {
		token = r.token + fmt.Sprintf("%02d", r.called)
	}
	return token, r.expiresIn, nil
}

func TestGetToken(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantErr := error(nil)
	wantToken := "CachedToken123"

	// Define tokenRefresher service.
	m := tokenRefresher{
		logger: log15.New("global", "backoff_test"),
		done:   make(chan struct{}),
		force:  make(chan struct{}),
		token:  wantToken,
	}

	// Test the results.
	gotToken, gotErr := m.GetToken()

	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if gotToken != wantToken {
		t.Errorf("The expected token was not returned. Want '%v', Got '%v'", wantToken, gotToken)
	}
}

func TestGetTokenError(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantErr := errors.New("Token is invalid or expired")
	wantToken := ""

	// Define tokenRefresher service.
	m := tokenRefresher{
		logger: log15.New("global", "backoff_test"),
		done:   make(chan struct{}),
		force:  make(chan struct{}),
		token:  wantToken,
	}

	// Test the results.
	gotToken, gotErr := m.GetToken()

	if gotErr.Error() != wantErr.Error() {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if gotToken != wantToken {
		t.Errorf("The expected token was not returned. Want '%v', Got '%v'", wantToken, gotToken)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantErr := error(nil)

	// Define tokenRefresher service.
	m := tokenRefresher{
		logger: log15.New("global", "backoff_test"),
		done:   make(chan struct{}),
		force:  make(chan struct{}),
	}

	// Test the results.
	gotErr := m.Close()

	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
}

func TestCloseTwice(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantErr := error(nil)

	// Define tokenRefresher service.
	m := tokenRefresher{
		logger: log15.New("global", "backoff_test"),
		done:   make(chan struct{}),
		force:  make(chan struct{}),
	}

	// Test the results.
	gotErr := m.Close()
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}

	gotErr = m.Close()
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
}

func TestRefreshInnerConstant(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantToken := "newToken123"
	wantExpiresIn := time.Second * 3600
	wantErr := error(nil)
	wantCalled := 4

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  3,
		token:     wantToken,
		expiresIn: wantExpiresIn,
	}
	m := tokenRefresher{
		logger:    log15.New("global", "backoff_test"),
		done:      make(chan struct{}),
		force:     make(chan struct{}),
		retriever: &retriever,
	}

	// Test the results.
	gotToken, gotExpiresIn, gotErr := m.refreshInner(backoff.NewConstantBackOff(1*time.Microsecond), m.done)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, gotToken)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}
}

func TestRefreshInnerExponential(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantToken := "newToken123"
	wantExpiresIn := time.Second * 3600
	wantErr := error(nil)
	wantCalled := 4

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  3,
		token:     wantToken,
		expiresIn: wantExpiresIn,
	}
	m := tokenRefresher{
		logger:    log15.New("global", "backoff_test"),
		done:      make(chan struct{}),
		force:     make(chan struct{}),
		retriever: &retriever,
	}

	// Test the results.
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 20 * time.Second
	gotToken, gotExpiresIn, gotErr := m.refreshInner(eb, m.done)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, gotToken)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}
}

func TestRefreshInnerExponentialMaxElapsed(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantToken := ""
	wantExpiresIn := time.Duration(0)
	wantErr := mockRetrieverErr

	// Define tokenRefresher service.
	retriever := mockRetriever{
		permanentFail: true,
		token:         "NeverReturnedToken",
		expiresIn:     time.Second * 3600,
	}
	m := tokenRefresher{
		logger:    log15.New("global", "backoff_test"),
		done:      make(chan struct{}),
		force:     make(chan struct{}),
		retriever: &retriever,
	}

	// Test the results.
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 1 * time.Second
	gotToken, gotExpiresIn, gotErr := m.refreshInner(eb, m.done)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, gotToken)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
}

func TestRefreshInnerCancellation(t *testing.T) {
	t.Parallel()

	// Define expectations.
	wantToken := ""
	wantExpiresIn := time.Duration(0)
	wantErr := ErrShutdown

	// Define tokenRefresher service.
	retriever := mockRetriever{
		permanentFail: true,
		token:         "NeverReturnedToken",
		expiresIn:     time.Second * 3600,
	}
	m := tokenRefresher{
		logger:    log15.New("global", "backoff_test"),
		done:      make(chan struct{}),
		force:     make(chan struct{}),
		retriever: &retriever,
	}

	// Test the results.
	go func() {
		time.Sleep(5 * time.Millisecond)
		m.Close()
	}()
	gotToken, gotExpiresIn, gotErr := m.refreshInner(backoff.NewConstantBackOff(1*time.Millisecond), m.done)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, gotToken)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
}

func TestRefreshInternalForceImmediateSuccess(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantToken := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)
	wantCalled := 1

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  0,
		token:     wantToken,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	gotExpiresIn, gotErr := m.refresh(true)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
}

func TestRefreshInternalForceExponentialSuccess(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantToken := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)
	wantCalled := 2

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  1,
		token:     wantToken,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	gotExpiresIn, gotErr := m.refresh(true)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
}

func TestRefreshInternalForceExponentialShutdown(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantToken := ""
	wantExpiresIn := time.Duration(0)
	wantErr := ErrShutdown
	wantGetTokenErr := errors.New("Token is invalid or expired")

	// Define tokenRefresher service.
	retriever := mockRetriever{
		permanentFail: true,
		token:         wantToken,
		expiresIn:     expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	go func() {
		time.Sleep(time.Second)
		m.Close()
	}()
	gotExpiresIn, gotErr := m.refresh(true)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}

	gotToken, gotGetTokenErr := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotGetTokenErr.Error() != wantGetTokenErr.Error() {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantGetTokenErr, gotGetTokenErr)
	}
}

func TestRefreshInternalForceConstantSuccess(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := time.Millisecond // Will give us a negative max elapsed time, causing the exponential backoff to stop after the first (guaranteed) tick.
	wantToken := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)
	wantCalled := 3

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  2,
		token:     wantToken,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	gotExpiresIn, gotErr := m.refresh(true)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
}

func TestRefreshInternalConstantShutdown(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := time.Millisecond // Will give us a negative max elapsed time, causing the exponential backoff to stop after the first (guaranteed) tick.
	wantToken := ""
	wantExpiresIn := time.Duration(0)
	wantErr := ErrShutdown
	wantCalled := 3
	wantGetTokenErr := errors.New("Token is invalid or expired")

	// Define tokenRefresher service.
	retriever := mockRetriever{
		permanentFail: true,
		token:         wantToken,
		expiresIn:     expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	go func() {
		time.Sleep(time.Second)
		m.Close()
	}()
	gotExpiresIn, gotErr := m.refresh(true)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}

	gotToken, gotGetTokenErr := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotGetTokenErr.Error() != wantGetTokenErr.Error() {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantGetTokenErr, gotGetTokenErr)
	}
}

func TestRefreshInternalExponentialSuccess(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantToken := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)
	wantCalled := 1

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  0,
		token:     wantToken,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	gotExpiresIn, gotErr := m.refresh(false)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
}

func TestRefreshInternalExponentialShutdown(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantToken := ""
	wantExpiresIn := time.Duration(0)
	wantErr := ErrShutdown
	wantGetTokenErr := errors.New("Token is invalid or expired")

	// Define tokenRefresher service.
	retriever := mockRetriever{
		permanentFail: true,
		token:         wantToken,
		expiresIn:     expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	go func() {
		time.Sleep(time.Second)
		m.Close()
	}()
	gotExpiresIn, gotErr := m.refresh(false)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}

	gotToken, gotGetTokenErr := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotGetTokenErr.Error() != wantGetTokenErr.Error() {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantGetTokenErr, gotGetTokenErr)
	}
}

func TestRefreshInternalConstantSuccess(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := time.Millisecond // Will give us a negative max elapsed time, causing the exponential backoff to stop after the first (guaranteed) tick.
	wantToken := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)
	wantCalled := 2

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  1,
		token:     wantToken,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	gotExpiresIn, gotErr := m.refresh(false)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
}

func TestGetTokenLockedDuringForce(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantTokenInitial := ""
	wantTokenFinal := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  3,
		token:     wantTokenFinal,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	go func() {
		time.Sleep(time.Millisecond * 200)
		if m.token != wantTokenInitial {
			t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenInitial, m.token)
		}
		gotToken, _ := m.GetToken() // This should only return once the lock is released and the token has been updated.
		if gotToken != wantTokenFinal {
			t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenFinal, m.token)
		}
	}()
	gotExpiresIn, gotErr := m.refresh(true)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantTokenFinal {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenFinal, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantTokenFinal {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenFinal, m.token)
	}
}

func TestGetTokenNotLockedDuringTimed(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantTokenInitial := "cachedToken123"
	wantTokenFinal := "newToken123"
	wantExpiresIn := expiresIn - refreshBuffer
	wantErr := error(nil)

	// Define tokenRefresher service.
	retriever := mockRetriever{
		numFails:  3,
		token:     wantTokenFinal,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
		token:         wantTokenInitial,
	}

	// Test the results.
	go func() {
		time.Sleep(time.Millisecond * 200)
		if m.token != wantTokenInitial {
			t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenInitial, m.token)
		}
		gotToken, _ := m.GetToken() // This should return immediately because the previous token hasn't expired yet (timed refresh, during exponential phase).
		if gotToken != wantTokenInitial {
			t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenInitial, m.token)
		}
	}()
	gotExpiresIn, gotErr := m.refresh(false)
	if gotErr != wantErr {
		t.Errorf("An unexpected error occurred. Want '%v', Got '%v'", wantErr, gotErr)
	}
	if m.token != wantTokenFinal {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenFinal, m.token)
	}
	if gotExpiresIn != wantExpiresIn {
		t.Errorf("An unexpected expiresIn was returned. Want '%v', Got '%v'", wantExpiresIn, gotExpiresIn)
	}

	gotToken, _ := m.GetToken() // Confirms the lock is unlocked.
	if gotToken != wantTokenFinal {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenFinal, m.token)
	}
}

func TestTokenRefresherTimed(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := 2 * time.Second
	refreshBuffer := time.Second
	token := "newToken"
	wantTokenInitial := ""
	wantTokenStartup := "newToken01"
	wantTokenRefresh := "newToken02"
	wantCalled := 2

	// Define tokenRefresher service.
	retriever := mockRetriever{
		incrementToken: true,
		token:          token,
		expiresIn:      expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	if m.token != wantTokenInitial {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenInitial, m.token)
	}
	go m.refresher()
	time.Sleep(100 * time.Millisecond)
	if m.token != wantTokenStartup {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenStartup, m.token)
	}
	time.Sleep(1 * time.Second)
	if m.token != wantTokenRefresh {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenRefresh, m.token)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}
	m.Close()
}

func TestTokenRefresherForce(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	token := "newToken"
	wantTokenInitial := ""
	wantTokenStartup := "newToken01"
	wantTokenRefresh := "newToken02"
	wantCalled := 2

	// Define tokenRefresher service.
	retriever := mockRetriever{
		incrementToken: true,
		token:          token,
		expiresIn:      expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	if m.token != wantTokenInitial {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenInitial, m.token)
	}
	go m.refresher()
	time.Sleep(100 * time.Millisecond)
	if m.token != wantTokenStartup {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenStartup, m.token)
	}
	m.Refresh()
	time.Sleep(100 * time.Millisecond)
	if m.token != wantTokenRefresh {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantTokenRefresh, m.token)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}
	m.Close()
}

func TestTokenRefresherShutdown(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	token := "newToken"
	wantToken := ""

	// Define tokenRefresher service.
	retriever := mockRetriever{
		permanentFail: true,
		token:         token,
		expiresIn:     expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
		retriever:     &retriever,
	}

	// Test the results.
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	go m.refresher()
	m.Close()
	time.Sleep(100 * time.Millisecond)
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
}

func TestTokenRefresherRecovery(t *testing.T) {
	t.Parallel()

	// Define expectations.
	expiresIn := time.Second * 3600
	refreshBuffer := 5 * time.Minute
	wantToken := "newToken123"
	wantCalled := 1

	// Define tokenRefresher service.
	retriever := mockRetriever{
		token:     wantToken,
		expiresIn: expiresIn,
	}
	m := tokenRefresher{
		logger:        log15.New("global", "backoff_test"),
		done:          make(chan struct{}),
		force:         make(chan struct{}),
		refreshBuffer: refreshBuffer,
	}

	// Test the results.
	go m.refresher()
	time.Sleep(1 * time.Millisecond)
	m.retriever = &retriever
	time.Sleep(100 * time.Millisecond)
	if m.token != wantToken {
		t.Errorf("An unexpected token was returned. Want '%v', Got '%v'", wantToken, m.token)
	}
	if retriever.called != wantCalled {
		t.Errorf("The retriever was called an unexpected number of times. Want '%v', Got '%v'", wantCalled, retriever.called)
	}
	m.Close()
}
