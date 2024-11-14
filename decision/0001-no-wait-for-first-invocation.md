author: Islam Shehata (@dasfmi)
date: 2024-011-14
---

# No wait for first invocation

## Background

We had a mechanism in the extension that blocks execution until we receive the first invocation. This was done to ensure that the customers can
receive logs for their first invocation. However, this was causing issues in some cases where the extension was not able to receive the first invocation
and times out. 

As this occurs more often, the invocation numbers in Axiom were significantly lower than the CloudWatch invocations.


## Decision

- Decided to remove the blocking mechanism and allow the extension to start processing logs as soon as it is invoked. This will ensure that the customers receive any incoming logs without any delay. Removal of the first invocation and the channel associated with it simplifies the extension and simplifies the workflow.
- We had a function that checks if we should flush or not based on last time of flush. I replaced this function with a simple ticker instead, the ticker will tick every 1s and will flush the logs if the buffer is not empty. This will ensure that we are not blocking the logs for a long time.
- We had two Axiom clients, one that retries and one that doesn't. I think it complicates things and we should have only one client that retries. We still need a mechanism to prevent infinite retries though, a circuit breaker or something.



