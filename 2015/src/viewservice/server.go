package viewservice

import "net"
import "net/rpc"
import "log"
import "time"
import "sync"
import "fmt"
import "os"
import "sync/atomic"
	
type ServerState int

const (
    StateOk ServerState = iota
    StateRestarted
    StateError
)

type ViewServer struct {
	mu       sync.Mutex
	l        net.Listener
	dead     int32 // for testing
	rpccount int32 // for testing
	me       string

	// Your declarations here.
	// keep track of the most recent time at which the viewservice has heard a Ping from each server
	lastPings map[string]time.Time
	// last Viewnum this server pinged with
	lastViewnum map[string]uint
	// Keep track of the return view
	viewnum     uint
	primary     string
	backup      string
	// keep track of whether the primary for the current view has acknowledged it
	// Whether the view service is initialized
	inited       bool
	// Whether the primary is acknowledged the current view
	primaryAck   bool
}

//
// server Ping RPC handler.
//
func (vs *ViewServer) Ping(args *PingArgs, reply *PingReply) error {

	// Your code here.
	vs.mu.Lock()
	defer vs.mu.Unlock()

	log.Println("Ping from server", args.Me, "view is ", args.Viewnum)

	vs.lastPings[args.Me] = time.Now()
	vs.lastViewnum[args.Me] = args.Viewnum

	if !vs.inited {

		vs.inited = true
		vs.viewnum = args.Viewnum + 1
		vs.primary = args.Me

		log.Println("initialized the view service, the primary is", args.Me)

		reply.View.Viewnum = vs.viewnum
		reply.View.Primary = vs.primary
		reply.View.Backup = vs.backup

		return nil
	} 

	if vs.viewnum == args.Viewnum && vs.primary == args.Me {
		vs.primaryAck = true
		log.Println("primary", vs.primary, "is acknowledged view", vs.viewnum)
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
	vs.lastViewnum = make(map[string]uint)
	vs.inited 			= false
	vs.primaryAck 	= false
	vs.primary = ""
	vs.backup = ""

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
// Change view if primary is acknowledged and the primary/backupstatus change 
//
func (vs *ViewServer) changeView() {
	// No ACK from primary
	if !vs.primaryAck {
		return
	}

	log.Println("viewnum is", vs.viewnum,", primary is", vs.primary,", backup is", vs.backup)

	viewChanged := false

	// Handle primary if primary is not alive or is restarted
	if !vs.isAlive(vs.primary) {
		// Promote backup as primary
		if vs.isAlive(vs.backup) {
			log.Println("Promote backup", vs.backup, "as primary")
			vs.primary = vs.backup
			vs.backup = "" // trigger backup replacement
			viewChanged = true
		}
		// No safe backup after the primary is initialized and failed
	 	// In this case, the data is lost
	 	// change vs.primary to reflect the failure
		if !viewChanged && vs.primary != "" {
			vs.primary = ""
			viewChanged = true
		}
	}

	// Handle Backup if primary is alive(and not restarted) and backup is not alive in current view
	if vs.isAlive(vs.primary) && !vs.isAlive(vs.backup) {
		if vs.backup != "" {
			vs.backup = ""
			viewChanged = true
		}
		for server, _ := range vs.lastPings {
			// Allow re-select the restarted backup as new backup in the immediate next view
			if server == vs.primary || vs.isDead(server) { continue }
			log.Println("Select idle(restarted) server", server, "as backup in view", vs.viewnum + 1)
			vs.backup = server
			vs.lastViewnum[server] = vs.viewnum
			viewChanged = true
			break
		}
	}

	if viewChanged {
		log.Println("change view from", vs.viewnum, "to", vs.viewnum + 1, "primary is", vs.primary, ", backup is", vs.backup)
		vs.viewnum += 1
		vs.primaryAck = false
	}
}

// 
// Check if the server is dead
//
func (vs *ViewServer) isDead(server string) bool {
	if server == "" { return true }
	lastTime, ok := vs.lastPings[server]
	if !ok { return true }
  if time.Since(lastTime) > DeadPings*PingInterval { return true }
	return false
}

// 
// Check if the server is alive and not restarted
//
func (vs *ViewServer) isAlive(server string) bool {
	if vs.isDead(server) { return false }
	return vs.lastViewnum[server] != 0
}
