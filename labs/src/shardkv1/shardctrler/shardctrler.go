package shardctrler

//
// Shardctrler with InitConfig, Query, and ChangeConfigTo methods
//

import (
	"fmt"
	"log"
	"sync"
	"time"
	"slices"
	"sync/atomic"

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

var shardCtrlerId atomic.Int64

// ShardCtrler for the controller and kv clerk.
type ShardCtrler struct {
	clnt *tester.Clnt
	kvtest.IKVClerk

	killed int32 // set by Kill()

	// Your data here.
	mu sync.Mutex

	cks 						map[tester.Tgid]*shardgrp.Clerk // map gid to shard group clerk
	gidToServers		map[tester.Tgid][]string

	id              int64
}

// Make a ShardCltler, which stores its state in a kvsrv.
func MakeShardCtrler(clnt *tester.Clnt) *ShardCtrler {
	sck := &ShardCtrler{clnt: clnt}
	srv := tester.ServerName(tester.GRP0, 0)
	sck.IKVClerk = kvsrv.MakeClerk(clnt, srv)
	sck.cks = make(map[tester.Tgid]*shardgrp.Clerk)
	sck.gidToServers = make(map[tester.Tgid][]string)
	sck.id = shardCtrlerId.Add(1)
	// Your code here.
	return sck
}

// The tester calls InitController() before starting a new
// controller. In part A, this method doesn't need to do anything. In
// B and C, this method implements recovery.
func (sck *ShardCtrler) InitController() {
	sck.mu.Lock()
	defer sck.mu.Unlock()

	oldConfig, _, oldErr := sck.IKVClerk.Get("config")
	newConfig, _, newErr := sck.IKVClerk.Get("new-config")

	if newErr == rpc.OK && oldErr == rpc.OK {
		if newConfig != oldConfig {
			sck.changeConfigTo(shardcfg.FromString(newConfig))
		}
	}
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
	sck.IKVClerk.Put("new-config", cfg.String(), 0)
}

// Called by the tester to ask the controller to change the
// configuration from the current one to new.  While the controller
// changes the configuration it may be superseded by another
// controller.
func (sck *ShardCtrler) ChangeConfigTo(new *shardcfg.ShardConfig) {
	// Your code here.
	sck.mu.Lock()
	defer sck.mu.Unlock()

	sck.changeConfigTo(new)
}


// Return the current configuration
func (sck *ShardCtrler) Query() *shardcfg.ShardConfig {
	// Your code here.
	sck.mu.Lock()
	defer sck.mu.Unlock()
	// DPrintf(fmt.Sprintf("SCK %d: Query() is invoked", sck.id))

	cfgStr, _, _ := sck.IKVClerk.Get("config")

	return shardcfg.FromString(cfgStr)
}


func (sck *ShardCtrler) changeConfigTo(new *shardcfg.ShardConfig) {
	// Stores the next configuration
	storedNew, newVer, newErr := sck.IKVClerk.Get("new-config")
	if newErr == rpc.OK {
		storedCfg := shardcfg.FromString(storedNew)
		// Note: we allow multiple controllers post the same next configuration
		if storedCfg.Num > new.Num || storedCfg.Num == new.Num && storedNew != new.String() {
			DPrintf(fmt.Sprintf("SCK %d: only one controller can post a next configuration for a configuration Num %d", sck.id, new.Num))
			return
		}
	} else {
		DPrintf(fmt.Sprintf("SCK %d: failed to get stored new config", sck.id))
		return
	}
	err := sck.IKVClerk.Put("new-config", new.String(), newVer)
	if err != rpc.OK {
		DPrintf(fmt.Sprintf("SCK %d: failed to put new config", sck.id))
		return
	}

	for {

		cfgStr, version, err := sck.IKVClerk.Get("config")

		if err != rpc.OK {
			DPrintf(fmt.Sprintf("SCK %d: failed to get current config", sck.id))
			return
		}

		oldConfig := shardcfg.FromString(cfgStr)

		storedNew, _, newErr := sck.IKVClerk.Get("new-config")

		if newErr != rpc.OK || storedNew != new.String() {
			DPrintf(fmt.Sprintf("SCK %d: stored new config is overwritten", sck.id))
			return
		}

		if oldConfig.Num >= new.Num {
			DPrintf(fmt.Sprintf("SCK %d: current config Num (%d) >= new config Num (%d)", sck.id, oldConfig.Num, new.Num))
			return
		}

		errCount := 0

		for s := 0; s < shardcfg.NShards; s++ {
			time.Sleep(time.Duration(20) * time.Millisecond)

			shard := shardcfg.Tshid(s)

			oldGid, oldServers, oldOk := oldConfig.GidServers(shard)
			newGid, newServers, newOk := new.GidServers(shard)

			if newOk && oldOk {
				if oldGid == newGid {
					// DPrintf(fmt.Sprintf("SCK %d: the gid of shard %d remains unchange", sck.id, s))
					continue
				}
				DPrintf(fmt.Sprintf("SCK %d: change shard %d from current gid %d to new gid %d", sck.id, s, oldGid, newGid))

				oldShardGrpCk, oldOk := sck.cks[oldGid]
				newShardGrpCk, newOk := sck.cks[newGid]

				if !oldOk || !slices.Equal(oldServers, sck.gidToServers[oldGid]) {
					oldShardGrpCk = shardgrp.MakeClerk(sck.clnt, oldServers)
					sck.cks[oldGid] = oldShardGrpCk
				}
				if !newOk || !slices.Equal(newServers, sck.gidToServers[newGid]) {
					newShardGrpCk = shardgrp.MakeClerk(sck.clnt, newServers)
					sck.cks[newGid] = newShardGrpCk
				}

				state, freezeErr := oldShardGrpCk.FreezeShard(shard, new.Num)

				if freezeErr != rpc.OK {
					DPrintf(fmt.Sprintf("SCK %d: failed to freezed shard %d", sck.id, s))
					errCount++
					continue
				}

				inShdErr := newShardGrpCk.InstallShard(shard, state, new.Num)

				if inShdErr != rpc.OK && inShdErr != rpc.ErrVersion {
					DPrintf(fmt.Sprintf("SCK %d: failed to install shard %d to group %d, err is %s", sck.id, s, newGid, inShdErr))
					errCount++
					continue
				}

				delShdErr := oldShardGrpCk.DeleteShard(shard, new.Num)

				if delShdErr != rpc.OK && delShdErr != rpc.ErrVersion {
					DPrintf(fmt.Sprintf("SCK %d: failed to delete shard %d from group %d, err is %s", sck.id, s, oldGid, delShdErr))
					errCount++
				}
			}
		}

		if errCount > 0 {
			DPrintf(fmt.Sprintf("SCK %d: error count > 0 => retry to change config again", sck.id))
			time.Sleep(time.Duration(50) * time.Millisecond)
			continue
		}

		putErr := sck.IKVClerk.Put("config", new.String(), version)
		if putErr != rpc.OK {
			DPrintf(fmt.Sprintf("SCK %d: fail to put new config, version is %d", sck.id, version))
		}
		return
	}
}
