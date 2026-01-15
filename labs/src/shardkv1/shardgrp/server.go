package shardgrp

import (
	"fmt"
	"log"
	"bytes"
	"runtime"
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
	dupMu 	 sync.Mutex

	ShardNums  map[shardcfg.Tshid]shardcfg.Tnum

	Freezed   		map[shardcfg.Tshid]bool
	Installed   	map[shardcfg.Tshid]bool

	Kvs      map[shardcfg.Tshid]map[string]ValueWithVersion

	DupTable    map[shardcfg.Tshid]map[int64]int // duplicate table; entry per client

	LastReplys  map[shardcfg.Tshid]map[int64]interface{}
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

	runtime.GC()

	w := new(bytes.Buffer)

	e := labgob.NewEncoder(w)

	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	e.Encode(kv.Kvs)
	e.Encode(kv.DupTable)
	e.Encode(kv.LastReplys)
	e.Encode(kv.ShardNums)
	e.Encode(kv.Freezed)
	e.Encode(kv.Installed)

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
	clear(kv.LastReplys)
	clear(kv.ShardNums)
	clear(kv.Freezed)
	clear(kv.Installed)

	if d.Decode(&kv.Kvs) != nil { log.Fatalf("Kv %d: couldn't decode kvs", kv.me) }
	if d.Decode(&kv.DupTable) != nil { log.Fatalf("Kv %d: couldn't decode dupTable", kv.me) }
	if d.Decode(&kv.LastReplys) != nil { log.Fatalf("Kv %d: couldn't decode lastReplys", kv.me) }
	if d.Decode(&kv.ShardNums) != nil { log.Fatalf("Kv %d: couldn't decode ShardNums", kv.me) }
	if d.Decode(&kv.Freezed) != nil { log.Fatalf("Kv %d: couldn't decode Freezed", kv.me) }
	if d.Decode(&kv.Installed) != nil { log.Fatalf("Kv %d: couldn't decode Installed", kv.me) }
}

func (kv *KVServer) Get(args *rpc.GetArgs, reply *rpc.GetReply) {
	// Your code here
	if kv.killed() {
		DPrintf(fmt.Sprintf("Kv %d: deny Get req because it was killed", kv.me))
		reply.Err = rpc.ErrWrongLeader
		return
	}

	DPrintf(fmt.Sprintf("Kv %d: received Get operation from client %d, args.seqNum is %d", kv.me, args.ClientId, args.SeqNum))

	kv.dupMu.Lock()

	shard := shardcfg.Key2Shard(args.Key)
	if kv.Freezed[shard] {
		DPrintf(fmt.Sprintf("Kv %d: deny Get operation from client %d because the shard %d is freezed", kv.me, args.ClientId, shard))
		reply.Err = rpc.ErrWrongGroup
		kv.dupMu.Unlock()
		return
	}

	seqNum := kv.DupTable[shard][args.ClientId]

	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Get operation from client %d, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.LastReplys[shard][args.ClientId].(rpc.GetReply).Err
		reply.Value = kv.LastReplys[shard][args.ClientId].(rpc.GetReply).Value
		reply.Version = kv.LastReplys[shard][args.ClientId].(rpc.GetReply).Version
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
	if kv.killed() {
		DPrintf(fmt.Sprintf("Kv %d: deny Put req because it was killed", kv.me))
		reply.Err = rpc.ErrWrongLeader
		return
	}

	DPrintf(fmt.Sprintf("Kv %d: received Put operation from client %d, args.seqNum is %d", kv.me, args.ClientId, args.SeqNum))

	kv.dupMu.Lock()

	shard := shardcfg.Key2Shard(args.Key)
	if kv.Freezed[shard] {
		DPrintf(fmt.Sprintf("Kv %d: deny Put operation from client %d because the shard %d is freezed", kv.me, args.ClientId, shard))
		reply.Err = rpc.ErrWrongGroup
		kv.dupMu.Unlock()
		return
	}

	seqNum := kv.DupTable[shard][args.ClientId]

	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Put operation from client %d, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.LastReplys[shard][args.ClientId].(rpc.PutReply).Err
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
	if kv.killed() {
		DPrintf(fmt.Sprintf("Kv %d: deny FreezeShard req because it was killed", kv.me))
		reply.Err = rpc.ErrWrongLeader
		return
	}

	DPrintf(fmt.Sprintf("Kv %d: freezing shard %d", kv.me, args.Shard))

	kv.dupMu.Lock()

	// reject old RPCs based on Num
	// Note: we accept args.Num == kv.ShardNums[args.Shard]
	if args.Num < kv.ShardNums[args.Shard] {
		reply.Num = kv.ShardNums[args.Shard]
		reply.Err = rpc.ErrVersion
		kv.dupMu.Unlock()
		return
	}
	kv.dupMu.Unlock()

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
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	// reject old RPCs based on Num
	if args.Num < kv.ShardNums[args.Shard] {
		reply.Num = kv.ShardNums[args.Shard]
		reply.Err = rpc.ErrVersion
		return
	}

	kv.Freezed[args.Shard] = true
	kv.Installed[args.Shard] = false
	kv.ShardNums[args.Shard] = args.Num

	// encode
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(kv.Kvs[args.Shard])
	e.Encode(kv.DupTable[args.Shard])
	e.Encode(kv.LastReplys[args.Shard])

	reply.Num = kv.ShardNums[args.Shard]
	reply.Err = rpc.OK
	reply.State = w.Bytes()

	DPrintf(fmt.Sprintf("Kv %d: freezed shard %d", kv.me, args.Shard))
}

