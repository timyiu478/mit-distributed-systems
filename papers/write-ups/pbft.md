---
title: "Practical Byzantine Fault Tolerance"
description: ""
tags: ["Byzantine Fault Tolerance"]
reference: https://pdos.csail.mit.edu/6.824/papers/castro-practicalbft.pdf
---



---

## Questions

Q. Why survive *f* number of failstop failures need *2f + 1* number of replicas?



Q. Suppose that we eliminated the pre-prepare phase from the Practical BFT protocol. Instead, the primary multicasts a PREPARE,v,n,m message to the replicas, and the replicas multicast COMMIT messages and reply to the client as before. What could go wrong with this protocol? Give a short example, e.g., ``The primary sends foo, replicas 1 and 2 reply with bar...''

---

## Talks or Lectures

1. https://www.youtube.com/watch?v=Uj638eFIWg8
1. https://www.youtube.com/watch?v=S2Hqd7v6Xn4
1. https://www.youtube.com/watch?v=Q0xYCN-rvUs
