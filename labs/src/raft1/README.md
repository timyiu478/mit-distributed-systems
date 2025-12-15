# Raft Diagram 

See [docs/raft_diagram.pdf](docs/raft_diagram.pdf)

# Visualisation of the overview of the principal components of the protocol

https://thesecretlivesofdata.com/raft/

# Implementation Tips

1. Like a request, the reply can be delayed, and the reply handler can receive a reply from a past term
1. Your code may have loops that repeatedly check for certain events. Don't have these loops execute continuously without pausing, since that will slow your implementation enough that it fails tests. Use Go's condition variables, or insert a `time.Sleep(10 * time.Millisecond)` in each loop iteration.
1. In Go, when you create a subslice from an existing slice, the subslice is not a copy of the original data. Instead, it is a new slice header that references the same underlying array as the original slice.
1. No need two goroutines sending AppendEntries: one for heartbeat (empty), one for logs. We can combine them into one goroutine to avoid double sending.
1. The `applyCh` is a unbuffered channel. When you work on Lab3D, you might encounter some bugs because of it.

# Implementation Details

## Snapshot

If the follower receives a AppendEntries RPC message from leader where the *prevLogIndex* is smaller then the followers's *lastIncludedIndex*, the follower will reject this message and tells the leader his *lastIncludedIndex*.

This enables the leader know the reason of rejecting the message is related to snapshot and trimmed log. The leader can advance the *nextIndex[followerId]* to *lastIncludedIndex + 1* to skip the log entries that included in the snapshot or send a snapshot that cover more log entries than *lastIncludedIndex*.

# Current Progress

The program can pass the 3D tests several times.

https://github.com/user-attachments/assets/5fdbd0fe-b1a7-4c9e-a904-a4431b8b1df0

