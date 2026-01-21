package paxos

const (
	OK             = "OK"
	ErrReject      = "ErrReject"
)

type Err string

type Proposal struct {
	Num       int
	Id        int
}

func (p1 Proposal) lessThan(p2 Proposal) bool {
	if p1.Num < p2.Num { return true }
	if p1.Num > p2.Num { return false }
	if p1.Id < p2.Id { return true }

	return false
}

func (p1 Proposal) lessThanOrEqual(p2 Proposal) bool {
	if p1.lessThan(p2) || p1.Num == p2.Num && p1.Id == p2.Id {
		return true
	}

	return false
}

type PrepareArgs struct {
	Me  		  int
	SeqNum  	int  // Instance Sequence Number
	DoneSeq   int  // Highest Done argument supplied by its local application
	Num  			Proposal
}

type PrepareReply struct {
	Me  		int
	DoneSeq int
	Na     	Proposal
	Va   		interface{} // Value Accepted
	HasVa   bool
	Err 		Err
}

type DecidedArgs struct {
	Me  		int
	SeqNum 	int
	DoneSeq int
	Value   interface{}
}

type DecidedReply struct {
	Me  		int
	DoneSeq int
}


type AcceptArgs struct {
	Me  		int
	SeqNum 	int
	DoneSeq int
	Num     Proposal
	Value   interface{}
}

type AcceptReply struct {
	Me  		int
	DoneSeq int
	Err 		Err
}

type DoneArgs struct {
	Me  		int
	DoneSeq int
}

type DoneReply struct {
	Me  		int
	DoneSeq int
}
