# Backoff
Playing with the github.com/cenkalti/backoff library (a ready to use implementation of an exponential backoff algorithm in Go).

Wrote some boiler plate code that:

 - Acquires a token
 - Refreshes it at set intervals, with a user configurable buffer
 - Handles failure modes:
     - Attempts to refresh the token with an exponential backoff strategy
     - If the token expires, locks it to prevent further reads, and switches to a constant back off strategy
