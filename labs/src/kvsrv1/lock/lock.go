package lock

import (
	"time"
	"6.5840/kvtest1"
	"6.5840/kvsrv1/rpc"
)

type Lock struct {
	// IKVClerk is a go interface for k/v clerks: the interface hides
	// the specific Clerk type of ck but promises that ck supports
	// Put and Get.  The tester passes the clerk in when calling
	// MakeLock().
	ck kvtest.IKVClerk

	// You may add code here
	clientID string
	lockKey string
}

// The tester calls MakeLock() and passes in a k/v clerk; your code can
// perform a Put or Get by calling lk.ck.Put() or lk.ck.Get().
//
// Use l as the key to store the "lock state" (you would have to decide
// precisely what the lock state is).
func MakeLock(ck kvtest.IKVClerk, l string) *Lock {
	lk := &Lock{ck: ck, clientID: kvtest.RandValue(8), lockKey: l}

	return lk
}

func (lk *Lock) Acquire() {
	for {
		value, version, err := lk.ck.Get(lk.lockKey)
		if err == rpc.ErrNoKey || value == "free" {
			lk.ck.Put(lk.lockKey, lk.clientID, version)
		} else if value == lk.clientID {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (lk *Lock) Release() {
	for {
		value, version, err := lk.ck.Get(lk.lockKey)
		if err == rpc.OK && value == lk.clientID {
			lk.ck.Put(lk.lockKey, "free", version)
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}
