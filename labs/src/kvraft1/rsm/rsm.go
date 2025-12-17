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

const Debug = false

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
	waitersCh    chan struct{}
	persister    *tester.Persister
	seqNum            int
	lastAppliedIndex  int
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
		waitersCh:    make(chan struct{}, 12), // control maximum number of concurrent submitters waiting the command to be committed
		persister:    persister,
		seqNum:           0,
		lastAppliedIndex: 0,
	}
	if !useRaftStateMachine {
		rsm.rf = raft.Make(servers, me, persister, rsm.applyCh)
	}

	go rsm.reader()

	return rsm
}

func (rsm *RSM) Raft() raftapi.Raft {
	return rsm.rf
}

func (rsm *RSM) reader() {
	for msg := range rsm.applyCh {

		if msg.CommandValid {
			DPrintf(fmt.Sprintf("RSM %d: reads committed operation, commitIndex is %d", rsm.me, msg.CommandIndex))
			if rsm.lastAppliedIndex >= msg.CommandIndex {
				DPrintf(fmt.Sprintf("RSM %d: ignore command because command index(%d) <= lastAppliedIndex(%d)", rsm.me, msg.CommandIndex, rsm.lastAppliedIndex))
				continue
			}

			committedOp := msg.Command.(Op)

			opres := rsm.sm.DoOp(committedOp.Req)

			rsm.lastAppliedIndex = msg.CommandIndex

			DPrintf(fmt.Sprintf("RSM %d: did Op, commitIndex is %d", rsm.me, msg.CommandIndex))

			if rsm.maxraftstate > -1 {
				rfStateSize := rsm.persister.RaftStateSize()
				if rfStateSize > rsm.maxraftstate {
					snapshot := rsm.sm.Snapshot()
					index := msg.CommandIndex
					rsm.rf.Snapshot(msg.CommandIndex, snapshot)
					DPrintf(fmt.Sprintf("RSM %d: snapshotted, last include index is %d", rsm.me, index))
				}
			}

			rsm.mu.Lock()
			ch, ok := rsm.opresCh[msg.CommandIndex]
			if ok {
				delete(rsm.opresCh, msg.CommandIndex)
				ch <- OpRes{msg, opres}
				close(ch)
				DPrintf(fmt.Sprintf("RSM %d: handled the subscription, command index is %d", rsm.me, msg.CommandIndex))
			}
			rsm.mu.Unlock()
		} else if msg.SnapshotValid {
			DPrintf(fmt.Sprintf("RSM %d: reads installsnapshot, SnapshotIndex is %d, Snapshot term is %d", rsm.me, msg.SnapshotIndex, msg.SnapshotTerm))
			if rsm.lastAppliedIndex >= msg.SnapshotIndex {
				DPrintf(fmt.Sprintf("RSM %d: ignore install snapshot because command index(%d) <= lastAppliedIndex(%d)", rsm.me, msg.CommandIndex, rsm.lastAppliedIndex))
				continue
			}
			rsm.mu.Lock()
			rsm.sm.Restore(msg.Snapshot)
			rsm.lastAppliedIndex = msg.SnapshotIndex
			for idx, ch := range rsm.opresCh {
				if idx <= msg.SnapshotIndex {
					delete(rsm.opresCh, idx)
					close(ch)
				}
			}
			rsm.mu.Unlock()
		}
	}

	DPrintf(fmt.Sprintf("RSM %d: close readerDead channel to unblock all submitters", rsm.me))
	close(rsm.readerDead)
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

	ok := true
	select {
		case _, ok = <- rsm.readerDead:
		default:
	}
	if !ok {
		DPrintf(fmt.Sprintf("RSM %d: return ErrWrongLeader since reader is dead", rsm.me))
		return rpc.ErrWrongLeader, nil
	}

	rsm.waitersCh <- struct{}{}
	defer func() {
		<- rsm.waitersCh
	}()

	rsm.mu.Lock()

	id := rsm.seqNum
	rsm.seqNum++
	op := Op{Me: rsm.me, Id: id, Req: req}

	index, term, isLeader := rsm.rf.Start(op)

	if !isLeader {
		rsm.mu.Unlock()
		DPrintf(fmt.Sprintf("RSM %d: i'm dead, try another server", rsm.me))
		return rpc.ErrWrongLeader, nil // i'm dead, try another server.
	}

	// allow reader place the single result into the channel without blocking
	// even if the consumer (Submit) hasn't started receiving yet.
	ch := make(chan OpRes, 1)
	rsm.opresCh[index] = ch

	rsm.mu.Unlock()

	DPrintf(fmt.Sprintf("RSM %d: waits for the req to be committed, req Id is %d", rsm.me, id))

	for {
		t, l := rsm.rf.GetState()
		if t != term || l != isLeader { 
			DPrintf(fmt.Sprintf("RSM %d: detected term change or leader change", rsm.me))
			rsm.mu.Lock()
			c, ok := rsm.opresCh[index]
			if ok {
				delete(rsm.opresCh, index)
				close(c)
			}
			rsm.mu.Unlock()
			return rpc.ErrWrongLeader, nil
		}

		select {
			case opres, ok := <- ch: {
				if ok && opres.msg.CommandValid {
					if opres.msg.CommandIndex == index {
						req := opres.msg.Command.(Op)
						if req.Id == id {
							DPrintf(fmt.Sprintf("RSM %d: received the return value of the request, req ID is %d", rsm.me, req.Id))
							return rpc.OK, opres.res
						}
					}
				} else if !ok {
					DPrintf(fmt.Sprintf("RSM %d: subscription channel is closed, ID is %d", rsm.me, id))
				}
				return rpc.ErrWrongLeader, nil
			}
			case <- time.After(time.Duration(500) * time.Millisecond): {
				continue
			}
			case <- rsm.readerDead: {
				DPrintf(fmt.Sprintf("RSM %d: return ErrWrongLeader since reader is dead", rsm.me))
				rsm.mu.Lock()
				c, ok := rsm.opresCh[index]
				if ok {
					delete(rsm.opresCh, index)
					close(c)
				}
				rsm.mu.Unlock()
				return rpc.ErrWrongLeader, nil
			}
		}
	}

}
