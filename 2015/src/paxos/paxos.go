package paxos

//
// Paxos library, to be included in an application.
// Multiple applications will run, each including
// a Paxos peer.
//
// Manages a sequence of agreed-on values.
// The set of peers is fixed.
// Copes with network failures (partition, msg loss, &c).
// Does not store anything persistently, so cannot handle crash+restart.
//
// The application interface:
//
// px = paxos.Make(peers []string, me string)
// px.Start(seq int, v interface{}) -- start agreement on new instance
// px.Status(seq int) (Fate, v interface{}) -- get info about an instance
// px.Done(seq int) -- ok to forget all instances <= seq
// px.Max() int -- highest instance seq known, or -1
// px.Min() int -- instances before this seq have been forgotten
//

import "net"
import "net/rpc"
import "log"

import "os"
import "syscall"
import "sync"
import "sync/atomic"
import "fmt"
import "math/rand"

import "time"

type Proposer struct {
	mu      	sync.Mutex
	me        int
	maxN    	Proposal
	prepareOk  []bool
	accepted   []bool
	val        interface{}
}

type Instance struct {
	decided bool
	inited  bool
	// acceptor state
	hasVa  bool
	np      Proposal // highest prepare seen
	na      Proposal // highest accept seen
	va 			interface{} // accepted value
}

func MakeInstance(me int) *Instance {
	in := &Instance{}

	in.decided = false
	in.hasVa 	 = false
	in.inited  = true
	in.np = Proposal{Num: -1, Id: me}
	in.na = Proposal{Num: -1, Id: me}

	return in
}

// px.Status() return values, indicating
// whether an agreement has been decided,
// or Paxos has not yet reached agreement,
// or it was agreed but forgotten (i.e. < Min()).
type Fate int

const (
	Decided   Fate = iota + 1
	Pending        // not yet decided.
	Forgotten      // decided but forgotten.
)

type Paxos struct {
	mu         sync.Mutex
	l          net.Listener
	dead       int32 // for testing
	unreliable int32 // for testing
	rpcCount   int32 // for testing
	peers      []string
	me         int // index into peers[]


	// Your data here.
	maxSeqNum int   // highest instance seq known, or -1
	minDoneSeqNum int   // a minimum over *all* Paxos peers
	offset    int   // the first sequence number of the log[0]
	majority  int
	doneSeqNums   []int // each Paxos peer's highest Done argument
	instances	map[int]*Instance
}

