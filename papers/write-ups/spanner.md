---
title: "Spanner: Google’s Globally-Distributed Database"
description: ""
tags: []
reference: https://pdos.csail.mit.edu/6.824/papers/spanner.pdf
---

## Questions

Q. Suppose a Spanner server's TT.now() returns correct information, but the uncertainty is large. For example, suppose the absolute time is 10:15:30, and TT.now() returns the interval [10:15:20,10:15:40]. That interval is correct in that it contains the absolute time, but the error bound is 10 seconds. See Section 3 for an explanation TT.now(). What bad effect will a large error bound have on Spanner's operation? Give a specific example.
