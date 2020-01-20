# Backoff
Discovered the github.com/cenkalti/backoff library -- an easy way to implement exponential backoff. Wrote some boiler plate code that:

 - Acquires a token
 - Refreshes it at set intervals, with a user configurable buffer
 - Handles failure modes:
     - Attempt to refresh the token with an exponential backoff strategy until it expires
     - If the token expires, lock it to prevent reads, and switch to a constant back off strategy
