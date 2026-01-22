---
title: "Resilient Distributed Datasets: A Fault-Tolerant Abstraction for In-Memory Cluster Computing"
description: "The first system that allows a general-purpose programming language to be used at interactive speeds for in-memory data mining on clusters"
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

## Weaknesses

* not suitable for real time processing
* not suitable for the applications that no RDD is ever reused

---

## Details

### RDD

Abstraction:

* read-only
    * no re-write
    * only be created (“written”) through coarse grained transformation
* user can control the persistence and partitioning of the RDDs
    * why persistence? avoid re-computation(re-transformation) from the original data

How does the RDD model achieve fault-tolerance?

* all operations are deterministic => results are re-computable
* lineage graph + rebuild

Advantages of the RDD Mode:

* Immutability => easy to backup because the backup node does not have to worry about data consistency or becoming straggler

When the computation happens?

* When the user runs the action operation.
* The transformation operations are used to build the lineage graph(the recipe of computation)

Applications Not Suitable for RDDs:

The applications that make asynchronous finegrained updates to shared state, such as a storage system for a web application or an incremental web crawler.

Five pieces of information that the interface exposes:

1. a set of partitions, which are atomic pieces of the dataset
2. a set of dependencies on parent RDDs;
    * two types of dependencies: narrow and wide
    * narrow: each partition of the parent RDD is used by **at most one** partition of the child RDD
        * e.g. filter, map
    * wide: multiple child partitions may depend on it.
        * e.g. groupByKey        
3. a function for computing the dataset based on its parents
4. metadata about its partitioning scheme
5. and data placement.

---

## Questions

Q. Why RDD provides an interface based on coarse-grained transformations(e.g. Map/Join/Filter)?

* efficiently provide fault tolerance by logging the transformations used to build a dataset (its lineage) rather than the actual data
* why efficient? the transaformation overhead may >> the overhead of log updates across machines/disks

Q. What are the benefits of classifying the dependencies into two types: narrow and wide?

The narrow dependencies allow for pipelined execution on **one cluster node**, which can compute all the parent partitions. For example, one can apply a map followed by a filter on an element-by-element basic.

Worker 1:

```
        Map     Filter
-----------------------------------------------
T1:     e1
T2:     e2       map(e1)
T3:     e3       map(e2)     filter(map(e1))
T4:     e4       map(e3)     filter(map(e2))
...
```

Worker 2:

```
        Map     Filter
-----------------------------------------------
T1:     e5
T2:     e6       map(e5)
T3:     e7       map(e6)     filter(map(e5))
T4:     e8       map(e7)     filter(map(e6))
...
```

No data is passed between worker 1 and worker 2.

In contrast, wide dependencies require data from all parent partitions to be available and to be shuffled **across the nodes** using a MapReducelike operation

Q. What applications can Spark support well that MapReduce/Hadoop cannot support?

