# Paxos

## Test Results

Video: https://ty478.wistia.com/medias/unutii77tr

Runtime Specifications:

* CPU: 2.3 GHz Quad-Core Intel Core i5
* Memory: 8 GB 2133 MHz LPDDR3
* OS: macOS 15.3.1

```
Test: Single proposer ...
  ... Passed
Test: Many proposers, same value ...
  ... Passed
Test: Many proposers, different values ...
  ... Passed
Test: Out-of-order instances ...
  ... Passed
Test: Deaf proposer ...
  ... Passed
Test: Forgetting ...
  ... Passed
Test: Lots of forgetting ...
  ... Passed
Test: Paxos frees forgotten instance memory ...
  ... Passed
Test: Paxos Max() after Done()s ...
  ... Passed
Test: RPC counts aren't too high ...
  ... Passed
Test: Many instances ...
  ... Passed
Test: Minority proposal ignored ...
  ... Passed
Test: Many instances, unreliable RPC ...
  ... Passed
Test: No decision if partitioned ...
  ... Passed
Test: Decision in majority partition ...
  ... Passed
Test: All agree after full heal ...
  ... Passed
Test: One peer switches partitions ...
  ... Passed
Test: One peer switches partitions, unreliable ...
  ... Passed
Test: Many requests, changing partitions ...
  ... Passed
PASS
ok  	lab/paxos	88.427s
```

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

## High Level Idea

How to make progress on agreement for multiple instances at the same time:

![](assets/paxos_library_how_to_start_multiple_instance_in_parallel.png)

## Limitations

* It won't cope with crashes, since it stores neither the key/value database nor the Paxos state on disk. If one of the Paxos peers crashes, it will never be re-started.
* It requires the set of servers to be fixed, so one cannot replace old servers.
* It is slow: many Paxos messages are exchanged for each client operation.