//
// call() sends an RPC to the rpcname handler on server srv
// with arguments args, waits for the reply, and leaves the
// reply in reply. the reply argument should be a pointer
// to a reply structure.
//
// the return value is true if the server responded, and false
// if call() was not able to contact the server. in particular,
// the replys contents are only valid if call() returned true.
//
// you should assume that call() will time out and return an
// error after a while if it does not get a reply from the server.
//
// please use call() to send all RPCs, in client.go and server.go.
// please do not change this function.
//
func call(srv string, name string, args interface{}, reply interface{}) bool {
	c, err := rpc.Dial("unix", srv)
	if err != nil {
		err1 := err.(*net.OpError)
		if err1.Err != syscall.ENOENT && err1.Err != syscall.ECONNREFUSED {
			fmt.Printf("paxos Dial() failed: %v\n", err1)
		}
		return false
	}
	defer c.Close()

	err = c.Call(name, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}


//
// the application wants paxos to start agreement on
// instance seq, with proposed value v.
// Start() returns right away; the application will
// call Status() to find out if/when agreement
// is reached.
//
func (px *Paxos) Start(seq int, v interface{}) {
	// Your code here.
	px.mu.Lock()
	defer px.mu.Unlock()

	if seq < px.min() { return }

	if seq > px.maxSeqNum {
		px.maxSeqNum = seq
	}

	// Paxos protocol
	go func(seq int, v interface{}) {
		p := MakeProposer(px.me, v, len(px.peers))
		px.startProtocol(p, seq, v) 
	}(seq, v)
}

func (px *Paxos) startProtocol(p *Proposer, seq int, v interface{}) {
	// log.Println("Px:", px.me, ": start protocol for seq", seq)

	for !px.isdead() {
		randMill := time.Duration(rand.Intn(150)) * time.Millisecond
		time.Sleep(time.Duration(50) * time.Millisecond + randMill)

		if px.isInstanceDecided(seq) { return }
		
		// Choose n, unique and higher than any n seen so far
		n := p.newN()

		// log.Println("Px", px.me, ": new n is", n.Num,"-",n.Id)

		// Reset accepted, prepareOK, maxN
		for i := 0; i < len(px.peers) && !px.isdead() ; i++ {
			p.accepted[i]  = false
			p.prepareOk[i] = false
		}
		p.maxN      = Proposal{Num: -1, Id: px.me}

		// Send prepare(n) to all servers including self
		// log.Println("Px", px.me, ": send prepare(n) to all servers for seq", seq)

		var wgPrep sync.WaitGroup
		for i := 0; i < len(px.peers) && !px.isdead() ; i++ {
			wgPrep.Add(1)

			pArgs  := &PrepareArgs{
				Me: px.me,
				SeqNum: seq,
				DoneSeq: px.doneSeqNums[px.me],
				Num: n,
			}
		
			if i == px.me {
				go func(p *Proposer, pArgs *PrepareArgs, wg *sync.WaitGroup) {
					pReply := &PrepareReply{}
					err := px.Prepare(pArgs, pReply)

					if err == nil {
						p.prepareReply(pReply)
						px.updateDoneSeq(pReply.DoneSeq, pReply.Me)
					}

					wg.Done()
				}(p, pArgs, &wgPrep)
				continue
			}

			go func(p *Proposer, peer string, pArgs *PrepareArgs, wg *sync.WaitGroup) {
				pReply := &PrepareReply{}

				ret := call(peer, "Paxos.Prepare", pArgs, pReply)

				if ret {
					p.prepareReply(pReply)
					px.updateDoneSeq(pReply.DoneSeq, pReply.Me)
				}

				wg.Done()
			}(p, px.peers[i], pArgs, &wgPrep)
		}
		wgPrep.Wait()

		if p.prepareOkCount() < px.majority {
			if p.maxN.lessThan(n) { p.maxN = n }
			continue
		}

		// Send accept(n, v') to all
		// log.Println("Px", px.me, ": send accept(n, v') to all servers for seq", seq)
		var wgAccept sync.WaitGroup
		for i := 0; i < len(px.peers) && !px.isdead() ; i++ {
			wgAccept.Add(1)

			aArgs := &AcceptArgs{
				Me: px.me,
				SeqNum: seq,
				DoneSeq: px.doneSeqNums[px.me],
				Num: n,
				Value: p.val,
			}

			if i == px.me {
				go func(p *Proposer, aArgs *AcceptArgs, wg *sync.WaitGroup) {
					aReply := &AcceptReply{}

					err := px.Accept(aArgs, aReply)

					if err == nil {
						p.acceptReply(aReply)
						px.updateDoneSeq(aReply.DoneSeq, aReply.Me)
					}

					wg.Done()
				}(p, aArgs, &wgAccept)
				continue
			}
			go func(peer string, p *Proposer, aArgs *AcceptArgs, wg *sync.WaitGroup) {
				aReply := &AcceptReply{}

				ret := call(peer, "Paxos.Accept", aArgs, aReply)

				if ret {
					p.acceptReply(aReply)
					px.updateDoneSeq(aReply.DoneSeq, aReply.Me)
				}

				wg.Done()
			}(px.peers[i], p, aArgs, &wgAccept)

		}
		wgAccept.Wait()

		if p.acceptedCount() < px.majority {
			if p.maxN.lessThan(n) { p.maxN = n }
			continue
		}
		
		// Send decided(v') to all
		// log.Println("Px", px.me, ": send decided(v') to all servers for seq", seq)
		var wgDecided sync.WaitGroup
		for i := 0; i < len(px.peers) && !px.isdead() ; i++ {
			wgDecided.Add(1)

			dArgs := &DecidedArgs{
				Me: px.me,
				SeqNum: seq,
				DoneSeq: px.doneSeqNums[px.me],
				Value: p.val,
			}

			if i == px.me {
				go func(p *Proposer, dArgs *DecidedArgs, wg *sync.WaitGroup) {
					reply := &DecidedReply{}

					err := px.Decided(dArgs, reply)

					if err == nil {
						px.updateDoneSeq(reply.DoneSeq, reply.Me)
					}

					wg.Done()
				}(p, dArgs, &wgDecided)
				continue
			}
			go func(peer string, p *Proposer, dArgs *DecidedArgs, wg *sync.WaitGroup) {
				reply := &DecidedReply{}

				ret := call(peer, "Paxos.Decided", dArgs, reply)

				if ret {
					px.updateDoneSeq(reply.DoneSeq, reply.Me)
				}

				wg.Done()
			}(px.peers[i], p, dArgs, &wgDecided)

		}
		wgDecided.Wait()
	}
}

func (px *Paxos) Prepare(args *PrepareArgs, reply *PrepareReply) error {
	px.updateDoneSeq(args.DoneSeq, args.Me) 
	px.createInstanceIfNotExist(args.SeqNum)

	px.mu.Lock()
	defer px.mu.Unlock()

	// Set default reply
	reply.Me  		= px.me
	reply.DoneSeq = px.doneSeqNums[px.me]
	reply.Na      = px.instances[args.SeqNum].na
	reply.Va 			= px.instances[args.SeqNum].va
	reply.HasVa 	= px.instances[args.SeqNum].hasVa
	reply.Err 		= ErrReject

	// Prepare OK case
	if px.instances[args.SeqNum].np.lessThan(args.Num) {
		px.instances[args.SeqNum].np = args.Num

		reply.Err = OK
	}

	return nil
}

func (px *Paxos) Accept(args *AcceptArgs, reply *AcceptReply) error {
	px.updateDoneSeq(args.DoneSeq, args.Me) 
	px.createInstanceIfNotExist(args.SeqNum)
	
	px.mu.Lock()
	defer px.mu.Unlock()

	// Default Reply
	reply.Me  		= px.me
	reply.DoneSeq = px.doneSeqNums[px.me]
	reply.Err 		= ErrReject

	if !px.instances[args.SeqNum].np.lessThanOrEqual(args.Num) {
		return nil
	}

	// log.Println("Px", px.me,": accept proprosal", args.Num.Num,"-",args.Num.Id, "for seq", args.SeqNum) 

	px.instances[args.SeqNum].np 		= args.Num
	px.instances[args.SeqNum].na 		= args.Num
	px.instances[args.SeqNum].va 		= args.Value
	px.instances[args.SeqNum].hasVa = true

	reply.Err 		= OK

	return nil
}

func (px *Paxos) Decided(args *DecidedArgs, reply *DecidedReply) error {
	px.updateDoneSeq(args.DoneSeq, args.Me) 
	px.createInstanceIfNotExist(args.SeqNum)

	px.mu.Lock()
	defer px.mu.Unlock()

	// Default Reply
	reply.Me  		= px.me
	reply.DoneSeq = px.doneSeqNums[px.me]

	in := px.instances[args.SeqNum]

	if in.decided { return nil }

	in.va = args.Value
	in.decided = true

	return nil
}

//
// the application on this machine is done with
// all instances <= seq.
//
// see the comments for Min() for more explanation.
//
func (px *Paxos) Done(seq int) {
	// Your code here.
	// log.Println("Px", px.me, ": Done(seq =", seq,")")
	px.updateDoneSeq(seq, px.me)
	px.broadcastDoneSeqNum()
}

//
// the application wants to know the
// highest instance sequence known to
// this peer.
//
func (px *Paxos) Max() int {
	// Your code here.
	px.mu.Lock()
	defer px.mu.Unlock()

	return px.maxSeqNum
}

//
// Min() should return one more than the minimum among z_i,
// where z_i is the highest number ever passed
// to Done() on peer i. A peers z_i is -1 if it has
// never called Done().
//
// Paxos is required to have forgotten all information
// about any instances it knows that are < Min().
// The point is to free up memory in long-running
// Paxos-based servers.
//
// Paxos peers need to exchange their highest Done()
// arguments in order to implement Min(). These
// exchanges can be piggybacked on ordinary Paxos
// agreement protocol messages, so it is OK if one
// peers Min does not reflect another Peers Done()
// until after the next instance is agreed to.
//
// The fact that Min() is defined as a minimum over
// *all* Paxos peers means that Min() cannot increase until
// all peers have been heard from. So if a peer is dead
// or unreachable, other peers Min()s will not increase
// even if all reachable peers call Done. The reason for
// this is that when the unreachable peer comes back to
// life, it will need to catch up on instances that it
// missed -- the other peers therefor cannot forget these
// instances.
//
func (px *Paxos) Min() int {
	// You code here.
	px.mu.Lock()
	defer px.mu.Unlock()

	return px.min()
}

func (px *Paxos) min() int {
	return px.minDoneSeqNum + 1
}

//
// the application wants to know whether this
// peer thinks an instance has been decided,
// and if so what the agreed value is. Status()
// should just inspect the local peer state;
// it should not contact other Paxos peers.
//
func (px *Paxos) Status(seq int) (Fate, interface{}) {
	// Your code here.
	px.mu.Lock()
	defer px.mu.Unlock()

	if seq < px.offset {
		return Forgotten, nil
	}

	in, ok := px.instances[seq]

	if !ok || !in.decided {
		return Pending, nil
	}

	return Decided, in.va
}



//
// tell the peer to shut itself down.
// for testing.
// please do not change these two functions.
//
func (px *Paxos) Kill() {
	atomic.StoreInt32(&px.dead, 1)
	if px.l != nil {
		px.l.Close()
	}
}

//
// has this peer been asked to shut down?
//
func (px *Paxos) isdead() bool {
	return atomic.LoadInt32(&px.dead) != 0
}

// please do not change these two functions.
func (px *Paxos) setunreliable(what bool) {
	if what {
		atomic.StoreInt32(&px.unreliable, 1)
	} else {
		atomic.StoreInt32(&px.unreliable, 0)
	}
}

func (px *Paxos) isunreliable() bool {
	return atomic.LoadInt32(&px.unreliable) != 0
}

//
// the application wants to create a paxos peer.
// the ports of all the paxos peers (including this one)
// are in peers[]. this servers port is peers[me].
//
func Make(peers []string, me int, rpcs *rpc.Server) *Paxos {
	px := &Paxos{}
	px.peers = peers
	px.me = me

	// Your initialization code here.
	px.maxSeqNum = -1
	px.minDoneSeqNum = -1
	px.offset		 = 0
	px.majority  = len(peers) / 2 + 1
	px.doneSeqNums   = make([]int, len(peers))
	px.instances = make(map[int]*Instance)

	for i := 0; i < len(peers); i++ {
		px.doneSeqNums[i] = -1
	}

	if rpcs != nil {
		// caller will create socket &c
		rpcs.Register(px)
	} else {
		rpcs = rpc.NewServer()
		rpcs.Register(px)

		// prepare to receive connections from clients.
		// change "unix" to "tcp" to use over a network.
		os.Remove(peers[me]) // only needed for "unix"
		l, e := net.Listen("unix", peers[me])
		if e != nil {
			log.Fatal("listen error: ", e)
		}
		px.l = l

		// please do not change any of the following code,
		// or do anything to subvert it.

		// create a thread to accept RPC connections
		go func() {
			for px.isdead() == false {
				conn, err := px.l.Accept()
				if err == nil && px.isdead() == false {
					if px.isunreliable() && (rand.Int63()%1000) < 100 {
						// discard the request.
						conn.Close()
					} else if px.isunreliable() && (rand.Int63()%1000) < 200 {
						// process the request but force discard of reply.
						c1 := conn.(*net.UnixConn)
						f, _ := c1.File()
						err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
						if err != nil {
							fmt.Printf("shutdown: %v\n", err)
						}
						atomic.AddInt32(&px.rpcCount, 1)
						go rpcs.ServeConn(conn)
					} else {
						atomic.AddInt32(&px.rpcCount, 1)
						go rpcs.ServeConn(conn)
					}
				} else if err == nil {
					conn.Close()
				}
				if err != nil && px.isdead() == false {
					fmt.Printf("Paxos(%v) accept: %v\n", me, err.Error())
				}
			}
		}()
	}

	go func() {
		for px.isdead() == false{ 
			px.broadcastDoneSeqNum()
			time.Sleep(time.Duration(1000) * time.Millisecond)
		}
	}()

	go func() {
		for px.isdead() == false{ 
			px.forgetInstances()
			time.Sleep(time.Duration(400) * time.Millisecond)
		}
	}()


	return px
}

func (px *Paxos) forgetInstances() {
	px.mu.Lock()
	defer px.mu.Unlock()

	if px.offset > px.minDoneSeqNum { return }

	for seq := px.offset; seq <= px.minDoneSeqNum; seq++ {
		delete(px.instances, seq)
	}

	px.offset = px.minDoneSeqNum + 1
}

//
// Broadcast the highest Done argument supplied by its local application
//
func (px *Paxos) broadcastDoneSeqNum() {
	px.mu.Lock()
	defer px.mu.Unlock()

	doneSeq := px.doneSeqNums[px.me]

	if doneSeq < 0 { return }

	for i := 0; i < len(px.peers); i++ {
		if i == px.me { continue }

		go func(peer string, me int, doneSeq int) {
			args := &DoneArgs{Me: me, DoneSeq: doneSeq}
			reply := &DoneReply{}

			ret := call(peer, "Paxos.DoneSeq", args, reply)

			if ret {
				px.handleDoneSeqReply(reply)
			}
		}(px.peers[i], px.me, doneSeq)
	}
}

func (px *Paxos) DoneSeq(args *DoneArgs, reply *DoneReply) error {
	px.updateDoneSeq(args.DoneSeq, args.Me)

	px.mu.Lock()
	defer px.mu.Unlock()

	reply.Me = px.me
	reply.DoneSeq = px.doneSeqNums[px.me]

	return nil
}

func (px *Paxos) handleDoneSeqReply(reply *DoneReply) {
	px.updateDoneSeq(reply.DoneSeq, reply.Me)
}

func (px *Paxos) isInstanceDecided(seq int) bool {
	px.mu.Lock()
	defer px.mu.Unlock()

	in, ok := px.instances[seq]

	if ok && in.decided {
		return true
	}

	return false
}


func (px *Paxos) updateDoneSeq(doneSeq int, peerIdx int) {
	px.mu.Lock()
	defer px.mu.Unlock()
	
	if doneSeq > px.doneSeqNums[peerIdx] {
		px.doneSeqNums[peerIdx] = doneSeq

		minSeqNum := px.doneSeqNums[px.me]

		for _, seqNum := range px.doneSeqNums {
			if seqNum < minSeqNum {
				minSeqNum = seqNum
			}
		}

		px.minDoneSeqNum = minSeqNum
		
		// log.Println("Px ",px.me, ": updated done seq to", px.minDoneSeqNum)
	}
}

func (px *Paxos) createInstanceIfNotExist(seq int) {
	px.mu.Lock()
	defer px.mu.Unlock()

	in, ok := px.instances[seq]

	if !ok || !in.inited {
		inst := MakeInstance(px.me)
		px.instances[seq] = inst
	}
}

func MakeProposer(me int, val interface{}, peerLen int) *Proposer {
	p := &Proposer{}	

	p.accepted 	= make([]bool, peerLen)
	p.prepareOk = make([]bool, peerLen)
	p.maxN      = Proposal{Num: -1, Id: me}
	p.me       	= me
	p.val      	= val

	return p
}

func (p *Proposer) newN() Proposal {
	p.mu.Lock()
	defer p.mu.Unlock()

	return Proposal{Num: p.maxN.Num + 1, Id: p.me}
}

func (p *Proposer) prepareReply(reply *PrepareReply) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if reply.Err == ErrReject { 
		return
	}

	if reply.Err != OK {
		// log.Println("Proposer", p.me, ": incorrect prepare reply err -", reply.Err)
		return
	}
	
	p.prepareOk[reply.Me] = true

	if p.maxN.lessThan(reply.Na) {
		p.maxN = reply.Na
		if reply.HasVa {
			p.val  = reply.Va
		}
	}
}

func (p *Proposer) acceptReply(reply *AcceptReply) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if reply.Err == ErrReject { 
		return
	}

	if reply.Err != OK {
		// log.Println("Proposer", p.me, ": incorrect accept reply err -", reply.Err)
		return
	}

	p.accepted[reply.Me] = true
}

func (p *Proposer) prepareOkCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0

	for _, ok := range p.prepareOk {
		if ok { count++ }
	}

	// log.Println("Prepare OK count is ", count)

	return count
}

func (p *Proposer) acceptedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0

	for _, ok := range p.accepted {
		if ok { count++ }
	}

	// log.Println("Accept count is ", count)

	return count
}
