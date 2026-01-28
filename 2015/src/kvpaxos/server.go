package kvpaxos

import "net"
import "fmt"
import "net/rpc"
import "log"
import "lab/paxos"
import "sync"
import "sync/atomic"
import "os"
import "syscall"
import "encoding/gob"
import "math/rand"


const Debug = true

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type KVPaxos struct {
	mu         sync.Mutex
	l          net.Listener
	me         int
	dead       int32 // for testing
	unreliable int32 // for testing
	px         *paxos.Paxos

	// Your definitions here.
	kvs 			 map[string]string
	// Deduplication Detection
	// Assume that each clerk has only one outstanding Put, Get, or Append
	lastReqId  		map[int64]int
	lastReplys    map[int64]interface{}
	
	// RSM
	rsm        *RSM
}


func (kv *KVPaxos) Get(args *GetArgs, reply *GetReply) error {
	// Your code here.
	isDup, lastReply := kv.checkDeplicatedReq(args.Me, args.SeqId)
	if isDup {
		DPrintf("Server %d: request %d is duplicated", kv.me, args.SeqId)

		reply.Value = lastReply.(GetReply).Value
		reply.Err   = lastReply.(GetReply).Err
		return nil
	}

	for !kv.isdead() {
		err, res := kv.rsm.Submit(*args)
		
		if err == OK {
			reply.Err = res.(GetReply).Err
			reply.Value = res.(GetReply).Value
			break
		}
	}

	return nil
}

func (kv *KVPaxos) PutAppend(args *PutAppendArgs, reply *PutAppendReply) error {
	// Your code here.
	isDup, lastReply := kv.checkDeplicatedReq(args.Me, args.SeqId)
	if isDup {
		DPrintf("Server %d: request %d is duplicated", kv.me, args.SeqId)

		reply.Err   = lastReply.(PutAppendReply).Err
		return nil
	}

	for !kv.isdead() {
		err, res := kv.rsm.Submit(*args)
		
		if err == OK {
			reply.Err = res.(PutAppendReply).Err
			break
		}
	}

	return nil
}

func (kv *KVPaxos) checkDeplicatedReq(clientId int64, reqId int) (bool, interface{}) {
	kv.mu.Lock()
	defer kv.mu.Unlock()


	if reqId <= kv.lastReqId[clientId] {
		return true, kv.lastReplys[clientId]
	}

	return false, struct{}{}
}


// tell the server to shut itself down.
// please do not change these two functions.
func (kv *KVPaxos) kill() {
	DPrintf("Kill(%d): die\n", kv.me)
	atomic.StoreInt32(&kv.dead, 1)
	kv.l.Close()
	kv.px.Kill()
	kv.rsm.Kill()
}

// call this to find out if the server is dead.
func (kv *KVPaxos) isdead() bool {
	return atomic.LoadInt32(&kv.dead) != 0
}

// please do not change these two functions.
func (kv *KVPaxos) setunreliable(what bool) {
	if what {
		atomic.StoreInt32(&kv.unreliable, 1)
	} else {
		atomic.StoreInt32(&kv.unreliable, 0)
	}
}

func (kv *KVPaxos) isunreliable() bool {
	return atomic.LoadInt32(&kv.unreliable) != 0
}

func (kv *KVPaxos) DoOp(req any) any {
	switch r := req.(type) {
		case GetArgs: {
			reply := &GetReply{}
			kv.get(&r, reply)
			return *reply
		}
		case PutAppendArgs: {
			reply := &PutAppendReply{}
			kv.putAppend(&r, reply)
			return *reply
		}
	}

	// Invalid req
	DPrintf("KV %d: DoOP receives invalid req", kv.me)
	return nil
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Paxos to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
//
func StartServer(servers []string, me int) *KVPaxos {
	// call gob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	gob.Register(Op{})
	gob.Register(PutAppendArgs{})
	gob.Register(GetArgs{})
	gob.Register(PutAppendReply{})
	gob.Register(GetReply{})

	kv := new(KVPaxos)
	kv.me = me

	// Your initialization code here.
	kv.kvs = make(map[string]string)
	kv.lastReqId = make(map[int64]int)
	kv.lastReplys = make(map[int64]interface{})

	rpcs := rpc.NewServer()
	rpcs.Register(kv)

	kv.px = paxos.Make(servers, me, rpcs)
	kv.rsm = MakeRSM(me, kv.px, kv)

	os.Remove(servers[me])
	l, e := net.Listen("unix", servers[me])
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	kv.l = l


	// please do not change any of the following code,
	// or do anything to subvert it.

	go func() {
		for kv.isdead() == false {
			conn, err := kv.l.Accept()
			if err == nil && kv.isdead() == false {
				if kv.isunreliable() && (rand.Int63()%1000) < 100 {
					// discard the request.
					conn.Close()
				} else if kv.isunreliable() && (rand.Int63()%1000) < 200 {
					// process the request but force discard of reply.
					c1 := conn.(*net.UnixConn)
					f, _ := c1.File()
					err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
					if err != nil {
						fmt.Printf("shutdown: %v\n", err)
					}
					go rpcs.ServeConn(conn)
				} else {
					go rpcs.ServeConn(conn)
				}
			} else if err == nil {
				conn.Close()
			}
			if err != nil && kv.isdead() == false {
				fmt.Printf("KVPaxos(%v) accept: %v\n", me, err.Error())
				kv.kill()
			}
		}
	}()

	return kv
}

func (kv *KVPaxos) get(args *GetArgs, reply *GetReply) {
	isDup, lastReply := kv.checkDeplicatedReq(args.Me, args.SeqId)
	if isDup {
		DPrintf("Server %d: request %d is duplicated", kv.me, args.SeqId)

		reply.Err = lastReply.(GetReply).Err
	}

	val, ok := kv.kvs[args.Key]
	if ok {
		reply.Err   = OK
		reply.Value = val
		return
	}
	reply.Err   = ErrNoKey
}

func (kv *KVPaxos) putAppend(args *PutAppendArgs, reply *PutAppendReply) {
	isDup, lastReply := kv.checkDeplicatedReq(args.Me, args.SeqId)
	if isDup {
		DPrintf("Server %d: request %d is duplicated", kv.me, args.SeqId)

		reply.Err = lastReply.(PutAppendReply).Err
	}

	oldval, ok := kv.kvs[args.Key]

	switch args.Op {
		case "Put":
			kv.kvs[args.Key] = args.Value
		case "Append":
			if ok {
				kv.kvs[args.Key] = oldval + args.Value
			} else {
				kv.kvs[args.Key] = args.Value
			}
	}

	reply.Err = OK
}
