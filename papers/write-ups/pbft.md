---
title: "Practical Byzantine Fault Tolerance"
description: "First Byzantine consensus protocol that was truly practical"
tags: ["Byzantine Fault Tolerance", ""]
reference: https://pdos.csail.mit.edu/6.824/papers/castro-practicalbft.pdf
---

## Novelty

First BFT protocol that provide safety without sychrony assumption and it is practical for real system (a PBFT-replicated NFS file server had only ≈3% overhead compared to an unreplicated one).

## Assumptions

* Honest client
    * the client waits for one request to complete before sending the next one.
    * but we can allow a client to make asynchronous requests, yet preserve ordering constraints on them.
* At most *f* # of nodes can be byzantine where total # of nodes is *3f + 1*
* Permissioned setting: known and fixed set of nodes
* PKI: all replicas know the others' public keys
* Network: the messages can be delayed(finite unknown time), out-of-order, duplicated, or missed

## Guarantees

Consensus:

* Safety in asynchronous network
* Liveness in sychronous network

Client:

* client's replies are correct according to linearizability

---

## Details

### Client

* Why is the view number included in the client reply? What is the problem of the replies are coming from different view?

### Normal-Case Operation



---

## Questions

Q. Viewstamped Replication vs PBFT

The PBFT protocol is an extension of the VR protocol that can tolerate byzantine failures.

Q. Why survive *f* number of failstop failures need *2f + 1* number of replicas?

![](assets/failstop_failure_the_need_of_qc_intersection.png)

Q. How are the *pre-prepare* and *preserve* phrase used to totally order requests?

Q. Does the byzantine primary can order the client meesages however it want?


Q. Suppose that we eliminated the pre-prepare phase from the Practical BFT protocol. Instead, the primary multicasts a PREPARE,v,n,m message to the replicas, and the replicas multicast COMMIT messages and reply to the client as before. What could go wrong with this protocol? Give a short example, e.g., ``The primary sends foo, replicas 1 and 2 reply with bar...''

Q. Leader election: PBFT vs Raft

---

## Talks or Lectures

1. https://www.youtube.com/watch?v=Uj638eFIWg8
1. https://www.youtube.com/watch?v=S2Hqd7v6Xn4
1. https://www.youtube.com/watch?v=Q0xYCN-rvUs
1. https://www.youtube.com/watch?v=JEKyVMUjFPw

---

## TODO

* read section 5 to the end
