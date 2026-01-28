package kvpaxos

import (
	"time"
	"sync"
	"sync/atomic"
	"lab/paxos"
)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Me          int // server id
	Id          int // op     id
	Req    			any
}

type OpRes struct {
	me					int
	id          int
	res         any
}

type StateMachine interface {
	DoOp(any) any
}

type RSM struct {
	mu           			sync.Mutex
	me           			int
	dead       				int32
	px           			*paxos.Paxos
	sm           			StateMachine
	lastAppliedIndex  int
	opId              int
	callbacks         map[int]chan OpRes
}

func (rsm *RSM) Kill() {
	atomic.StoreInt32(&rsm.dead, 1)
}

func (rsm *RSM) isdead() bool {
	return atomic.LoadInt32(&rsm.dead) != 0
}


func MakeRSM(me int, px *paxos.Paxos, sm StateMachine) *RSM {
	rsm := &RSM{}

	atomic.StoreInt32(&rsm.dead, 0)

	rsm.me = me
	rsm.px = px
	rsm.sm = sm
	rsm.lastAppliedIndex = -1
	rsm.opId = -1
	rsm.callbacks = make(map[int]chan OpRes)

	go rsm.reader()

	return rsm
}

func (rsm *RSM) Submit(req any) (Err, any) {
	opId, callback := rsm.start(req)

	for !rsm.isdead() {
		select {
			case <- time.After(time.Duration(100) * time.Millisecond):
				if rsm.isdead() {
					return ErrNotAccept, nil
				}
			case opRes := <- callback:
				DPrintf("RSM %d: opRes.id = %d, opRes.me = %d, opId = %d", rsm.me, opRes.id, opRes.me, opId)
				if rsm.isdead() || opRes.id != opId || opRes.me != rsm.me {
					return ErrNotAccept, nil
				}
				return OK, opRes.res
		}
	}

	return ErrNotAccept, nil
}

func (rsm *RSM) start(req any) (int, <-chan OpRes) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	rsm.opId++

	op := Op{
		Me: rsm.me,
		Id: rsm.opId,
		Req: req,
	}

	seq := rsm.px.Max() + 1

	if seq <= rsm.lastAppliedIndex {
		seq = rsm.lastAppliedIndex + 1
	}

	DPrintf("RSM %d: call px.Start(seq=%d, op.Me=%d, op.Id=%d)", rsm.me, seq, op.Me, op.Id)

	rsm.px.Start(seq, op)

	rsm.callbacks[seq] = make(chan OpRes)

	return rsm.opId, rsm.callbacks[seq]
}

func (rsm *RSM) reader() {
	to := 10 * time.Millisecond

	for !rsm.isdead() {
		if !rsm.read() {
			time.Sleep(to)
			if to < 1 * time.Second {
				to *= 2
			}
		}
		to = 10 * time.Millisecond
	}
}

func (rsm *RSM) read() bool {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	seq := rsm.lastAppliedIndex + 1

	fate, val := rsm.px.Status(seq)

	if fate != paxos.Decided {
		return false
	}

	DPrintf("RSM %d: find log entry %d is decided", rsm.me, seq)

	op := val.(Op)

	res := rsm.sm.DoOp(op.Req)

	rsm.lastAppliedIndex = seq

	rsm.px.Done(seq)

	opRes := OpRes{
		me: op.Me,
		id: op.Id,
		res: res,
	}

	callback, ok := rsm.callbacks[seq]

	if ok {
		DPrintf("RSM %d: send opRes %d to callback, seq is %d", rsm.me, opRes.id, seq)

		callback <- opRes

		close(callback)
		delete(rsm.callbacks, seq)
	}

	return true
}
