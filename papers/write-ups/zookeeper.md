---
title: "ZooKeeper: Wait-free coordination for Internet-scale systems"
description: ""
tags: ["Coordination Primitive", "Configuration Management", "Atomic Broadcost Protocol"]
reference: https://pdos.csail.mit.edu/6.824/papers/zookeeper.pdf
---

## Novelty

* **Coordination Kernel**: Instead of implementign sepcific coordination needs on the server, Zookeepr expose an API that enables application developers to implement their own primitives.

## Takeaways

?? Zab: a leader-based atomic broadcast protocol to guarantee that update operations satisfy linearisability 

---

## Assumptions

* Read to Write ratio: 2:1, 100:1
* Only rely on TCP connection. No assumption on the hardware feature/network topology
* Store metadata for coordination purpose instead of general data

## Design Challenges

* How to avoid slow or faulty clients to impact negatively the performance of faster clients?

## Guarantees

* per client of FIFO execution of requests
* lineraizability of all WRITE requests that change the Zookeeper state

## Terminology

* client: a user of the Zookeeper service
* server: process providing the ZooKeeper service
* **znode**: in-memory data node in the ZooKeeper data
* **data tree**: a set of *znode* that is organized in a hierarchical namespace

---

## Questions

Q. How is the configuration used for coordination?

* The configuration is a list of operational parameters for the system process that can change dynamically.
* operational parameters: e.g. alive members of the cluster, leader of the cluster, roles of the member, lock owner

Q. Why is Zookeeper implemented using a pipelined architecture? What is a pipelined architecture?


Q. Why *data tree*?

* can run 1 Zookeeper for N applications/coordinations
* by allocating subtrees for the namespace of different applications
* and for setting access rights to those subtrees

![](assets/zookeeper_data_tree.png)

Q. One use of Zookeeper is as a fault-tolerant lock service (see the section "Simple locks" on page 6). Why isn't possible for two clients to acquire the same lock? In particular, how does Zookeeper decide if a client has failed and it can give the client's locks to other clients?
