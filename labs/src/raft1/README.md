# Raft Diagram 

See [docs/raft_diagram.pdf](docs/raft_diagram.pdf)

# Visualization of the overview of the principal components of the protocol

https://thesecretlivesofdata.com/raft/

# Implementation Tips

1. Like a request, the reply can be delayed, and the reply handler can receive a reply from a past term
1. Your code may have loops that repeatedly check for certain events. Don't have these loops execute continuously without pausing, since that will slow your implementation enough that it fails tests. Use Go's condition variables, or insert a `time.Sleep(10 * time.Millisecond)` in each loop iteration.
1. In Go, when you create a subslice from an existing slice, the subslice is not a copy of the original data. Instead, it is a new slice header that references the same underlying array as the original slice.
1. No need two goroutines sending AppendEntries: one for heartbeat (empty), one for logs. We can combine them into one goroutine to avoid double sending.

# Current Progress

The program can pass the 3C tests several time.

![assets/mit-ds-raft-lab-3c.gif](assets/mit-ds-raft-lab-3c.gif)
