* When the leader election thread acquired the mutex lock, the RPC handlers can't process because the handlers also require the mutex lock.
