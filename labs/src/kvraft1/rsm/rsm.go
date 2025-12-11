package rsm

import (
	"fmt"
	"log"
	"sync"
	"time"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	"6.5840/raft1"
	"6.5840/raftapi"
	"6.5840/tester1"

)

const Debug = true

var useRaftStateMachine bool // to plug in another raft besided raft1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}


type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Me 				 	int
	Id 					int
	Req 				any
}

type OpRes struct {
	msg         raftapi.ApplyMsg
	res         any
}


// A server (i.e., ../server.go) that wants to replicate itself calls
// MakeRSM and must implement the StateMachine interface.  This
// interface allows the rsm package to interact with the server for
// server-specific operations: the server must implement DoOp to
// execute an operation (e.g., a Get or Put request), and
// Snapshot/Restore to snapshot and restore the server's state.
type StateMachine interface {
	DoOp(any) any
	Snapshot() []byte
	Restore([]byte)
}

type RSM struct {
	mu           sync.Mutex
	me           int
	rf           raftapi.Raft
	applyCh      chan raftapi.ApplyMsg
	maxraftstate int // snapshot if log grows this big
	sm           StateMachine
	// Your definitions here.
	opresCh      map[int]chan OpRes
	readerDead   chan struct{}
	seqNum       int
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
//
// me is the index of the current server in servers[].
//
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// The RSM should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
//
// MakeRSM() must return quickly, so it should start goroutines for
// any long-running work.
func MakeRSM(servers []*labrpc.ClientEnd, me int, persister *tester.Persister, maxraftstate int, sm StateMachine) *RSM {
	rsm := &RSM{
		me:           me,
		maxraftstate: maxraftstate,
		applyCh:      make(chan raftapi.ApplyMsg),
		sm:           sm,
		opresCh:      make(map[int]chan OpRes),
		readerDead:   make(chan struct{}),
	}
	if !useRaftStateMachine {
		rsm.rf = raft.Make(servers, me, persister, rsm.applyCh)
	}

	go rsm.reader()

	if maxraftstate > -1 {
		go rsm.snapshotter()
	}

	return rsm
}

func (rsm *RSM) Raft() raftapi.Raft {
	return rsm.rf
}

func (rsm *RSM) reader() {
	for msg := range rsm.applyCh {

		committedOp := 	msg.Command.(Op)

		if msg.CommandValid {
			DPrintf(fmt.Sprintf("RSM %d: reads applyMsg, commitIndex is %d", rsm.me, msg.CommandIndex))

			opres := rsm.sm.DoOp(committedOp.Req)

			DPrintf(fmt.Sprintf("RSM %d: did Op, commitIndex is %d", rsm.me, msg.CommandIndex))
			
			rsm.mu.Lock()
			ch, ok := rsm.opresCh[msg.CommandIndex]
			rsm.mu.Unlock()

			if ok {
				// make the "subscription" not block the apply loop
				go func(ch chan OpRes) {
					ch <- OpRes{msg, opres}
					// close channel once the submit goroutine received the opres
					close(ch)
				}(ch)
			}
		}
	}

	rsm.readerDead <- struct{}{}
}

// Submit a command to Raft, and wait for it to be committed.  It
// should return ErrWrongLeader if client should find new leader and
// try again.
func (rsm *RSM) Submit(req any) (rpc.Err, any) {

	// Submit creates an Op structure to run a command through Raft;
	// for example: op := Op{Me: rsm.me, Id: id, Req: req}, where req
	// is the argument to Submit and id is a unique id for the op.

	// your code here
	DPrintf(fmt.Sprintf("RSM %d: received req", rsm.me))

	rsm.mu.Lock()

	id := rsm.seqNum
	rsm.seqNum += 1
	op := Op{Me: rsm.me, Id: id, Req: req}

	index, term, isLeader := rsm.rf.Start(op)

	if !isLeader {
		rsm.mu.Unlock()
		return rpc.ErrWrongLeader, nil // i'm dead, try another server.
	}

	ch := make(chan OpRes)
	rsm.opresCh[index] = ch

	rsm.mu.Unlock()

	for {
		t, l := rsm.rf.GetState()
		if t != term || l != isLeader { 
			DPrintf(fmt.Sprintf("RSM %d: detected term change or leader change", rsm.me))
			go func(ch chan OpRes){
				// unblock the reader goroutine by comsuming the value from the channel
				// the reader goroutine will help to close the channel
				// this handles the situation that no Submit goroutine to cosume the
				// value passed by the reader gorountine
				<- ch
			}(ch)
			return rpc.ErrWrongLeader, nil
		}

		select {
			case opres, ok := <- ch: {
				if ok && opres.msg.CommandValid {
					if opres.msg.CommandIndex == index {
						req := opres.msg.Command.(Op)
						if req.Id == id {
							return rpc.OK, opres.res
						}
					}
				}
				return rpc.ErrWrongLeader, nil
			}
			case <- time.After(time.Duration(500) * time.Millisecond): {
				continue
			}
			case <- rsm.readerDead: {
				go func(ch chan OpRes){ 
					<- ch
				}(ch)
				return rpc.ErrWrongLeader, nil
			}
		}
	}

}

func (rsm *RSM) snapshotter() {
}
