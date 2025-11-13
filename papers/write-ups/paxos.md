---
title: "Paxos Made Simple"
description: ""
tags: ["Consensus"]
reference: http://nil.csail.mit.edu/6.824/2015/papers/paxos-simple.pdf
---

## Questions

Q. Suppose that the acceptors are A, B, and C. A and B are also proposers. How does Paxos ensure that the following sequence of events can't happen? What actually happens, and which value is ultimately chosen?

1. A sends prepare requests with proposal number 1, and gets responses from A, B, and C.
2. A sends accept(1, "foo") to A and C and gets responses from both. Because a majority accepted, A thinks that "foo" has been chosen. However, A crashes before sending an accept to B.
3. B sends prepare messages with proposal number 2, and gets responses from B and C.
4. B sends accept(2, "bar") messages to B and C and gets responses from both, so B thinks that "bar" has been chosen.

Q. Paxos vs Raft
