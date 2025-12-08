# The diagram of Raft interactions

See [assets/kvraft.pdf](assets/kvraft.pdf)

# Implementation Details

## The `rsm` package

The key challenge of building the `rsm` package for me is how the `Subm it` goroutine **waits** for the reader goroutine to hand the `DoOp` return value, or how the reader goroutine knows when it needs to give the `DoOp` return value.

The hashmap of channels `rsm.opresCh` is the key structure for solving this challenge...
