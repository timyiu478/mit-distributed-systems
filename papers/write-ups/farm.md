---
title: "No compromises: distributed transactions with consistency, availability, and performance"
description: ""
tags: ["Optimistic Concurrency Control", "Distributed Transaction"]
reference: https://pdos.csail.mit.edu/6.824/papers/farm-2015.pdf
---

## Novelty


## Overview



## The problems that the paper address

The problem of the distributed transaction implementations (that provide linearisability gaurentee) performed poorly (low performance).

## Key Insights

* Modern data centers that use RDMA and non-volatile DRAM => eliminate storage and network bottlenecks => expose **CPU bottlenecks**
* New transaction, replication, and recovery protocols to address CPU boottlenects

## Key Contributions



## Shortcoming or Flaws



---

## Details



---

## Questions


Q. Suppose there are two FaRM transactions that both increment the same object. They start at the same time and see the same initial value for the object. One transaction completely finishes committing (see Section 4 and Figure 4). Then the second transaction starts to commit. There are no failures. What is the evidence that FaRM will use to realize that it must abort the second transaction? At what point in the Section 4 / Figure 4 protocol will FaRM realize that it must abort?