// Install the supplied state for the specified shard.
func (kv *KVServer) InstallShard(args *shardrpc.InstallShardArgs, reply *shardrpc.InstallShardReply) {
	if kv.killed() {
		DPrintf(fmt.Sprintf("Kv %d: deny InstallShard req because it was killed", kv.me))
		reply.Err = rpc.ErrWrongLeader
		return
	}

	DPrintf(fmt.Sprintf("Kv %d: installing shard %d", kv.me, args.Shard))

	// Your code here
	kv.dupMu.Lock()
	// reject old RPCs based on Num
	if args.Num <= kv.ShardNums[args.Shard] {
		reply.Err = rpc.ErrVersion
		kv.dupMu.Unlock()
		return
	}
	if kv.Installed[args.Shard] {
		DPrintf(fmt.Sprintf("Kv %d: unable to install shard %d because the shard %d is installed", kv.me, args.Shard, args.Shard))
		reply.Err = rpc.ErrWrongGroup
		kv.dupMu.Unlock()
		return
	}
	kv.dupMu.Unlock()

	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	reply.Err = rep.(shardrpc.InstallShardReply).Err
}

func (kv *KVServer) installShard(args *shardrpc.InstallShardArgs, reply *shardrpc.InstallShardReply) {
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	// reject old RPCs based on Num
	if args.Num <= kv.ShardNums[args.Shard] {
		reply.Err = rpc.ErrVersion
		return
	}

	if kv.Installed[args.Shard] {
		DPrintf(fmt.Sprintf("Kv %d: the shard %d is already installed", kv.me, args.Shard))
		reply.Err = rpc.OK
		return
	}

	kv.ShardNums[args.Shard] = args.Num

	kv.Freezed[args.Shard] = false
	kv.Installed[args.Shard] = true

	r := bytes.NewBuffer(args.State)

	d := labgob.NewDecoder(r)

	var kvs        map[string]ValueWithVersion
	var dupTable   map[int64]int
	var lastReplys map[int64]interface{}

	if d.Decode(&kvs) != nil {
		log.Fatalf("Kv %d: couldn't decode kvs", kv.me)
	}
	if d.Decode(&dupTable) != nil {
		log.Fatalf("Kv %d: couldn't decode dupTable", kv.me)
	}
	if d.Decode(&lastReplys) != nil {
		log.Fatalf("Kv %d: couldn't decode lastReplys", kv.me)
	}

	kv.Kvs[args.Shard] = kvs
	kv.DupTable[args.Shard] = dupTable
	kv.LastReplys[args.Shard] = lastReplys

	reply.Err = rpc.OK

	DPrintf(fmt.Sprintf("Kv %d: installed shard %d", kv.me, args.Shard))
}

// Delete the specified shard.
func (kv *KVServer) DeleteShard(args *shardrpc.DeleteShardArgs, reply *shardrpc.DeleteShardReply) {
	// Your code here
	if kv.killed() {
		DPrintf(fmt.Sprintf("Kv %d: deny DeleteShard req because it was killed", kv.me))
		reply.Err = rpc.ErrWrongLeader
		return
	}

	DPrintf(fmt.Sprintf("Kv %d: deleting shard %d", kv.me, args.Shard))

	kv.dupMu.Lock()
	// reject old RPCs based on Num
	if args.Num != kv.ShardNums[args.Shard] {
		reply.Err = rpc.ErrVersion
		kv.dupMu.Unlock()
		return
	}
	if kv.Freezed[args.Shard] {
		DPrintf(fmt.Sprintf("Kv %d: the shard %d is already deleted", kv.me, args.Shard))
		reply.Err = rpc.OK
		kv.dupMu.Unlock()
		return
	}
	kv.dupMu.Unlock()

	err, rep := kv.rsm.Submit(*args)
	if err == rpc.ErrWrongLeader {
		reply.Err = rpc.ErrWrongLeader
		return
	}

	reply.Err = rep.(shardrpc.DeleteShardReply).Err

	DPrintf(fmt.Sprintf("Kv %d: deleted shard %d", kv.me, args.Shard))
}

