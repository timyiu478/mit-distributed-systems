# 6.5840: Distributed Systems

6.5840: Distributed Systems is a course offered by MIT that presents abstractions and implementation techniques for engineering distributed systems. Major topics include fault tolerance, replication, and consistency. Much of the class consists of studying and discussing case studies of distributed systems.

# Hands-On Programming Projects

My completed projects at a glance:

| # | Title | Description | Link | Tags |
| - | - | - | - | - |
| 1 | A at-most-once linearisable key-value store | A key/value server for a single machine that ensures that each *Put* operation is executed at-most-once despite network failures and that the operations are linearizable.  | [Here](labs/src/kvsrv1) | `At-most-once semantics`, `Linearisability`, `KV-Store` |
| 2 | Raft | A replicated state machine protocol for fault-tolerance  | [In Progress](labs/src/raft1) | `Consensus`, `Leader Election`, `Replicated State Machine`, `Log` |

# Readings

My writeups at a glance:

| # | Title | Description | Link | Tags |
| - | - | - | - | - |
| 1| The Design of Practical System for Fault-Tolerance Virtual Machine | A state machine replication approach to replicate machine-level state for fault-tolerant VM | [Here](papers/write-ups/vm-ft-2010.md) | `Fault-Tolerance`, `Backup` |
