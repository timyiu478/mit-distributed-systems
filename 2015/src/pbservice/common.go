package pbservice

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongServer = "ErrWrongServer"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	// You'll have to add definitions here.
	Op    string
	ClientId int64
	SeqNum   int
	Viewnum uint

	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
	Viewnum uint
}

type GetReply struct {
	Err   Err
	Value string
}


// Your RPC definitions here.
type ForwardArgs struct {
	Me string
	Viewnum   uint 
	PAArgs PutAppendArgs
	Op string
}

type ForwardReply struct {
	Err   Err
}

type TransferStateArgs struct {
	Me string
	Viewnum   uint 
	Kvs 			map[string]string
	DupTable	map[int64]int
}

type TransferStateReply struct {
	Err   Err
}
