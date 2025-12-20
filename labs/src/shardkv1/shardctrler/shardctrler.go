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

const Debug = false

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

	cks map[tester.Tgid]*shardgrp.Clerk
}

// Make a ShardCltler, which stores its state in a kvsrv.
func MakeShardCtrler(clnt *tester.Clnt) *ShardCtrler {
	sck := &ShardCtrler{clnt: clnt}
	srv := tester.ServerName(tester.GRP0, 0)
	sck.IKVClerk = kvsrv.MakeClerk(clnt, srv)
	sck.cks = make(map[tester.Tgid]*shardgrp.Clerk)
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
	sck.mu.Lock()
	defer sck.mu.Unlock()

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

	cfgStr, version, err := sck.IKVClerk.Get("config")

	if err != rpc.OK {
		DPrintf("SCK: failed to get current config")
		return
	}

	oldConfig := shardcfg.FromString(cfgStr)


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

			oldShardGrpCk, oldOk := sck.cks[oldGid]
			newShardGrpCk, newOk := sck.cks[newGid]

			if !oldOk { 
				oldShardGrpCk = shardgrp.MakeClerk(sck.clnt, oldServers)
				sck.cks[oldGid] = oldShardGrpCk
			}
			if !newOk {
				newShardGrpCk = shardgrp.MakeClerk(sck.clnt, newServers)
				sck.cks[newGid] = newShardGrpCk
			}

			state, freezeErr := oldShardGrpCk.FreezeShard(shard, new.Num)

			if freezeErr != rpc.OK {
				DPrintf(fmt.Sprintf("SCK: failed to freezed shard %d", s))
				continue
			}

			inShdErr := newShardGrpCk.InstallShard(shard, state, new.Num)

			if inShdErr != rpc.OK {
				DPrintf(fmt.Sprintf("SCK: failed to install shard %d to group %d", s, newGid))
				continue
			}

			delShdErr := oldShardGrpCk.DeleteShard(shard, new.Num)

			if delShdErr != rpc.OK {
				DPrintf(fmt.Sprintf("SCK: failed to delete shard %d from group %d", s, oldGid))
			}
		}
	}

	putErr := sck.IKVClerk.Put("config", new.String(), version)

	if putErr != rpc.OK {
		DPrintf(fmt.Sprintf("SCK: fail to Put new config, version is %d", version))
	}
}


// Return the current configuration
func (sck *ShardCtrler) Query() *shardcfg.ShardConfig {
	// Your code here.
	sck.mu.Lock()
	defer sck.mu.Unlock()
	DPrintf("SCK: Query() is invoked")

	cfgStr, _, _ := sck.IKVClerk.Get("config")

	return shardcfg.FromString(cfgStr)
}