func (kv *KVServer) deleteShard(args *shardrpc.DeleteShardArgs, reply *shardrpc.DeleteShardReply) {
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	// reject old RPCs based on Num
	if args.Num != kv.ShardNums[args.Shard] || !kv.Freezed[args.Shard] {
		reply.Err = rpc.ErrVersion
		return
	}

	delete(kv.Kvs, args.Shard)
	delete(kv.DupTable, args.Shard)
	delete(kv.ShardNums, args.Shard)
	delete(kv.LastReplys, args.Shard)

	DPrintf(fmt.Sprintf("Kv %d deleted shard %d", kv.me, args.Shard))

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
	labgob.Register(rpc.GetReply{})
	labgob.Register(rpc.PutReply{})

	kv := &KVServer{gid: gid, me: me}
	kv.rsm = rsm.MakeRSM(servers, me, persister, maxraftstate, kv)

	// Your code here
	DPrintf(fmt.Sprintf("Kv %d is starting", kv.me))

	kv.Kvs       = make(map[shardcfg.Tshid]map[string]ValueWithVersion)
  kv.DupTable  = make(map[shardcfg.Tshid]map[int64]int)
	kv.LastReplys = make(map[shardcfg.Tshid]map[int64]interface{})

	kv.Freezed     	= make(map[shardcfg.Tshid]bool)
	kv.Installed    = make(map[shardcfg.Tshid]bool)
	kv.ShardNums   = make(map[shardcfg.Tshid]shardcfg.Tnum)

	for s := 0; s < shardcfg.NShards; s++ {
		kv.Freezed[shardcfg.Tshid(s)] = false
	}

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

	shard := shardcfg.Key2Shard(args.Key)

	if kv.Freezed[shard] {
		DPrintf(fmt.Sprintf("Kv %d: deny Get operation from client %d because the shard %d is freezed", kv.me, args.ClientId, shard))
		reply.Err = rpc.ErrWrongGroup
		return
	}

	seqNum := kv.DupTable[shard][args.ClientId]
	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Get operation in Log from client %d, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.LastReplys[shard][args.ClientId].(rpc.GetReply).Err
		reply.Value = kv.LastReplys[shard][args.ClientId].(rpc.GetReply).Value
		reply.Version = kv.LastReplys[shard][args.ClientId].(rpc.GetReply).Version
		return
	}

	vv, ok := kv.Kvs[shard][args.Key]

	if ok {
		reply.Value = vv.Value
		reply.Version = vv.Version
		reply.Err = rpc.OK
	} else {
		reply.Err = rpc.ErrNoKey
	}

	_, ok2 := kv.DupTable[shard]
	_, ok3 := kv.LastReplys[shard]
	if !ok2 {
		kv.DupTable[shard] = make(map[int64]int)
	}
	if !ok3 {
		kv.LastReplys[shard] = make(map[int64]interface{})
	}

	kv.DupTable[shard][args.ClientId] = args.SeqNum
	kv.LastReplys[shard][args.ClientId] = *reply
}

func (kv *KVServer) put(args *rpc.PutArgs, reply *rpc.PutReply) {
	kv.dupMu.Lock()
	defer kv.dupMu.Unlock()

	shard := shardcfg.Key2Shard(args.Key)

	if kv.Freezed[shard] {
		DPrintf(fmt.Sprintf("Kv %d: deny Get operation from client %d because the shard %d is freezed", kv.me, args.ClientId, shard))
		reply.Err = rpc.ErrWrongGroup
		return
	}

	seqNum := kv.DupTable[shard][args.ClientId]
	if args.SeqNum <= seqNum {
		DPrintf(fmt.Sprintf("Kv %d: duplicated Put operation in Log from client %d, args.seqNum is %d, seqNum is %d", kv.me, args.ClientId, args.SeqNum, seqNum))
		reply.Err = kv.LastReplys[shard][args.ClientId].(rpc.PutReply).Err
		return
	}

	_, ok := kv.Kvs[shard]
	if !ok {
		kv.Kvs[shard] = make(map[string]ValueWithVersion)
	}

	vv, ok := kv.Kvs[shard][args.Key]

	if ok && args.Version == vv.Version {
		vv.Value = args.Value
		vv.Version += 1
		kv.Kvs[shard][args.Key] = vv
		reply.Err = rpc.OK
	} else if ok && args.Version != vv.Version {
			reply.Err = rpc.ErrVersion
	} else if args.Version == 0 {
		vv := ValueWithVersion{}
		vv.Value = args.Value
		vv.Version = 1
		kv.Kvs[shard][args.Key] = vv
		reply.Err = rpc.OK
	} else {
		reply.Err = rpc.ErrNoKey
	}

	_, ok2 := kv.DupTable[shard]
	_, ok3 := kv.LastReplys[shard]
	if !ok2 {
		kv.DupTable[shard] = make(map[int64]int)
	}
	if !ok3 {
		kv.LastReplys[shard] = make(map[int64]interface{})
	}

	kv.DupTable[shard][args.ClientId] = args.SeqNum
	kv.LastReplys[shard][args.ClientId] = *reply
}
