---
title: "Spanner: Google’s Globally-Distributed Database"
description: "The first system to distribute data at global scale and support externally-consistent distributed transactions."
tags: ["Database", "Distributed Transaction"]
reference: https://pdos.csail.mit.edu/6.824/papers/spanner.pdf
---

## Novelty

The first system to distribute data at global scale and support externally-consistent distributed transactions.

## Motivations of building spanner

* Bigtable can be difficult to use for some kinds of applications: those that have **complex, evolving schemas**, or those that want **strong consistency in the presence of wide-area replication**.
* Megastore is semirelational data model and it supports for synchronous replication, but its relatively **poor write throughput**.

## Strengths and Weaknesses

## Key Takeaways

---

## Details

### Spanserver Software Stack and Placement

![](assets/spanner_software_stack_and_placement.png)

### Data Model

* ~ SQL database
    * row-based
    * key-value mapping = primary-key columns -> non-primary-key columns

Schema Example:

![](assets/spanner_schema_example.png)

### TrueTime



---

## Questions

Q. What is external consistency?

It means linearizability(with respect the real time ordering) according to the paper's introduction section. If a transaction T1 commits before another transaction T2 starts, then T1’s commit timestamp is smaller than T2’s.

Q. What is zone in Spanner?

Q. How does the application control the locality of the keys?

Q. One spanserver == One replica in paxos group?

Yes.

Q. Suppose a Spanner server's TT.now() returns correct information, but the uncertainty is large. For example, suppose the absolute time is 10:15:30, and TT.now() returns the interval [10:15:20,10:15:40]. That interval is correct in that it contains the absolute time, but the error bound is 10 seconds. See Section 3 for an explanation TT.now(). What bad effect will a large error bound have on Spanner's operation? Give a specific example.

Q. An application example that use Spanner



---

## Further Study

* Why and How *Movedir* is used for adding or removing replicas to Paxos group: https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/eurosys2006.pdf
