package rpc

type Err string

const (
	// Err's returned by server and Clerk
	OK         = "OK"
	ErrNoKey   = "ErrNoKey"
	ErrVersion = "ErrVersion"

	// Err returned by Clerk only
	ErrMaybe = "ErrMaybe"

	// For future kvraft lab
	ErrWrongLeader = "ErrWrongLeader"
	ErrWrongGroup  = "ErrWrongGroup"
)

type Tversion uint64

type PutArgs struct {
	Key      string
	Value    string
	Version  Tversion
	ClientId uint64
	SeqNum   int
}

type PutReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	ClientId uint64
	SeqNum   int
}

type GetReply struct {
	Value   string
	Version Tversion
	Err     Err
}

