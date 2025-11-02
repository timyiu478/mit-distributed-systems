---
title: "In Search of an Understandable Consensus Algorithm (Extended Version)"
description: "WIP"
tags: ["Consensus", "Replicated State Machine", "Raft", "Leader Election", "Log Compaction", "Linearisability"]
reference: https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf
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

* *Broadcast Time <= Election Timeout <= MTBF*
* *Broadcast Time* is the average time of sending RPCs to all servers in parallel. If *Election Timeout < Broadcast Time*, there is NO time for servers sending the reply back to the candidate. So the leader election algorithm will run forever without making any progress.
* *MTBF* is the mean time between a failure of a single server. If *Election Timeout > MTBF*, failures occur during election.


### What are the details that not covered in the Figure 2 of the paper?

* Tips on how to implement it in a maintainable way
    * How to synchronize RPC handlers, leader election algo, and heart beat correctly for changing the server state such as current term and voteIdFor?
* We may need to store more volatile state
* When to reset election timeout: 
    * receive heart beat
    * become a candidate
    * grant a vote to a candidate
        * the follower know someone is handling the leader failure
        * this is especially important in unreliable networks where it is likely that followers have different logs; in those situations, you will often end up with only a small number of servers that a majority of servers are willing to vote for. If you reset the election timer whenever someone asks you to vote for them, this makes it equally likely for a server with an outdated log to step forward as for a server with a longer log.
        * the servers with the more up-to-date logs won’t be interrupted by outdated servers’ elections, and so are more likely to complete the election and become the leader
* The leader needs to update *nextIndex[]* and *matchIndex[]* if the appendEntriesRPC succeed. But it does not tell what are the values should them update to.

### Catch up log quickly

Rejection message should include:

```
XTerm:  term in the conflicting entry (if any)
XIndex: index of first entry with that term (if any)
XLen:   log length
```

Leader Logic:

```
Case 1: leader doesn't have XTerm:
    nextIndex = XIndex
Case 2: leader has XTerm:
    nextIndex = (index of leader's last entry for XTerm) + 1
Case 3: follower's log is too short:
    nextIndex = XLen
```

Example:

```
S1: [4, 5, 5, 5, 5]
S2: [4, 6, 6, 6, 6]

* S2 is leader
* XTerm: 5
* XIndex: 2 // 1-indexed
* XLen: 6
* S2 set nextIndex = 2 because S2 doesn't have XTerm(5)
```


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

* What is a logical clock? A logical clock is a mechanism for capturing chronological and causal relationships in a distributed system.
* The "Term" helps Raft to order the event of leader election and it plays a crucial role in ensuring the safety property.
    * e.g. (1) detect stale messages (the network can send message in arbitrary order), 
    * (2) prevent confusion of older leader


Q. When the term ends in Raft?

When the followers assume that the leader has crashed. The leader should send heartbeat messages periodically to the followers to prevent them from starting a new election.

Q. What is a committed log entry in Raft?

* A leader knows that an entry from its current term is committed once that entry is stored on a majority of the servers.
* Once a majority of followers acknowledge successful replication of the entry, the leader commits the entry (marks it as committed in its log) and notifies followers of the commit via subsequent AppendEntries RPCs. Followers then commit the entry as well.
    * See Figure 2's AppendEntries RPC in the paper.
    * The RPC argument contains "leaderCommit".


Q. Why a candidate recognizes the leader as legitimate when it receives an AppendEntries RPC from the leader with a term number greater than or equal to its own current term?

* It assumes the server runs the algorithm correctly. The server will only send *AppendEntries* RPC when it is leader.
* The candidate may lose the compete of the leader election or it actually is lacking behind in terms of "term".

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
    * Servers that can vote for (d): (a), (b), (c), (e), (d), (g), and (f).
        * (a) and (f) will first transite back to follower state when they see (d)'s request vote
        * then they vote for (d)
* Server (f): (f) only. No one would vote for it because everyone else has a more up-to-date log in terms of the last entry's term number.
* Server (a): (a), (b), (e), and (f) can vote for it. So (a) can be elected.
    * network can be partitioned such that no one can see (d)

