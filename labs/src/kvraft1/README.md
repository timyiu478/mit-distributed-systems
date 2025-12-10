# The diagram of Raft interactions

See [assets/kvraft.pdf](assets/kvraft.pdf)

# Implementation Details

## The `rsm` package

The key challenge of building the `rsm` package for me is how the `Submit` goroutine **waits** for the reader goroutine to hand the `DoOp` return value, or how the reader goroutine knows when it needs to give the `DoOp` return value.

The hashmap of channels `rsm.opresCh` is the key structure for solving this challenge. The `Submit` goroutine tells it needs the response of particular command by open a channel at `rsm.opresCh[index]` where `rf.Log[index]` stores the command that it wants. When the reader goroutine receives a message(it contains command and command index `msg.CommandIndex`) from the apply channel `applyCh`, it will execute the command and then check if anyone openned the channel `rsm.opresCh[commandIndex]`. If the channel is opening, it will pass the response of the command to this channel. After the `Submit` goroutine receives this response, the reader will immediately close this channel.

## Duplicate RPC detection

http://nil.csail.mit.edu/6.5840/2023/notes/l-raft-QA.txt
