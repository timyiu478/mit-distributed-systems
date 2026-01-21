---
title: "Resilient Distributed Datasets: A Fault-Tolerant Abstraction for In-Memory Cluster Computing"
description: ""
tags: ["Distributed Datasets", "Spark", "Cluster Computing"]
reference: http://nil.csail.mit.edu/6.824/2020/papers/zaharia-spark.pdf
---

## Novelty

The first system that allows a general-purpose programming language to be used at interactive speeds for in-memory data mining on clusters

## Takeaways

## Motivations

The data analytic frameworks such as MapReduce and Dryad **lack of the abstraction of leveraging distributed memory**.
    * frameworks use distributed file system instead

What is the problem of lack of the abstraction of leveraging distributed memory? It is inefficient to reuse intermediate results across multiple computations.
    * Use case: Interactive data mining, where a user runs multiple adhoc queries on the same subset of the data

## Strengths & Weaknesses

---

## Details

### RDD

Abstraction:

* read-only
    * no re-write
    * only be created (“written”) through coarse grained transformation
* user can control the persistence and partitioning of the RDDs

How does the RDD model achieve fault-tolerance?

* linear graph + rebuild

Advantages of the RDD Mode:

* Immutability => easy to backup because the backup node does not have to worry about data consistency or becoming straggler

Applications Not Suitable for RDDs:

The applications that make asynchronous finegrained updates to shared state, such as a storage system for a web application or an incremental web crawler.


---

## Questions

Q. Why RDD provides an interface based on coarse-grained transformations(e.g. Map/Join/Filter)?

* efficiently provide fault tolerance by logging the transformations used to build a dataset (its lineage) rather than the actual data
* why efficient? the transaformation overhead may >> the overhead of log updates across machines/disks

Q. What applications can Spark support well that MapReduce/Hadoop cannot support?
