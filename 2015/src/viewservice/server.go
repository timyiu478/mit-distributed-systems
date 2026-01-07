package viewservice

import "net"
import "net/rpc"
import "log"
import "time"
import "sync"
import "fmt"
import "os"
import "sync/atomic"

type ViewServer struct {
	mu       sync.Mutex
	l        net.Listener
	dead     int32 // for testing
	rpccount int32 // for testing
	me       string

	// Your declarations here.
	// keep track of the most recent time at which the viewservice has heard a Ping from each server
	lastPings map[string]time.Time
	// Keep track of the real-time status of primary/backup
	members   map[string]bool // false -> restarted/failed
	// Keep track of the return view
	viewnum     uint
	primary     string
	backup      string
	primaryLive bool
	backupLive  bool
	// keep track of whether the primary for the current view has acknowledged it
	primaryAcked bool

	

}

//
// server Ping RPC handler.
//
func (vs *ViewServer) Ping(args *PingArgs, reply *PingReply) error {

	// Your code here.

	// The primary for the current view has acknowledged it
	if vs.viewnum == args.Viewnum && args.Me == vs.primary || (vs.viewnum == 0) {
		vs.primaryAcked = true
		if vs.viewnum == 0 {
			vs.viewnum = args.Viewnum
		}
	}

	// Update last ping time
	vs.lastPings[args.Me] = time.Now()
	vs.members[args.Me] = true

	// Viewnum = 0 represents the server has failed and re-started
	if args.Viewnum == 0 {
		vs.members[args.Me] = false
		log.Println("Server", args.Me, " has failed and re-started")
	}

	vs.changeView()

	reply.View.Viewnum = vs.viewnum
	reply.View.Primary = vs.primary
	reply.View.Backup = vs.backup

	return nil
}

//
// server Get() RPC handler.
//
func (vs *ViewServer) Get(args *GetArgs, reply *GetReply) error {

	// Your code here.
	vs.mu.Lock()
	defer vs.mu.Unlock()

	reply.View.Viewnum = vs.viewnum
	reply.View.Primary = vs.primary
	reply.View.Backup = vs.backup

	return nil
}


//
// tick() is called once per PingInterval; it should notice
// if servers have died or recovered, and change the view
// accordingly.
//
func (vs *ViewServer) tick() {

	// Your code here.
	vs.mu.Lock()
	defer vs.mu.Unlock()

	for server, lastPing := range vs.lastPings {
		vs.members[server] = true
		if lastPing.Add(PingInterval).Before(time.Now()) {
			delete(vs.lastPings, server)
			vs.members[server] = false
			log.Println("Server", server, "has no ping")
		}
	}

	vs.changeView()
}

//
// tell the server to shut itself down.
// for testing.
// please don't change these two functions.
//
func (vs *ViewServer) Kill() {
	atomic.StoreInt32(&vs.dead, 1)
	vs.l.Close()
}

//
// has this server been asked to shut down?
//
func (vs *ViewServer) isdead() bool {
	return atomic.LoadInt32(&vs.dead) != 0
}

// please don't change this function.
func (vs *ViewServer) GetRPCCount() int32 {
	return atomic.LoadInt32(&vs.rpccount)
}

func StartServer(me string) *ViewServer {
	vs := new(ViewServer)
	vs.me = me
	// Your vs.* initializations here.
	vs.lastPings = make(map[string]time.Time)
	vs.members = make(map[string]bool)
	vs.primaryAcked = false
	vs.primary = ""
	vs.backup = ""
	vs.members[""] = false

	// tell net/rpc about our RPC server and handlers.
	rpcs := rpc.NewServer()
	rpcs.Register(vs)

	// prepare to receive connections from clients.
	// change "unix" to "tcp" to use over a network.
	os.Remove(vs.me) // only needed for "unix"
	l, e := net.Listen("unix", vs.me)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	vs.l = l

	// please don't change any of the following code,
	// or do anything to subvert it.

	// create a thread to accept RPC connections from clients.
	go func() {
		for vs.isdead() == false {
			conn, err := vs.l.Accept()
			if err == nil && vs.isdead() == false {
				atomic.AddInt32(&vs.rpccount, 1)
				go rpcs.ServeConn(conn)
			} else if err == nil {
				conn.Close()
			}
			if err != nil && vs.isdead() == false {
				fmt.Printf("ViewServer(%v) accept: %v\n", me, err.Error())
				vs.Kill()
			}
		}
	}()

	// create a thread to call tick() periodically.
	go func() {
		for vs.isdead() == false {
			vs.tick()
			time.Sleep(PingInterval)
		}
	}()

	return vs
}

//
// Change view if primary is acknowledged and the primary/backupstatus can change 
//
func (vs *ViewServer) changeView() {
	// No ACK from primary
	if !vs.primaryAcked {
		return
	}

	// Count # of failures
	failCount := 0

	if !vs.members[vs.primary] {
		failCount += 1
	}
	if !vs.members[vs.backup] {
		failCount += 1
	}

	// Primary and backup are health
	if failCount == 0 {
		return
	}

	viewChanged := false

  if !vs.members[vs.primary] {
		// Promote backup as primary
		if vs.members[vs.backup] {
			vs.primary = vs.backup
			vs.members[vs.primary] = true
			vs.members[vs.backup] = false
			vs.backup = ""
			viewChanged = true
		} else if len(vs.lastPings) > 0 {
			// Select an idle(restarted) server as primary
			for server, _ := range vs.lastPings {
				vs.primary = server
				vs.members[vs.primary] = true
				break
			}
			viewChanged = true
		} else if vs.primary != "" {
			vs.primary = ""
			viewChanged = true
		}
	}

	if !vs.members[vs.backup] {
		// Select an idle(restarted) server as primary
		if len(vs.lastPings) > 1 {
			for server, _ := range vs.lastPings {
				if server == vs.primary { continue }
				vs.backup = server
				vs.members[vs.backup] = true
				viewChanged = true
				break
			}
		} else if vs.backup != "" {
			vs.backup = ""
			viewChanged = true
		}
	}

	if viewChanged {
		vs.viewnum += 1
		vs.primaryAcked = false
	}
}
