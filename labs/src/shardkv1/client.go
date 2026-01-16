package shardkv

//
// client code to talk to a sharded key/value service.
//
// the client uses the shardctrler to query for the current
// configuration and find the assignment of shards (keys) to groups,
// and then talks to the group that holds the key's shard.
//

import (
	"time"
	"fmt"
	"sync"
	"slices"

	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
	"6.5840/shardkv1/shardctrler"
	"6.5840/shardkv1/shardcfg"
	"6.5840/shardkv1/shardgrp"
	"6.5840/tester1"
)

type Clerk struct {
	clnt *tester.Clnt
	sck  *shardctrler.ShardCtrler
	// You will have to modify this struct.
	mu        sync.Mutex

	shardgrpCks map[tester.Tgid]*shardgrp.Clerk
	gidToServers map[tester.Tgid][]string
}

// The tester calls MakeClerk and passes in a shardctrler so that
// client can call it's Query method
func MakeClerk(clnt *tester.Clnt, sck *shardctrler.ShardCtrler) kvtest.IKVClerk {
	ck := &Clerk{
		clnt: clnt,
		sck:  sck,
	}
	// You'll have to add code here.
	ck.shardgrpCks = make(map[tester.Tgid]*shardgrp.Clerk)
	ck.gidToServers = make(map[tester.Tgid][]string)

	return ck
}


// Get a key from a shardgrp.  You can use shardcfg.Key2Shard(key) to
// find the shard responsible for the key and ck.sck.Query() to read
// the current configuration and lookup the servers in the group
// responsible for key.  You can make a clerk for that group by
// calling shardgrp.MakeClerk(ck.clnt, servers).
func (ck *Clerk) Get(key string) (string, rpc.Tversion, rpc.Err) {
	// You will have to modify this function.
	ck.mu.Lock()
	defer ck.mu.Unlock()

	for {
		config := ck.sck.Query()
		shard  := shardcfg.Key2Shard(key)
	
		gid, servers, ok := config.GidServers(shard)

		if !ok {
			DPrintf(fmt.Sprintf("Fail to find servers for shard %d", shard))
			return "", 0, rpc.ErrWrongGroup
		}

		grpCk, ok := ck.shardgrpCks[gid]

		if !ok || !slices.Equal(ck.gidToServers[gid], servers) {
			DPrintf(fmt.Sprintf("Fail to find grpCh for shard %d. Create one now.", shard))
			gck := shardgrp.MakeClerk(ck.clnt, servers)
			ck.shardgrpCks[gid] = gck
			ck.gidToServers[gid] = servers
			grpCk = gck
		}

		val, ver, err := grpCk.Get(key)

		if err != rpc.ErrWrongGroup {
			return val, ver, err
		}

		time.Sleep(time.Duration(20) * time.Millisecond)
	}
}

// Put a key to a shard group.
func (ck *Clerk) Put(key string, value string, version rpc.Tversion) rpc.Err {
	// You will have to modify this function.
	ck.mu.Lock()
	defer ck.mu.Unlock()

	errMaybe := false

	for {
		config := ck.sck.Query()
		shard  := shardcfg.Key2Shard(key)

		gid, servers, ok := config.GidServers(shard)

		if !ok {
			time.Sleep(time.Duration(20) * time.Millisecond)
			continue
		}

		grpCk, ok := ck.shardgrpCks[gid]

		if !ok || !slices.Equal(ck.gidToServers[gid], servers) {
			DPrintf(fmt.Sprintf("Fail to find grpCh for shard %d. Create one now.", shard))
			gck := shardgrp.MakeClerk(ck.clnt, servers)
			ck.shardgrpCks[gid] = gck
			ck.gidToServers[gid] = servers
			grpCk = gck
		}

		err := grpCk.Put(key, value, version)

		if err == rpc.ErrMaybe {
			errMaybe = true
		}

		if err != rpc.ErrWrongGroup && err != rpc.ErrMaybe {
		 	// If the server returns ErrVersion on a resend RPC,
			// then Put must return ErrMaybe to the application, since
			// its earlier RPC might have been processed by the server successfully
			// but the response was lost, and the the Clerk doesn't know if
			// the Put was performed or not.
			if errMaybe && err == rpc.ErrVersion {
				return rpc.ErrMaybe
			}
			return err
		}

		time.Sleep(time.Duration(20) * time.Millisecond)
	}
}
