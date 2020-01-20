# Backoff
Playing with the github.com/cenkalti/backoff library (implementation in Go of exponential backoff). Wrote some boiler plate code that:

 - Acquires a token
 - Refreshes it at set intervals, with a user configurable buffer
 - Handles failure modes:
     - Attempts to refresh the token with an exponential backoff strategy until it expires
     - If the token expires, locks it to prevent further reads, and switches to a constant back off strategy
