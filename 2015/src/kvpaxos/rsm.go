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
	Req    			interface{}
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

func (rsm *RSM) kill() {
	atomic.StoreInt32(&rsm.dead, 1)
}

func (rsm *RSM) isdead() bool {
	return atomic.LoadInt32(&rsm.dead) != 0
}


func MakeRSM(me int, px *paxos.Paxos, sm StateMachine) *RSM {
	rsm := &RSM{}

	rsm.me = me
	rsm.px = px
	rsm.sm = sm
	rsm.dead = 0
	rsm.lastAppliedIndex = -1
	rsm.opId = -1
	rsm.callbacks = make(map[int]chan OpRes)

	go rsm.reader()

	return rsm
}

func (rsm *RSM) Submit(req interface{}) any {
	for {
		opId, callback := rsm.start(req)

		select {
			case <- time.After(time.Duration(100) * time.Millisecond):
				if rsm.isdead() {
					return nil
				}
			case opRes, ok := <- callback:
				if !ok || rsm.isdead() {
					return nil
				}
				if opRes.id == opId && opRes.me == rsm.me {
					return opRes.res
				}
		}
	}
}

func (rsm *RSM) start(req interface{}) (int, <-chan OpRes) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	rsm.opId++

	op := Op{
		Me: rsm.me,
		Id: rsm.opId,
		Req: req,
	}

	seq := rsm.px.Max() + 1

	rsm.px.Start(seq, op)

	rsm.callbacks[op.Id] = make(chan OpRes)

	return rsm.opId, rsm.callbacks[op.Id]
}

func (rsm *RSM) reader() {
	to := 10 * time.Millisecond

	for !rsm.isdead() {
		seq := rsm.lastAppliedIndex + 1
		fate, val := rsm.px.Status(seq)

		if fate != paxos.Decided {
			time.Sleep(to)
			if to < 2 * time.Second {
				to *= 2
			}
			continue
		}

		DPrintf("RSM %d: find log entry %d is decided", rsm.me, seq)

		to = 10 * time.Millisecond

		op := val.(Op)

		res := rsm.sm.DoOp(op)
	
		rsm.px.Done(seq)
		
		rsm.lastAppliedIndex = seq

		opRes := OpRes{
			me: op.Me,
			id: op.Id,
			res: res,
		}

		rsm.sendOpRes(opRes)
	}
}

func (rsm *RSM) sendOpRes(opRes OpRes) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	callback, ok := rsm.callbacks[opRes.id]

	if ok {
		DPrintf("RSM %d: send opRes %d to callback", rsm.me, opRes.id)

		callback <- opRes

		close(callback)
		delete(rsm.callbacks, opRes.id)
	}
}
