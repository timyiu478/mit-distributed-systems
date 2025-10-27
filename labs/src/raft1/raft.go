package raft

// The file raftapi/raft.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// Make() creates a new raft peer that implements the raft interface.

import (
	//	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	"6.5840/tester1"
)

type Entry struct {
	Command int
	Term    int
}

type State int

const (
	FollowerState State = iota
	CandidateState
	LeaderState
)

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *tester.Persister   // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (3A, 3B, 3C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm 	int 
	voteIdFor   	int
	log 					[]Entry

	commitIndex   int
	lastApplied   int
	lastIndexTerm int

	nextIndex 		[]int
	matchIndex 		[]int

	lastHeartbeat		 time.Time
	electionTimeout  time.Duration

	currentState  State

	applyCh 			chan raftapi.ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (3A).
	term = rf.currentTerm
	isleader = rf.currentState == LeaderState

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (3C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
}


// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (3C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

// how many bytes in Raft's persisted log?
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.RaftStateSize()
}


// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (3D).

}


// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (3A, 3B).
	Term					int
	CandidateId   int
	LastLogIndex  int
	LastLogTerm   int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (3A).
	Term					int
	VoteGranted   bool
}


// AppendEntriesArgs RPC arguments structure
type AppendEntriesArgs struct {
	Term					int
	LeaderId			int
	PrevLogIndex  int
	PrevLogTerm   int
	LeaderCommit  int
	Entries				[]Entry
}

// AppendEntriesArgs RPC reply structure
type AppendEntriesReply struct {
	Term					int
	Success				bool
}


// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	// rf.mu.Lock()
	// defer rf.mu.Unlock()

	if rf.currentTerm > args.Term {
			reply.Term = rf.currentTerm
			reply.VoteGranted = false
			return
	}
	if rf.currentTerm < args.Term {
			rf.currentTerm = args.Term
			rf.currentState = FollowerState
			rf.voteIdFor = -1
	}
	if rf.currentState == FollowerState && rf.voteIdFor == -1 && len(rf.log) - 1 <= args.LastLogIndex { // TODO: fix last log term check
			rf.voteIdFor = args.CandidateId
			reply.Term = rf.currentTerm
			reply.VoteGranted = true

			tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("vote for %d", rf.voteIdFor), "")
	}
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// send a AppendEntries RPC to a server.
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// AppendEntries RPC handler
// invoked by a librpc call
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// rf.mu.Lock()
	// defer rf.mu.Unlock()

	// update heartbeat
	rf.lastHeartbeat = time.Now()

}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (3B).


	return index, term, isLeader
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here (3A)
		// Check if a leader election should be started.

		// rf.mu.Lock()

		if rf.currentState != LeaderState && time.Now().After(rf.lastHeartbeat.Add(rf.electionTimeout)) {
			// transit to candidate state
			rf.currentState = CandidateState
			// increament current term
			rf.currentTerm += 1
			// vote for itself
			rf.voteIdFor = rf.me
			// reset election timer
			rf.lastHeartbeat = time.Now()
			// send RequestVote RPCs to all other servers
			voteCount := 1
			args := &RequestVoteArgs{
				Term: rf.currentTerm, 
				CandidateId: rf.me,
				LastLogIndex: len(rf.log) - 1,
				LastLogTerm: 0, // TODO: fix this
			}
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me { continue }
				reply := &RequestVoteReply{}
				ret := rf.sendRequestVote(i, args, reply)
				if ret {
						if reply.VoteGranted { 
							voteCount += 1 
						// discover new term
						} else if reply.Term > rf.currentTerm {
							// catch up the term
							rf.currentTerm = reply.Term
							// transit back to follower state
							rf.currentState = FollowerState	
							break
						}
				}
			}
			// reset voteIdFor
			rf.voteIdFor = -1
			// check if get majority votes
			if rf.currentState == CandidateState && voteCount > (len(rf.peers) / 2) {
				// transit to leader state	
				rf.currentState = LeaderState
				tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("become leader in term %d", rf.currentTerm), "")
			}
		}

		// rf.mu.Unlock()

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		ms := 50 + (rand.Int63() % 300)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

func (rf *Raft) heartbeat() {
	for rf.killed() == false {
		// pause for a random amount of time between 100 and 200
		// milliseconds.
		ms := 100 + (rand.Int63() % 100)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		// rf.mu.Lock()

		if rf.currentState != LeaderState { continue }

		// send heartbeat if it is leader
		for i := 0; i < len(rf.peers); i++ {
			if i == rf.me { continue }
			args := &AppendEntriesArgs{}
			reply := &AppendEntriesReply{}
			rf.sendAppendEntries(i, args, reply)
		}

		// rf.mu.Unlock()
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *tester.Persister, applyCh chan raftapi.ApplyMsg) raftapi.Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (3A, 3B, 3C).
	rf.currentTerm = 0
	rf.voteIdFor = -1
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.electionTimeout = 3 * time.Second
	rf.lastHeartbeat = time.Now()
	rf.currentState = FollowerState
	rf.nextIndex = []int{}
	rf.matchIndex = []int{}
	rf.log = []Entry{}
	rf.applyCh = applyCh

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	// start heartbeat goroutine to start hear beating
	go rf.heartbeat()

	return rf
}