Q. Why follower/candidate need to store *vote* on persistent storage?

* To ensure that the vote is not lost in case of a crash or restart and the follower/candidate never change its mind after voting.
* Otherwise, a server could vote multiple times (to different server) in the same term after a crash, violating the election safety property of Raft.
    * Timeline: B requests vote from A, A votes for B, A crashes, A reboot, C requests vote from A, A votes for C
* The server also need to store *currentTerm* on persistent storage. Otherwise, we can't distinguish the stale leader/candidate.

Q. Why server need to store the logs on persistent storage?

* If the server does not store the logs on persistent storage, the majorithy of committed logs can be losted.

Q. Why the leader needs both *nextIndex[]* and *matchIndex[]*?

* the leader cannot calculate the commit index by *nextIndex[]* because it is optimistic
    * *nextIndex[i]* is assumed equal to *nextIndex[leader]* when initialization 
        * decrement *nextIndex[i]* when the AppendEntries RPC failed for correcting his "guess" ->
        * this may not correct!
    * so the leader needs *matchIndex[]*

Q. Why *nextIndex[]* and *matchIndex[]* need to reinitialize after election?


* The followers cannot keep *nextIndex[]* and *matchIndex[]* "in-sync" because the append entries flow is from leader to follower.
* Consider this leader sequence: [S1, S2, S1]. S1 cannot use its old *nextIndex[]* and *matchIndex[]* because S2 can overwrite some of the uncommitted entries to the followers, and then S2 crashed before committing those entries.
* If S1 does not reinitialise *matchIndex[]* after election, S1 may incorrectly commit entries that are not stored by the majority.
* Note: DO NOT reinitialize `rf.matchIndex[rf.me] = len(rf.log) -1` after election.


Q. Could a received InstallSnapshot RPC cause the state machine to go backwards in time? That is, could step 8 in Figure 13 cause the state machine to be reset so that it reflects fewer executed operations? If yes, explain how this could happen. If no, explain why it can't happen.

* No.
* The leader takes a snapshot only after an entry has been committed.
* The leader call InstallSnapshot to send snapshots to followers that are too far behind -> follower's commitIndex <= leader's commitIndex -> follower's snapshot's last included index <= leader's commitIndex.
* When the state machine apply this snapshot, it reflects only equal or more executed operations.


Q. Why the *InstallSnapshot RPC* results do not need to contain *Success* field for indicating whether the receiver accept and execute his work successfully?

* Servers normally take snapshots **independently**
    * why not always leader send the snapshots to the followers?
        * reduce the waste of bandwidth to send information that all already known by the follower
* This RPC is used for the follower that are too far behind
    * the follower needs the obtain the entries from leader
    * but the leader discarded those entries and compacted them into a snapshot
    * the follower will update it's "last log index" if it applied the snapshot successfully
* After the call of *InstallSnapshot RPC*, the leader can use the *AppendEntries RPC*'s reply to check if the follower accept and execute his work successfully

Q. How client finds the leader?

* When a client first starts up, it connects to a randomlychosen server.
* If the client’s first choice is not the leader, that server will reject the client’s request and supply information about the most recent leader it has heard from (AppendEntries requests include the network address of the leader).
* If the leader crashes, client requests will timeout; clients then try again with randomly-chosen servers.

Q. How does Raft support linearizable semantics?

* Linearizable semantics: each operation appears to execute instantaneously, exactly once, at some point between its invocation and its response
* Solutions:
    * write operation: clients assign unique serial numbers to every command
        * The state machine tracks the latest serial number processed for each client, along with the associated response
        * If state machine recieves a command whose serial number has already been executed, it responds immediately without re-executing the request

Q. The difference between the RequestVote RPC arguments and the AppendEntries RPC arguments. RequestVote has *lastLogIndex/Term*, while AppendEntries has *prevLogIndex/Term*. Are these equivalent?

* No, they are not equivalent.
* The *lastLogIndex/Term* is the candidate/requestvote request sender's last log index/term.
* The *prevLogIndex/Term* is the follower's the log index/term immediate before the start log index of the new entries sent by the leader.



---

## TODO

read section 6
