package shardgrp

import (
	"fmt"
	"log"
	"bytes"
	"maps"
	"sync"
	"sync/atomic"


	"6.5840/kvraft1/rsm"
	"6.5840/kvsrv1/rpc"
	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/shardkv1/shardgrp/shardrpc"
	"6.5840/shardkv1/shardcfg"
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
	Value string
	Version rpc.Tversion
}

type KVServer struct {
	me   int
	dead int32 // set by Kill()
	rsm  *rsm.RSM
	gid  tester.Tgid

	// Your code here
	mu 	 		 sync.Mutex
	dupMu 	 sync.Mutex

	ShardNums  map[shardcfg.Tshid]shardcfg.Tnum

	Freezed   map[shardcfg.Tshid]bool

	Kvs      map[string]ValueWithVersion

	DupTable    map[string]int // duplicate table; entry per client

	GetReplys   map[string]rpc.GetReply
	PutReplys   map[string]rpc.PutReply
}


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
		case shardrpc.FreezeShardArgs: {
			reply := &shardrpc.FreezeShardReply{}
			kv.freezeShard(&r, reply)
			return *reply
		}
		case shardrpc.InstallShardArgs: {
			reply := &shardrpc.InstallShardReply{}
			kv.installShard(&r, reply)
			return *reply
		}
		case shardrpc.DeleteShardArgs: {
			reply := &shardrpc.DeleteShardReply{}
			kv.deleteShard(&r, reply)
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
	e.Encode(kv.ShardNums)
	e.Encode(kv.Freezed)

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
	clear(kv.Freezed)

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
	if d.Decode(&kv.ShardNums) != nil {
		log.Fatalf("Kv %d: couldn't decode ShardNums", kv.me)
	}
	if d.Decode(&kv.Freezed) != nil {
		log.Fatalf("Kv %d: couldn't decode Freezed", kv.me)
	}
}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	shard := shardcfg.Key2Shard(args.Key)
	if kv.Freezed[shard] {
		reply.Err = rpc.ErrWrongGroup
		return
	}

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
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	shard := shardcfg.Key2Shard(args.Key)
	if kv.Freezed[shard] {
		reply.Err = rpc.ErrWrongGroup
		return
	}

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

// Freeze the specified shard (i.e., reject future Get/Puts for this
// shard) and return the key/values stored in that shard.
func (kv *KVServer) FreezeShard(args *shardrpc.FreezeShardArgs, reply *shardrpc.FreezeShardReply) {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// reject old RPCs based on Num
	if args.Num <= kv.ShardNums[args.Shard] {
		reply.Num = kv.ShardNums[args.Shard]
		reply.Err = rpc.ErrVersion
		return
	}

	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	reply.Err = rep.(shardrpc.FreezeShardReply).Err
	reply.Num = rep.(shardrpc.FreezeShardReply).Num
	reply.State = rep.(shardrpc.FreezeShardReply).State
}

func (kv *KVServer) freezeShard(args *shardrpc.FreezeShardArgs, reply *shardrpc.FreezeShardReply) {
	// reject old RPCs based on Num
	if args.Num <= kv.ShardNums[args.Shard] {
		reply.Num = kv.ShardNums[args.Shard]
		reply.Err = rpc.ErrVersion
		return
	}

	kv.Freezed[args.Shard] = true
	kv.ShardNums[args.Shard] = args.Num

	// find key-value pairs that belong to shard args.Shard
	Kvs := make(map[string]ValueWithVersion)
	for k, v := range kv.Kvs {
		if shardcfg.Key2Shard(k) == args.Shard {
			Kvs[k] = v
		}
	}

	// encode kvs
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(Kvs)

	reply.Num = kv.ShardNums[args.Shard]
	reply.Err = rpc.OK
	reply.State = w.Bytes()
}

// Install the supplied state for the specified shard.
func (kv *KVServer) InstallShard(args *shardrpc.InstallShardArgs, reply *shardrpc.InstallShardReply) {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// reject old RPCs based on Num
	if args.Num <= kv.ShardNums[args.Shard] {
		reply.Err = rpc.ErrVersion
		return
	}

	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	reply.Err = rep.(shardrpc.InstallShardReply).Err
}

func (kv *KVServer) installShard(args *shardrpc.InstallShardArgs, reply *shardrpc.InstallShardReply) {
	// reject old RPCs based on Num
	if args.Num <= kv.ShardNums[args.Shard] {
		reply.Err = rpc.ErrVersion
		return
	}

	kv.ShardNums[args.Shard] = args.Num

	r := bytes.NewBuffer(args.State)

	d := labgob.NewDecoder(r)

	var kvs map[string]ValueWithVersion

	if d.Decode(&kvs) != nil {
		log.Fatalf("Kv %d: couldn't decode kvs", kv.me)
	}

	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	maps.Copy(kv.Kvs, kvs)

	reply.Err = rpc.OK
}

// Delete the specified shard.
func (kv *KVServer) DeleteShard(args *shardrpc.DeleteShardArgs, reply *shardrpc.DeleteShardReply) {
	// Your code here
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// reject old RPCs based on Num
	if args.Num != kv.ShardNums[args.Shard] || !kv.Freezed[args.Shard] {
		reply.Err = rpc.ErrVersion
		return
	}

	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	reply.Err = rep.(shardrpc.DeleteShardReply).Err
}

func (kv *KVServer) deleteShard(args *shardrpc.DeleteShardArgs, reply *shardrpc.DeleteShardReply) {
	// reject old RPCs based on Num
	if args.Num != kv.ShardNums[args.Shard] || !kv.Freezed[args.Shard] {
		reply.Err = rpc.ErrVersion
		return
	}

	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	// find keys that belong to shard args.Shard
	// and then delete them
	for k, _ := range kv.Kvs {
		if shardcfg.Key2Shard(k) == args.Shard {
			delete(kv.Kvs, k)
		}
	}

	reply.Err = rpc.OK
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

// StartShardServerGrp starts a server for shardgrp `gid`.
//
// StartShardServerGrp() and MakeRSM() must return quickly, so they should
// start goroutines for any long-running work.
func StartServerShardGrp(servers []*labrpc.ClientEnd, gid tester.Tgid, me int, persister *tester.Persister, maxraftstate int) []tester.IService {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(rpc.PutArgs{})
	labgob.Register(rpc.GetArgs{})
	labgob.Register(shardrpc.FreezeShardArgs{})
	labgob.Register(shardrpc.InstallShardArgs{})
	labgob.Register(shardrpc.DeleteShardArgs{})
	labgob.Register(rsm.Op{})

	kv := &KVServer{gid: gid, me: me}
	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)

	// Your code here
	DPrintf(fmt.Sprintf("Kv %d is starting", kv.me))

	kv.Kvs       = make(map[string]ValueWithVersion)
  kv.DupTable  = make(map[string]int)
	kv.GetReplys = make(map[string]rpc.GetReply)
	kv.PutReplys = make(map[string]rpc.PutReply)

	kv.Freezed     = make(map[shardcfg.Tshid]bool)
	kv.ShardNums   = make(map[shardcfg.Tshid]shardcfg.Tnum)

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
