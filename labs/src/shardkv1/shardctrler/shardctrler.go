package shardctrler

//
// Shardctrler with InitConfig, Query, and ChangeConfigTo methods
//

import (
	"fmt"
	"log"
	"sync"

	"6.5840/kvsrv1"
	"6.5840/kvsrv1/rpc"
	"6.5840/kvtest1"
	"6.5840/shardkv1/shardcfg"
	"6.5840/shardkv1/shardgrp"
	"6.5840/tester1"
)

const Debug = true

func DPrintf(format string, a ...interface{}) {
	if Debug {
		log.Printf(format, a...)
	}
}

// ShardCtrler for the controller and kv clerk.
type ShardCtrler struct {
	clnt *tester.Clnt
	kvtest.IKVClerk

	killed int32 // set by Kill()

	// Your data here.
	mu sync.Mutex
}

// Make a ShardCltler, which stores its state in a kvsrv.
func MakeShardCtrler(clnt *tester.Clnt) *ShardCtrler {
	sck := &ShardCtrler{clnt: clnt}
	srv := tester.ServerName(tester.GRP0, 0)
	sck.IKVClerk = kvsrv.MakeClerk(clnt, srv)
	// Your code here.
	return sck
}

// The tester calls InitController() before starting a new
// controller. In part A, this method doesn't need to do anything. In
// B and C, this method implements recovery.
func (sck *ShardCtrler) InitController() {
}

// Called once by the tester to supply the first configuration.  You
// can marshal ShardConfig into a string using shardcfg.String(), and
// then Put it in the kvsrv for the controller at version 0.  You can
// pick the key to name the configuration.  The initial configuration
// lists shardgrp shardcfg.Gid1 for all shards.
func (sck *ShardCtrler) InitConfig(cfg *shardcfg.ShardConfig) {
	// Your code here
	sck.IKVClerk.Put("config", cfg.String(), 0)
}

// Called by the tester to ask the controller to change the
// configuration from the current one to new.  While the controller
// changes the configuration it may be superseded by another
// controller.
func (sck *ShardCtrler) ChangeConfigTo(new *shardcfg.ShardConfig) {
	// Your code here.
	sck.mu.Lock()
	defer sck.mu.Unlock()

	oldConfig := sck.Query()

	cks := make(map[tester.Tgid]*shardgrp.Clerk)

	for s := 0; s < shardcfg.NShards; s++ {
		shard := shardcfg.Tshid(s)

		oldGid, oldServers, oldOk := oldConfig.GidServers(shard)
		newGid, newServers, newOk := new.GidServers(shard)

		if newOk && oldOk {
			if oldGid == newGid { 
				DPrintf(fmt.Sprintf("SCK: the gid of shard %d remains unchange", s))
				continue 
			}
			DPrintf(fmt.Sprintf("SCK: change shard %d from current gid %d to new gid %d", s, oldGid, newGid))

			oldShardGrpCk, oldOk := cks[oldGid]
			newShardGrpCk, newOk := cks[newGid]

			if !oldOk { 
				oldShardGrpCk = shardgrp.MakeClerk(sck.clnt, oldServers)
			}
			if !newOk {
				newShardGrpCk = shardgrp.MakeClerk(sck.clnt, newServers)
			}

			state, err1 := oldShardGrpCk.FreezeShard(shard, new.Num)

			if err1 == rpc.OK {
				err2 := newShardGrpCk.InstallShard(shard, state, new.Num)
				if err2 == rpc.OK {
					oldShardGrpCk.DeleteShard(shard, new.Num)

					sck.IKVClerk.Put("config", new.String(), rpc.Tversion(new.Num))
				}
			}
		}
	}
}


// Return the current configuration
func (sck *ShardCtrler) Query() *shardcfg.ShardConfig {
	// Your code here.
	cfgStr, _, _ := sck.IKVClerk.Get("config")

	return shardcfg.FromString(cfgStr)
}

