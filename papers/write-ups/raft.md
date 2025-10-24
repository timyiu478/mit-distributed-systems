---
title: "In Search of an Understandable Consensus Algorithm (Extended Version)"
description: "WIP"
tags: ["Consensus", "State Machine Replication", "Raft"]
reference: 
---

## Key Contributions

* It produces a consensus algorithm called Raft that is more understandable than Paxos while providing a similar level of performance.
* The algorithm not just works, but also provide a obvious way to understand why it works.
* Novel features: 
    * Strong leadership: e.g. log entries are only flow from the leader to the followers.
    * Leader election: use randomized timers to elect leaders.
    * Membership changes: allows the cluster to continue operating normally during configuration changes.

## Strengths and weaknesses of Paxos

Strengths:

* In partial synchrony network, Paxos can guarantee safety and eventual liveness, and it supports change in cluster membership.
* Paxos formulation is good to prove correctness.

Weaknesses:

* Paxos is hard to understand. Based on informal surveys, even experts have trouble to understand how Paxos works.
    * e.g. why single-decree protocol works, use symmetric peer-to-peer approach as its core


## The Raft Consensus algorithm

### Guarantees

* Election Safety: at most one leader can be elected in a given term.
* Leader Append-Only: a leader never overwrites or deletes entries in its log; it only appends new entries.
* Log Matching: if two logs contain an entry with the same index and term, then the logs are identical in all preceding entries.
* Leader Completeness: if a log entry is committed in a given term, then that entry will be present in the logs of the leaders for all higher-numbered terms.
* State Machine Safety: if a server has applied a log entry to its state machine, no other server will apply a different command for the same log index.


## Questions


Q. What is configuration change in Raft? How does the membership change mechanism in Raft ensure that the cluster continues to operate normally during configuration changes?

* The configuration in Raft means the set of servers that are participanting in the consensus algorithm.

Q. Why SMR needs consensus algorithms like Raft or Paxos?

It needs them to ensure that all honest replicas agree on the same sequence of commands to execute, even in some replicas fail or message delays.

Q. Why Raft is considered more understandable than Paxos?

* problem decomposition: divided the problem into subproblems that can be solved, explained, and understood independently.
* simpilified the number of states to consider

Q. Why does the server needs the *candidate* state for electing new leader?



Q. How does Raft ensure there is at most one leader in a given term?

* Each server will vote for at most one candidate in a given term (on first come first serve basic). 
* A candidate win the election only if it receives votes from a majority of the servers.

Q. What does "Terms act as a logical clock in Raft" mean?

Q. When the term ends in Raft?

When the followers assume that the leader has crashed. The leader should send heartbeat messages periodically to the followers to prevent them from starting a new election.

Q. How does Raft handle inconsistent term numbers among servers?

* Each log entry is stored with the term number when it was received by the leader.


Q. Why a candidate recognizes the leader as legitimate when it receives an AppendEntries RPC from the leader with a term number greater than or equal to its own current term?

Q. Why does Raft use randomized timers for leader election?

* To reduce the chance of split votes(no candidates can get the majority votes), which can occur when two or more candidates repeatedly start elections at the same time.
* Because this spreads out the election timeouts across servers, making it less likely that multiple servers will time out simultaneously and start competing elections.

Q. Why does the leader need to keep track of the highest log index it knows to be committed and include it in its AppendEntries RPCs?

* This enable a consistency check on the followers: if a follower cannot find an entry at that index with the same term, it refuses the AppendEntries RPC.
    * How can the follower find the entry?
    * Why an entry need to classify to different terms?
* Then If the AppendEntries RPC is accepted by the follower, the leader knows that the follower's log is consistent up to that index.
* This consistency check preverses the Log Matching property.


Q. Suppose we have the scenario shown in the Raft paper's Figure 7: a cluster of seven servers, with the log contents shown. The first server crashes (the one at the top of the figure), and cannot be contacted. A leader election ensues. For each of the servers marked (a), (d), and (f), could that server be elected? If yes, which servers would vote for it? If no, what specific Raft mechanism(s) would prevent it from being elected?
