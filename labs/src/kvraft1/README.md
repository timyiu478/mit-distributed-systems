# The diagram of Raft interactions

See [assets/kvraft.pdf](assets/kvraft.pdf)

# Implementation Details

## The `rsm` package

The key challenge of building the `rsm` package for me is how the `Submit` goroutine **waits** for the reader goroutine to hand the `DoOp` return value, or how the reader goroutine knows when it needs to give the `DoOp` return value.

The hashmap of 1-size buffered channels `rsm.opresCh` is the key structure for solving this challenge. The `Submit` goroutine tells it needs the response of a particular command by opening a channel at `rsm.opresCh[index]` where `rf.Log[index]` stores the command that it wants. When the reader goroutine receives a message(it contains a command and a command index `msg.CommandIndex`) from the apply channel `applyCh`, it will execute the command and then check if anyone opened the channel `rsm.opresCh[commandIndex]`. If the channel is open, it will pass the response of the command to this channel. Then the reader will immediately close this channel.

## Duplicate RPC detection

It is possible that the raft log stores duplicated client requests when the server is restarted and replaying the committed log entries while receiving duplicated client requests. This is because the server's duplicated table is restoring via re-applying the commands, and it does not reflect the latest sequence number the client submitted.

Ref: http://nil.csail.mit.edu/6.5840/2023/notes/l-raft-QA.txt

# Current Progress

Can pass 4C tests > 10 times.

https://github.com/user-attachments/assets/abd71051-9312-47b4-9910-ae7c8a5e334b






