package shardgrp

import (
	"time"
	"fmt"
	"sync"
	"sync/atomic"

	"6.5840/kvsrv1/rpc"
	"6.5840/shardkv1/shardcfg"
	"6.5840/shardkv1/shardgrp/shardrpc"
	"6.5840/tester1"
)

var clientId atomic.Int64

type Clerk struct {
	clnt    *tester.Clnt
	servers []string
	// You will have to modify this struct.
	leaderIdx int
	clientId  int64
	seqNum    int
	mu        sync.Mutex
}

func MakeClerk(clnt *tester.Clnt, servers []string) *Clerk {
	ck := &Clerk{clnt: clnt, servers: servers}
	ck.leaderIdx = 0
	ck.seqNum = 0
	ck.clientId  = clientId.Add(1)
	return ck
}

func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	ck.seqNum += 1
	DPrintf(fmt.Sprintf("ck %d: receive Get operation, updated seqNum to %d", ck.clientId, ck.seqNum))

	args := &rpc.GetArgs{Key: key, ClientId: ck.clientId, SeqNum: ck.seqNum}
	reply := &rpc.GetReply{}

	for {
		DPrintf(fmt.Sprintf("ck %d: send Get operation to leader idx %d", ck.clientId, ck.leaderIdx))
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.Get", args, reply)
		if ret == false {
			DPrintf(fmt.Sprintf("ck %d: unable to get Get response from leader idx %d within timeout, servername is %s", ck.clientId, ck.leaderIdx, server))
		}
		if ret == false || reply.Err == rpc.ErrWrongLeader {
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &rpc.GetReply{}
			time.Sleep(time.Duration(20) * time.Millisecond)
			continue
		}
		DPrintf(fmt.Sprintf("ck %d: receive response of Get operation, seqNum is %d, leader idx is %d", ck.clientId, ck.seqNum, ck.leaderIdx))
		return reply.Value, reply.Version, reply.Err
	}
}

func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	ck.seqNum += 1

	DPrintf(fmt.Sprintf("ck %d: receive Put operation, updated seqNum to %d", ck.clientId, ck.seqNum))

	args := &rpc.PutArgs{Key: key, Value: value, Version: version, ClientId: ck.clientId, SeqNum: ck.seqNum}
	reply := &rpc.PutReply{}

	counts := make([]int, len(ck.servers))

	for {
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.Put", args, reply)

		if ret == false {
			DPrintf(fmt.Sprintf("ck %d: unable to get Put response from leader idx %d within timeout, servername is %s", ck.clientId, ck.leaderIdx, server))
			counts[ck.leaderIdx] += 1
		}
		if ret == false || reply.Err == rpc.ErrWrongLeader { 
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &rpc.PutReply{}
			time.Sleep(time.Duration(20) * time.Millisecond)
			continue
		}

		if counts[ck.leaderIdx] > 0 && reply.Err == rpc.ErrVersion {
			DPrintf(fmt.Sprintf("ck %d: receive response of Put operation, seqNum is %d, leader idx is %d", ck.clientId, ck.seqNum, ck.leaderIdx))
			return rpc.ErrMaybe
		}
		return reply.Err
	}
}

func (ck *Clerk) FreezeShard(s shardcfg.Tshid, num shardcfg.Tnum) ([]byte, rpc.Err) {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	args := &shardrpc.FreezeShardArgs{Shard: s, Num: num}
	reply := &shardrpc.FreezeShardReply{}


	for {
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.FreezeShard", args, reply)

		if ret == false {
			DPrintf(fmt.Sprintf("ck %d: unable to get FreezeShard response from leader idx %d within timeout, servername is %s", ck.clientId, ck.leaderIdx, server))
		}
		if ret == false || reply.Err == rpc.ErrWrongLeader { 
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &shardrpc.FreezeShardReply{}
			time.Sleep(time.Duration(20) * time.Millisecond)
			continue
		}

		if reply.Num > args.Num {
			DPrintf(fmt.Sprintf("ck %d: Freezeshard reply Num %d > args.Num %d", ck.clientId, reply.Num, args.Num))
			return nil, reply.Err
		}

		return reply.State, reply.Err
	}
}

func (ck *Clerk) InstallShard(s shardcfg.Tshid, state []byte, num shardcfg.Tnum) rpc.Err {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	args := &shardrpc.InstallShardArgs{Shard: s, State: state, Num: num}
	reply := &shardrpc.InstallShardReply{}

	for {
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.InstallShard", args, reply)

		if ret == false {
			DPrintf(fmt.Sprintf("ck %d: unable to get InstallShard response from leader idx %d within timeout, servername is %s", ck.clientId, ck.leaderIdx, server))
		}
		if ret == false || reply.Err == rpc.ErrWrongLeader { 
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &shardrpc.InstallShardReply{}
			time.Sleep(time.Duration(20) * time.Millisecond)
			continue
		}

		return reply.Err
	}
}

func (ck *Clerk) DeleteShard(s shardcfg.Tshid, num shardcfg.Tnum) rpc.Err {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	args := &shardrpc.DeleteShardArgs{Shard: s, Num: num}
	reply := &shardrpc.DeleteShardReply{}

	for {
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.DeleteShard", args, reply)

		if ret == false {
			DPrintf(fmt.Sprintf("ck %d: unable to get InstallShard response from leader idx %d within timeout, servername is %s", ck.clientId, ck.leaderIdx, server))
		}
		if ret == false || reply.Err == rpc.ErrWrongLeader { 
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &shardrpc.DeleteShardReply{}
			time.Sleep(time.Duration(20) * time.Millisecond)
			continue
		}

		return reply.Err
	}
}
