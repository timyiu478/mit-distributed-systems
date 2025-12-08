package rsm

import (
	"sync"
	"math/rand"
	"sync/atomic"
	"time"

	"6.5840/kvsrv1/rpc"
	"6.5840/labrpc"
	"6.5840/raft1"
	"6.5840/raftapi"
	"6.5840/tester1"

)

var useRaftStateMachine bool // to plug in another raft besided raft1


type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Me 				 	int
	Id 					int64
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
	readerDead   int32
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
	}
	if !useRaftStateMachine {
		rsm.rf = raft.Make(servers, me, persister, rsm.applyCh)
	}

	go rsm.Reader()

	return rsm
}

func (rsm *RSM) Raft() raftapi.Raft {
	return rsm.rf
}

func (rsm *RSM) Reader() {
	for msg := range rsm.applyCh {
		committedOp := 	msg.Command.(Op)

		if msg.CommandValid {
			opres := rsm.sm.DoOp(committedOp.Req)
			
			rsm.mu.Lock()
			ch, ok := rsm.opresCh[msg.CommandIndex]
			rsm.mu.Unlock()

			if ok {
				ch <- OpRes{msg, opres}
				// close channel once the submit goroutine received the opres
				close(ch)
			}
		}
	}

	rsm.killReader()
}

func (rsm *RSM) killReader() {
	atomic.StoreInt32(&rsm.readerDead, 1)
}

func (rsm *RSM) readerKilled() bool {
	z := atomic.LoadInt32(&rsm.readerDead)
	return z == 1
}

// Submit a command to Raft, and wait for it to be committed.  It
// should return ErrWrongLeader if client should find new leader and
// try again.
func (rsm *RSM) Submit(req any) (rpc.Err, any) {

	// Submit creates an Op structure to run a command through Raft;
	// for example: op := Op{Me: rsm.me, Id: id, Req: req}, where req
	// is the argument to Submit and id is a unique id for the op.

	// your code here
	rsm.mu.Lock()

	id := rand.Int63()
	op := Op{Me: rsm.me, Id: id, Req: req}

	index, term, isLeader := rsm.rf.Start(op)

	if !isLeader {
		rsm.mu.Unlock()
		return rpc.ErrWrongLeader, nil // i'm dead, try another server.
	}

  rsm.opresCh[index] = make(chan OpRes)

	rsm.mu.Unlock()

	for !rsm.readerKilled() {
		t, l := rsm.rf.GetState()
		if t != term || l != isLeader { 
			return rpc.ErrWrongLeader, nil
		}

		rsm.mu.Lock()
		ch := rsm.opresCh[index]
		rsm.mu.Unlock()

		select {
			case opres := <- ch: {
				if opres.msg.CommandValid {
					if opres.msg.CommandIndex == index {
						req := opres.msg.Command.(Op)
						if req.Id == id {
							return rpc.OK, opres.res
						} else {
							return rpc.ErrWrongLeader, nil
						}
					}
				}
			}
			case <- time.After(time.Duration(10) * time.Millisecond): {
				continue
			}
		}

		break
	}

	return rpc.ErrWrongLeader, nil
}
