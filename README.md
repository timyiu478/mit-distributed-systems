# 6.5840: Distributed Systems

6.5840: Distributed Systems is a course offered by MIT that presents abstractions and implementation techniques for engineering distributed systems. Major topics include fault tolerance, replication, and consistency. Much of the class consists of studying and discussing case studies of distributed systems.

# Hands-On Programming Projects

My completed projects at a glance:

| # | Title | Description | Link | Tags |
| - | - | - | - | - |
| 1 | A at-most-once linearisable key-value store | A key/value server for a single machine that ensures that each *Put* operation is executed at-most-once despite network failures and that the operations are linearizable.  | [Here](labs/src/kvsrv1) | `At-most-once semantics`, `Linearisability`, `KV-Store` |
| 2 | **Raft** | A replicated state machine protocol for fault-tolerance  | [Here](labs/src/raft1) | `Consensus`, `Leader Election`, `Replicated State Machine`, `Log`, `Persistence` |

# Readings

My writeups at a glance:

| # | Title | Description | Link | Tags |
| - | - | - | - | - |
| 1| The Design of Practical System for Fault-Tolerance Virtual Machine | A state machine replication approach to replicate machine-level state for fault-tolerant VM | [Here](papers/write-ups/vm-ft-2010.md) | `Fault-Tolerance`, `Backup` |
| 2 | In Search of an Understandable Consensus Algorithm (Extended Version) | **Raft**: A consensus algorithm for managing a replicated log | [Here](papers/write-ups/raft.md) | `Consensus`, `Leader Election`, `Replicated State Machine`, `Log`, `Persistence`, `Linearisability` |
| 3 | The Google File System | One of the first distributed file systems for data-center applications such as large MapReduce jobs | [Here](papers/write-ups/gfs.md) | `File System`, `Parallel Performance`, `Fault Tolerance`, `Replication`, `Consistency` |
| 4 |  ZooKeeper: Wait-free coordination for Internet-scale systems | A storage system specialized to fault tolerant high-performance configuration management | [Here](papers/write-ups/zookeeper.md) | `Coordination Primitive`, `Configuration Management` |
| 5 | Principles of Computer System Design An Introduction - Chapter 9 | Two Phase Commit is a well known protocol to solve atomicity problem in distributed transaction | [Here](papers/write-ups/dtx.md) | `Distributed Transaction`, `Two-Phrase Commit`, `Serializability`, `Atomicity` |
