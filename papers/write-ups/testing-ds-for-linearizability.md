---
title: "Testing Distributed Systems for Linearizability"
description: "Understand what is linearizability"
tags: ["Testing", "Linearizability"]
reference: https://anishathalye.com/testing-distributed-systems-for-linearizability/
---

## Questions

Q. With a linearizable key/value storage system, could two clients who issue `get()` requests for the same key at the same time receive different values? Explain why not, or how it could occur.

It could occur. The two concurrent `get()` requests can return different values if and only if a write is linearized **between their linearization points**.

```
Client A:  inv(getA)   ----- LP_A -------- resp(getA)
Client W:  inv(writeW) ------- LP_W ------- resp(writeW)
Client B:  inv(getB)   -------    LP_B ------- resp(getB)

Value visible at LP:
- LP_A sees V1
- LP_W updates to V2
- LP_B sees V2
```
