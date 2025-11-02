# Raft Diagram 

See [docs/raft_diagram.pdf](docs/raft_diagram.pdf)

# Visualization of the overview of the principal components of the protocol

https://thesecretlivesofdata.com/raft/

# Implementation Tips

1. Like a request, the reply can be delayed, and the reply handler can receive a reply from a past term
1. To distinguish livelock(e.g. unable to elect a leader) and deadlock, you can add some debug logs. If the server is deadlocked, it cannot output the logs.
1. Don't have the loops execute continuously without pausing, since that will slow your implementation enough that it fails tests. Use Go's condition variables, or insert a `time.Sleep(10 * time.Millisecond)` in each loop iteration.
