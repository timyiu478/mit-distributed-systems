package kvraft

import (
	"fmt"
	"time"
	"math/rand"
	"sync"
	"github.com/google/uuid"

	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
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

func MakeClerk(clnt *tester.Clnt, servers []string) kvtest.IKVClerk {
	ck := &Clerk{clnt: clnt, servers: servers}
	// You'll have to add code here.
	ck.leaderIdx = 0
	ck.seqNum = 0
	ck.clientId  = uuid.New().String()
	return ck
}

// Get fetches the current value and version for a key.  It returns
// ErrNoKey if the key does not exist. It keeps trying forever in the
// face of all other errors.
//
// You can send an RPC to server i with code like this:
// ok := ck.clnt.Call(ck.servers[i], "KVServer.Get", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
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

// Put updates key with value only if the version in the
// request matches the version of the key at the server.  If the
// versions numbers don't match, the server should return
// ErrVersion.  If Put receives an ErrVersion on its first RPC, Put
// should return ErrVersion, since the Put was definitely not
// performed at the server. If the server returns ErrVersion on a
// resend RPC, then Put must return ErrMaybe to the application, since
// its earlier RPC might have been processed by the server successfully
// but the response was lost, and the the Clerk doesn't know if
// the Put was performed or not.
//
// You can send an RPC to server i with code like this:
// ok := ck.clnt.Call(ck.servers[i], "KVServer.Put", &args, &reply)
//
// The types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. Additionally, reply must be passed as a pointer.
func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
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
