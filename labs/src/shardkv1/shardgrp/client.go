package shardgrp

import (
	"fmt"
	"time"
	"math/rand"
	"sync"
	"github.com/google/uuid"

	"6.5840/kvsrv1/rpc"
	"6.5840/shardkv1/shardcfg"
	"6.5840/tester1"
)

type Clerk struct {
	clnt    *tester.Clnt
	servers []string
	// You will have to modify this struct.
	leaderIdx int
	clientId  string
	seqNum    int
	mu        sync.Mutex
}

func MakeClerk(clnt *tester.Clnt, servers []string) *Clerk {
	ck := &Clerk{clnt: clnt, servers: servers}
 	rand.Seed(time.Now().UnixNano())
	ck.leaderIdx = 0
	ck.seqNum = 0
	ck.clientId  = uuid.New().String()
	return ck
}

func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	ck.seqNum += 1
	DPrintf(fmt.Sprintf("ck %s: receive Get operation, updated seqNum to %d", ck.clientId, ck.seqNum))

	args := &rpc.GetArgs{Key: key, ClientId: ck.clientId, SeqNum: ck.seqNum}
	reply := &rpc.GetReply{}

	for {
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.Get", args, reply)
		if ret == false || reply.Err == rpc.ErrWrongLeader {
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &rpc.GetReply{}
			continue
		}
		DPrintf(fmt.Sprintf("ck %s: receive response of Get operation, seqNum is %d, leader idx is %d", ck.clientId, ck.seqNum, ck.leaderIdx))
		return reply.Value, reply.Version, reply.Err
	}
}

func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	// Your code here
	ck.mu.Lock()
	defer ck.mu.Unlock()

	ck.seqNum += 1

	DPrintf(fmt.Sprintf("ck %s: receive Put operation, updated seqNum to %d", ck.clientId, ck.seqNum))

	args := &rpc.PutArgs{Key: key, Value: value, Version: version, ClientId: ck.clientId, SeqNum: ck.seqNum}
	reply := &rpc.PutReply{}

	counts := make([]int, len(ck.servers))

	for {
		server := ck.servers[ck.leaderIdx]
		ret := ck.clnt.Call(server, "KVServer.Put", args, reply)

		if ret == false || reply.Err == rpc.ErrWrongLeader { 
			if ret == false { counts[ck.leaderIdx] += 1 }
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			reply = &rpc.PutReply{}
			continue
		}

		if counts[ck.leaderIdx] > 0 && reply.Err == rpc.ErrVersion {
			DPrintf(fmt.Sprintf("ck %s: receive response of Put operation, seqNum is %d, leader idx is %d", ck.clientId, ck.seqNum, ck.leaderIdx))
			return rpc.ErrMaybe
		}
		return reply.Err
	}
}

func (ck *Clerk) FreezeShard(s shardcfg.Tshid, num shardcfg.Tnum) ([]byte, rpc.Err) {
	// Your code here
	return nil, ""
}

func (ck *Clerk) InstallShard(s shardcfg.Tshid, state []byte, num shardcfg.Tnum) rpc.Err {
	// Your code here
	return ""
}

func (ck *Clerk) DeleteShard(s shardcfg.Tshid, num shardcfg.Tnum) rpc.Err {
	// Your code here
	return ""
}
