---
title: "In Search of an Understandable Consensus Algorithm (Extended Version)"
description: "WIP"
tags: ["Consensus", "State Machine Replication", "Raft", "Leader Election"]
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

### Assumptions

* network is unreliable: messages can be lost, delayed, duplicated, or delivered out of order.
* failure model: crash-recovery

### Guarantees

* Election Safety: at most one leader can be elected in a given term.
* **Leader** Append-Only: a leader never overwrites or deletes entries in its log; it only appends new entries.
* Log Matching: if two logs contain an entry with the same index and term, then the logs are identical in all preceding entries.
* Leader Completeness: if a log entry is committed in a given term, then that entry will be present in the logs of the leaders for all higher-numbered terms.
* State Machine Safety: if a server has applied a log entry to its state machine, no other server will apply a different command for the same log index.

### The Possible inconsistent scenarios

* A follower may be missing entries
* A follower may have extra uncommitted entries
* or both

See Figure 7 in the paper for illustration.

### Some Challenges

* Leader crashes immediately after it committed an entry but before it replicated the entry to any followers.
* Leader crashes after it replicated an entry to the majority of followers but before the entry is committed.
* Split vote during election: two or more candidates repeatedly start elections at the same time, preventing any candidate from receiving votes from a majority of the servers.

### Log Divergence 

Figure 7 walk-through: https://youtu.be/R2-9bsKmEbo?si=Kn_dFKJ3QrsBv65j&t=4509

### Election Timeout Cosiderations

* Broadcast Time <= Election Timeout <= MTBF
* If *Election Timeout < Broadcast Time*, 
* If *Election Timeout > MTBF*, 

### What are the details that not covered in the Figure 2 of the paper?

* Tips on how to implement it in a maintainable way
    * How to synchronize RPC handlers, leader election algo, and heart beat correctly for changing the server state such as current term and voteIdFor?
* We may need to store more volatile state

## Questions


Q. What is configuration change in Raft? How does the membership change mechanism in Raft ensure that the cluster continues to operate normally during configuration changes?

* The configuration in Raft means the set of servers that are participanting in the consensus algorithm.

Q. Why SMR needs consensus algorithms like Raft or Paxos?

It needs them to ensure that all honest replicas agree on the same sequence of commands to execute, even in some replicas fail or message delays.

Q. Why Raft is considered more understandable than Paxos?

* problem decomposition: divided the problem into subproblems that can be solved, explained, and understood independently.
* simpilified the number of states to consider

Q. Why does the server needs the *candidate* state for electing new leader?

* This state represents a server detect that the current leader probably failed.
* It is possible that only some servers detect the failure, while others still believe the leader is alive e.g. due to network partition.
* At *candidate* state, the server will not wait for leader messages as it does at *follower* state, but start an election to choose a new leader.

Q. How does Raft ensure there is at most one leader in a given term?

* Each server will vote for at most one candidate in a given term. 
* A candidate win the election only if it receives votes from a majority of the servers.

Q. What does "Terms act as a logical clock in Raft" mean?

Q. When the term ends in Raft?

When the followers assume that the leader has crashed. The leader should send heartbeat messages periodically to the followers to prevent them from starting a new election.

Q. What is a committed log entry in Raft?

* A leader knows that an entry from its current term is committed once that entry is stored on a majority of the servers.
* Once a majority of followers acknowledge successful replication of the entry, the leader commits the entry (marks it as committed in its log) and notifies followers of the commit via subsequent AppendEntries RPCs. Followers then commit the entry as well.
    * See Figure 2's AppendEntries RPC in the paper.
    * The RPC argument contains "leaderCommit".


Q. Why a candidate recognizes the leader as legitimate when it receives an AppendEntries RPC from the leader with a term number greater than or equal to its own current term?

Q. Why does Raft use randomized timers for leader election?

* To reduce the chance of split votes(no candidates can get the majority votes), which can occur when two or more candidates repeatedly start elections at the same time.
* Because this spreads out the election timeouts across servers, making it less likely that multiple servers will time out simultaneously and start competing elections.

Q. Why does the leader need to keep track of the highest log index it knows to be committed and include it in its AppendEntries RPCs?

* This enable a consistency check on the followers: if a follower cannot find an entry at that index with the same term, it refuses the AppendEntries RPC.
* Then if the AppendEntries RPC is accepted by the follower, the leader knows that the follower's log is consistent up to that index.
* This consistency check preverses the Log Matching property.

Q. Suppose we have the scenario shown in the Raft paper's Figure 7: a cluster of seven servers, with the log contents shown. The first server crashes (the one at the top of the figure), and cannot be contacted. A leader election ensues. For each of the servers marked (a), (d), and (f), could that server be elected? If yes, which servers would vote for it? If no, what specific Raft mechanism(s) would prevent it from being elected?

* The RequestVote RPC implements this restriction: the RPC includes information about the candidate’s log, and the voter denies its vote if its own log is more up-to-date than that of the candidate.
* Raft determines which of two logs is more up-to-date by comparing the index and term of the last entries in the logs. If the logs have last entries with different terms, then the log with the later term is more up-to-date. If the logs end with the same term, then whichever log is longer is more up-to-date.
* Assume (d), (a), and (f) all become candidates at the same time. They vote for themselves first, then send RequestVote RPCs to other servers.
* Server (d) could be elected because it has the most up-to-date log in terms of log length and the term number of the last entry among the three candidates and it can get votes from a majority of servers.
    * Servers that can vote for (d): (b), (c), (e), (d), and (g).
* Server (f): (f) only. No one would vote for it because everyone else has a more up-to-date log in terms of the last entry's term number.
* Server (a): (a), (b) and (e) can vote for it.

Q. Why candidate need to store vote on persistent storage?

* To ensure that the vote is not lost in case of a crash or restart and the candidate never change its mind after voting.
* Otherwise, a server could vote multiple times (to different server) in the same term after a crash, violating the election safety property of Raft.

