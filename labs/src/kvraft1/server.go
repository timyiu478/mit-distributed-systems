package kvraft

import (
	"fmt"
	"log"
	"bytes"
	"sync"
	"sync/atomic"

	"6.5840/kvraft1/rsm"
	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/tester1"

)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type ValueWithVersion struct {
	Value   string
	Version rpc.Tversion
}

type KVServer struct {
	me   int
	dead int32 // set by Kill()
	rsm  *rsm.RSM

	// Your definitions here.
	mu 	 		 sync.Mutex
	dupMu 	 sync.Mutex

	Kvs      map[string]ValueWithVersion

	DupTable    map[string]int // duplicate table; entry per client

	GetReplys   map[string]rpc.GetReply
	PutReplys   map[string]rpc.PutReply
}

// To type-cast req to the right type, take a look at Go's type switches or type
// assertions below:
//
// https://go.dev/tour/methods/16
// https://go.dev/tour/methods/15
func (kv *KVServer) DoOp(req any) any {
	// Your code here

	switch r := req.(type) {
		case rpc.GetArgs: {
			reply := &rpc.GetReply{}
			kv.get(&r, reply)
			return *reply
		}
		case rpc.PutArgs: {
			reply := &rpc.PutReply{}
			kv.put(&r, reply)
			return *reply
		}
	}

	// Invalid req
	return nil
}

func (kv *KVServer) Snapshot() []byte {
	// Your code here
	DPrintf(fmt.Sprintf("Kv %d: snapshot", kv.me)) 

	w := new(bytes.Buffer)

	e := labgob.NewEncoder(w)

	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	e.Encode(kv.Kvs)
	e.Encode(kv.DupTable)
	e.Encode(kv.GetReplys)
	e.Encode(kv.PutReplys)

	return w.Bytes()
}

