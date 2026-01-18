# Paxos

## Interfaces

Paxos:

```
px = paxos.Make(peers []string, me int)
px.Start(seq int, v interface{}) // start agreement on new instance
px.Status(seq int) (fate Fate, v interface{}) // get info about an instance
px.Done(seq int) // ok to forget all instances <= seq
px.Max() int // highest instance seq known, or -1
px.Min() int // instances before this have been forgotten
```

## Implementation Details

## Limitations

* It won't cope with crashes, since it stores neither the key/value database nor the Paxos state on disk. If one of the Paxos peers crashes, it will never be re-started.
* It requires the set of servers to be fixed, so one cannot replace old servers.
* It is slow: many Paxos messages are exchanged for each client operation.
