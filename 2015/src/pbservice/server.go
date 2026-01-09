package pbservice

import "net"
import "fmt"
import "net/rpc"
import "log"
import "time"
import "lab/viewservice"
import "sync"
import "sync/atomic"
import "os"
import "syscall"
import "math/rand"


const Debug = true

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type PBServer struct {
	mu         sync.Mutex
	l          net.Listener
	dead       int32 // for testing
	unreliable int32 // for testing
	me         string
	vs         *viewservice.Clerk
	// Your declarations here.
	Kvs 							map[string]string

	DupTable    			map[int64]int // duplicate table; entry per client

	view              viewservice.View // cached View

	stateTransfered bool // whether primary transfered state to backup
}


func (pb *PBServer) Get(args *GetArgs, reply *GetReply) error {

	// Your code here.
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.view.Primary != pb.me {
		fmt.Printf("Pb %v: received Get operation from client but this server is not the primary", pb.me)
		reply.Err = ErrWrongServer
		return nil
	}

	pb.get(args, reply)

	return nil
}


func (pb *PBServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) error {

	// Your code here.
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if pb.view.Primary != pb.me {
		fmt.Printf("Pb %v: received Get operation from client %d but this server is not the primary", pb.me, args.ClientId)
		reply.Err = ErrWrongServer
		return nil
	}

	seqNum := pb.DupTable[args.ClientId]

	if args.SeqNum <= seqNum {
		fmt.Printf("Pb %v: duplicated PutAppend operation from client %d, args.seqNum is %d, seqNum is %d", pb.me, args.ClientId, args.SeqNum, seqNum)
		reply.Err = OK
		return nil
	}

	// Forward operation to backup if state transfered
	if pb.stateTransfered {
		fmt.Printf("Pb %v: forward operation to backup %v", pb.me, pb.view.Backup)
		ok := pb.forward(*args)
		if !ok {
			reply.Err = ErrWrongServer
			return nil
		}
	}

	// Put or append
	pb.putAppend(args, reply)

	// Update state for deduplication
	pb.DupTable[args.ClientId] = args.SeqNum

	return nil
}


//
// ping the viewserver periodically.
// if view changed:
//   transition to new view.
//   manage transfer of state from primary to new backup.
//
func (pb *PBServer) tick() {

	// Your code here.
	pb.mu.Lock()
	defer pb.mu.Unlock()

	view, err := pb.vs.Ping(pb.view.Viewnum)

	// Unable to get reply from the view server
	if err != nil {
		return
	}

	// Transite to new view
	if view.Viewnum != pb.view.Viewnum {
		pb.stateTransfered = false
		pb.view = view
	}

	// Transfer state from primary to new backup
	if !pb.stateTransfered && view.Primary == pb.me && view.Backup != "" {
		args := &TransferStateArgs{}
		reply := &TransferStateReply{}

		args.Me = pb.me
		args.Kvs = pb.Kvs
		args.DupTable = pb.DupTable

		fmt.Printf("Pb %v: transfer state to backup %v", pb.me, pb.view.Backup)

		ok := call(pb.view.Backup, "PBServer.TransferState", args, reply)

		if ok && reply.Err == OK {
			pb.stateTransfered = true
		}
	}

}

func (pb *PBServer) TransferState(args *TransferStateArgs, reply *TransferStateReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if args.Me != pb.view.Primary || pb.view.Backup != pb.me {
		reply.Err = ErrWrongServer
		return nil
	}

	reply.Err = OK

	if pb.stateTransfered {
		return nil
	}

	pb.Kvs = args.Kvs
	pb.DupTable = args.DupTable

	pb.stateTransfered = true

	return nil
}

func (pb *PBServer) Forward(args *ForwardArgs, reply *ForwardReply) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if args.Me != pb.view.Primary || pb.view.Backup != pb.me || !pb.stateTransfered {
		reply.Err = ErrWrongServer
		return nil
	}

	pReply := &PutAppendReply{}

	// Put or append
	pb.putAppend(&args.PAArgs, pReply)

	reply.Err = pReply.Err

	// Update state for deduplication
	pb.DupTable[args.PAArgs.ClientId] = args.PAArgs.SeqNum

	return nil
}

// tell the server to shut itself down.
// please do not change these two functions.
func (pb *PBServer) kill() {
	atomic.StoreInt32(&pb.dead, 1)
	pb.l.Close()
}

// call this to find out if the server is dead.
func (pb *PBServer) isdead() bool {
	return atomic.LoadInt32(&pb.dead) != 0
}

// please do not change these two functions.
func (pb *PBServer) setunreliable(what bool) {
	if what {
		atomic.StoreInt32(&pb.unreliable, 1)
	} else {
		atomic.StoreInt32(&pb.unreliable, 0)
	}
}

func (pb *PBServer) isunreliable() bool {
	return atomic.LoadInt32(&pb.unreliable) != 0
}


func StartServer(vshost string, me string) *PBServer {
	pb := new(PBServer)
	pb.me = me
	pb.vs = viewservice.MakeClerk(me, vshost)
	// Your pb.* initializations here.
	pb.Kvs = make(map[string]string)
	pb.DupTable = make(map[int64]int)
	pb.view = viewservice.View{0, "", ""}
	pb.stateTransfered = false

	rpcs := rpc.NewServer()
	rpcs.Register(pb)

	os.Remove(pb.me)
	l, e := net.Listen("unix", pb.me)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	pb.l = l

	// please do not change any of the following code,
	// or do anything to subvert it.

	go func() {
		for pb.isdead() == false {
			conn, err := pb.l.Accept()
			if err == nil && pb.isdead() == false {
				if pb.isunreliable() && (rand.Int63()%1000) < 100 {
					// discard the request.
					conn.Close()
				} else if pb.isunreliable() && (rand.Int63()%1000) < 200 {
					// process the request but force discard of reply.
					c1 := conn.(*net.UnixConn)
					f, _ := c1.File()
					err := syscall.Shutdown(int(f.Fd()), syscall.SHUT_WR)
					if err != nil {
						fmt.Printf("shutdown: %v\n", err)
					}
					go rpcs.ServeConn(conn)
				} else {
					go rpcs.ServeConn(conn)
				}
			} else if err == nil {
				conn.Close()
			}
			if err != nil && pb.isdead() == false {
				fmt.Printf("PBServer(%v) accept: %v\n", me, err.Error())
				pb.kill()
			}
		}
	}()

	go func() {
		for pb.isdead() == false {
			pb.tick()
			time.Sleep(viewservice.PingInterval)
		}
	}()

	return pb
}

func (pb *PBServer) forward(args PutAppendArgs) bool {
	fargs := &ForwardArgs{}
	freply := &ForwardReply{}

	fargs.Me = pb.me
	fargs.PAArgs = args

	ok := call(pb.view.Backup, "PBServer.Forward", fargs, freply)

	return ok && freply.Err == OK
}

func (pb *PBServer) get(args *GetArgs, reply *GetReply) {
	val, ok := pb.Kvs[args.Key]
	if ok {
		reply.Err   = OK
		reply.Value = val
		return
	}
	reply.Err   = ErrNoKey
}

func (pb *PBServer) putAppend(args *PutAppendArgs, reply *PutAppendReply) {
	oldval, ok := pb.Kvs[args.Key]

	switch args.Op {
		case "Put":
			pb.Kvs[args.Key] = args.Value
		case "Append":
			if ok {
				pb.Kvs[args.Key] = oldval + args.Value
			} else {
				pb.Kvs[args.Key] = args.Value
			}
	}

	reply.Err = OK
}