func (kv *KVServer) Restore(data []byte) {
	// Your code here
	DPrintf(fmt.Sprintf("Kv %d: restore", kv.me))

	r := bytes.NewBuffer(data)

	d := labgob.NewDecoder(r)
	
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	clear(kv.Kvs)
	clear(kv.DupTable)
	clear(kv.GetReplys)
	clear(kv.PutReplys)

	if d.Decode(&kv.Kvs) != nil {
		log.Fatalf("Kv %d: couldn't decode kvs", kv.me)
	}
	if d.Decode(&kv.DupTable) != nil {
		log.Fatalf("Kv %d: couldn't decode dupTable", kv.me)
	}
	if d.Decode(&kv.GetReplys) != nil {
		log.Fatalf("Kv %d: couldn't decode getReplys", kv.me)
	}
	if d.Decode(&kv.PutReplys) != nil {
		log.Fatalf("Kv %d: couldn't decode putReplys", kv.me)
	}

}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here. Use kv.rsm.Submit() to submit args
	// You can use go's type casts to turn the any return value
	// of Submit() into a GetReply: rep.(rpc.GetReply)
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.dupMu.Lock()
	seqNum := kv.DupTable[args.ClientId]

	DPrintf(fmt.Sprintf("Kv %d: received Get operation from client %s, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum)) 

	if args.SeqNum <= seqNum { 
		DPrintf(fmt.Sprintf("Kv %d: duplicated Get operation from client %s, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum)) 
		reply.Err = kv.GetReplys[args.ClientId].Err 
		reply.Value = kv.GetReplys[args.ClientId].Value
		reply.Version = kv.GetReplys[args.ClientId].Version
		kv.dupMu.Unlock()
		return
	}
	kv.dupMu.Unlock()

	// Note: Submit() waits for the command to be committed if it is the leader
	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	reply.Err = rep.(rpc.GetReply).Err
	reply.Value = rep.(rpc.GetReply).Value
	reply.Version = rep.(rpc.GetReply).Version
}

func (kv *KVServer) Put(args *rpc.PutArgs, reply *rpc.PutReply) {
	// Your code here. Use kv.rsm.Submit() to submit args
	// You can use go's type casts to turn the any return value
	// of Submit() into a PutReply: rep.(rpc.PutReply)
	kv.mu.Lock()
	defer kv.mu.Unlock()
	
	kv.dupMu.Lock()
	seqNum := kv.DupTable[args.ClientId]

	DPrintf(fmt.Sprintf("Kv %d: received Put operation from client %s, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))

	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Put operation from client %s, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.PutReplys[args.ClientId].Err 
		kv.dupMu.Unlock()
		return
	}
	kv.dupMu.Unlock()

	// Note: Submit() waits for the command to be committed if it is the leader
	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}
	reply.Err = rep.(rpc.PutReply).Err
}

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// Get returns the value and version for args.Key, if args.Key
// exists. Otherwise, Get returns ErrNoKey.
func (kv *KVServer) get(args *rpc.GetArgs, reply *rpc.GetReply) {
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	seqNum := kv.DupTable[args.ClientId]
	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Get operation in Log from client %s, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.GetReplys[args.ClientId].Err 
		reply.Value = kv.GetReplys[args.ClientId].Value
		reply.Version = kv.GetReplys[args.ClientId].Version
		return
	}

	vv, ok := kv.Kvs[args.Key]

	if ok {
		reply.Value = vv.Value
		reply.Version = vv.Version
		reply.Err = rpc.OK
	} else {
		reply.Err = rpc.ErrNoKey
	}

	kv.DupTable[args.ClientId] = args.SeqNum
	kv.GetReplys[args.ClientId] = *reply
}

// Update the value for a key if args.Version matches the version of
// the key on the server. If versions don't match, return ErrVersion.
// If the key doesn't exist, Put installs the value if the
// args.Version is 0, and returns ErrNoKey otherwise.
func (kv *KVServer) put(args *rpc.PutArgs, reply *rpc.PutReply) {
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	seqNum := kv.DupTable[args.ClientId]
	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Put operation in Log from client %s, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.PutReplys[args.ClientId].Err 
		return
	}

	vv, ok := kv.Kvs[args.Key]

	if ok && args.Version == vv.Version {
		vv.Value = args.Value
		vv.Version += 1
		kv.Kvs[args.Key] = vv
		reply.Err = rpc.OK
	} else if ok && args.Version != vv.Version {
		reply.Err = rpc.ErrVersion
	} else if args.Version == 0 {
		vv := ValueWithVersion{}
		vv.Value = args.Value
		vv.Version = 1
		kv.Kvs[args.Key] = vv
		reply.Err = rpc.OK
	} else {
		reply.Err = rpc.ErrNoKey
	}

	kv.DupTable[args.ClientId] = args.SeqNum
	kv.PutReplys[args.ClientId] = *reply
}

// StartKVServer() and MakeRSM() must return quickly, so they should
// start goroutines for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, gid tester.Tgid, me int, persister *tester.Persister, maxraftstate int) []tester.IService {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(rsm.Op{})
	labgob.Register(rpc.PutArgs{})
	labgob.Register(rpc.GetArgs{})

	kv := &KVServer{me: me}


	// You may need initialization code here.
	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)

	DPrintf(fmt.Sprintf("Kv %d is starting", kv.me))

	kv.Kvs       = make(map[string]ValueWithVersion)
  kv.DupTable  = make(map[string]int)
	kv.GetReplys = make(map[string]rpc.GetReply)
	kv.PutReplys = make(map[string]rpc.PutReply)

	if maxraftstate > -1 {
		snapshotSize := persister.SnapshotSize()

		if snapshotSize > 0 {
			snapshot := persister.ReadSnapshot()
			DPrintf(fmt.Sprintf("KV %d: starts to restore snapshot", kv.me))
			kv.Restore(snapshot)
		}
	}

	return []tester.IService{kv, kv.rsm.Raft()}
}
