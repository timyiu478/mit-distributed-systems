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
	"slices"

	//	"6.5840/labgob"
	"6.5840/labrpc"
	"6.5840/raftapi"
	"6.5840/tester1"
)

type Entry struct {
	Command interface{}
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
	voteCount     int
	commitIndex   int
	lastApplied   int

	nextIndex 		[]int // for each server, index of the next log entry to send to that server
	matchIndex 		[]int // for each server, index of highest log entry known to be replicated on server

	log 					[]Entry // 0-indexed

	lastHeartbeat		 						time.Time
	electionTimeoutLowerBound  	time.Duration

	currentState  State

	applyCh 							chan raftapi.ApplyMsg
	
	requestVoteReplyCh 		chan *RequestVoteReply
	appendEntriesReplyCh 	chan *AppendEntriesReply
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (3A).
	rf.mu.Lock()
	defer rf.mu.Unlock()

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
	PeerId 				int
	PrevLogIndex  int
	EntriesLength int
	Success				bool
}


// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("recieved RV RPC in term %d", rf.currentTerm), "")

	// default reply
	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	// deny request from older term
	if rf.currentTerm > args.Term { 
		return
	}

	// adopt the newer term before handle the RPC
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.voteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		reply.Term = args.Term
	}

	// If votedFor is null or candidateId, and candidate’s log is at
	// least as up-to-date as receiver’s log, grant vote
	// See the section 5.4.1 of the paper for the definition of "up-to-date"
	lastLogIndex := len(rf.log) - 1
	if (rf.voteIdFor == -1 || rf.voteIdFor == args.CandidateId) && (lastLogIndex <= args.LastLogIndex && rf.log[lastLogIndex].Term == args.LastLogTerm || rf.log[lastLogIndex].Term < args.LastLogTerm) {
		// vote for first valid candidate 
		rf.voteIdFor = args.CandidateId
		// reset election timer
		rf.lastHeartbeat = time.Now()

		reply.VoteGranted = true

		tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("vote for %d in term %d", rf.voteIdFor, rf.currentTerm), "")
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("recieved AE RPC in term %d", rf.currentTerm), "")

	// default reply
	reply.Term = rf.currentTerm
	reply.Success = false
	reply.PeerId = rf.me
	reply.PrevLogIndex = args.PrevLogIndex
	reply.EntriesLength = len(args.Entries)

	// deny request from older term
	if rf.currentTerm > args.Term {
		return
	}

	// update heartbeat
	rf.lastHeartbeat = time.Now()

	// transit to follower state if discover higher term
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.voteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		reply.Term = args.Term
	} else if rf.currentState == CandidateState && rf.currentTerm == args.Term { // transit to follower state if discover leader in this term
		// DO NOT reset the voteIdFor
		// if a server clears voteIdFor while staying in the same term,
		// it can later grant a second vote in the same term to another candidate
		rf.voteIdFor = args.LeaderId
		rf.voteCount = 0
		rf.currentState = FollowerState
	}

	lastLogIndex := len(rf.log) - 1

	// deny request if the log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	if args.PrevLogIndex > lastLogIndex || args.PrevLogTerm != rf.log[args.PrevLogIndex].Term {
		return
	}

	// handle Entries
	for i := 0; i < len(args.Entries) && rf.killed() == false; i++ {
		logIndex := args.PrevLogIndex + i + 1
		// delete conflict existing entrie(s)
		if logIndex < len(rf.log) && rf.log[logIndex].Term != args.Entries[i].Term {
			rf.log = rf.log[:logIndex]
		}
		// append any new entries not already in the log
		if logIndex > len(rf.log) - 1 {
			rf.log = append(rf.log, args.Entries[i])	
		}
	}

	// update commit index
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, args.PrevLogIndex + len(args.Entries))
	}

	reply.Success = true
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.currentState != LeaderState {
		return index, term, false
	}

	index = len(rf.log)
	term 	= rf.currentTerm

	entry := Entry{Command: command, Term: term}

	// append entry to local log
	rf.log = append(rf.log, entry)
	rf.matchIndex[rf.me] = index

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

	close(rf.appendEntriesReplyCh)
	close(rf.requestVoteReplyCh)
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	for rf.killed() == false {
		// Your code here (3A)
		// Check if a leader election should be started.
		time.Sleep(time.Duration(100) * time.Millisecond)

		rf.mu.Lock()

		electionTimeout := rf.electionTimeoutLowerBound + time.Duration(rand.Int63() % 300) * time.Millisecond
		if rf.currentState == LeaderState || time.Now().Before(rf.lastHeartbeat.Add(electionTimeout)) {
			rf.mu.Unlock()
			continue
		}

		tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("start election for term %d", rf.currentTerm + 1), "")


		// transit to candidate state
		rf.currentState = CandidateState
		// increament current term
		rf.currentTerm += 1
		// vote for itself
		rf.voteIdFor = rf.me
		rf.voteCount = 1 
		// reset election timer
		rf.lastHeartbeat = time.Now()

		tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("sending votes in term %d", rf.currentTerm), "")

		lastLogIndex := len(rf.log) - 1
		lastLogTerm := rf.log[lastLogIndex].Term

		for i := 0; i < len(rf.peers) && rf.killed() == false && rf.currentState == CandidateState; i++ {
			if i == rf.me { continue }
			go func(term int, candId int, peer int){
				args := &RequestVoteArgs{
					Term: term,
					CandidateId: candId,
					LastLogIndex: lastLogIndex,
					LastLogTerm: lastLogTerm,
				}
				reply := new(RequestVoteReply)
				ret := rf.sendRequestVote(peer, args, reply)

				if ret { rf.requestVoteReplyCh <- reply }
			}(rf.currentTerm, rf.me, i)

		}

		rf.mu.Unlock()
	}
}

func (rf *Raft) heartbeat() {
	for rf.killed() == false {
		// pause for a random amount of time between 150 and 250
		// milliseconds.
		ms := 150 + (rand.Int63() % 100)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		rf.mu.Lock()

		// do nothing if it is not leader
		if rf.currentState != LeaderState { 
			rf.mu.Unlock()
			continue 
		}


		// broadcast heartbeat if it is leader
		for i := 0; i < len(rf.peers) && rf.killed() == false; i++ {
			if i == rf.me { continue }

			prevLogTerm := rf.log[rf.nextIndex[i] - 1].Term

			go func(term int, leaderId int, peer int, commitIndex int){
				args := &AppendEntriesArgs{
					Term: term,
					LeaderId: leaderId,
					PrevLogIndex: rf.nextIndex[i] - 1,
					PrevLogTerm: prevLogTerm,
					LeaderCommit: commitIndex,
				}
				reply := new(AppendEntriesReply)
				ret := rf.sendAppendEntries(peer, args, reply)
				if ret { rf.appendEntriesReplyCh <- reply }
			}(rf.currentTerm, rf.me, i, rf.commitIndex)
			
		}

		rf.mu.Unlock()
	}
}

func (rf *Raft) requestVoteReplyHandler() {
	for reply := range rf.requestVoteReplyCh {
		rf.mu.Lock()

		if rf.killed() {
			rf.mu.Unlock()
			break
		}

		tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Get RV Reply in term %d", rf.currentTerm), fmt.Sprintf("reply term: %d, reply voteGranted: %t", reply.Term, reply.VoteGranted))

		if reply.VoteGranted { 
			rf.voteCount += 1 
		// discover new term
		} else if reply.Term > rf.currentTerm {
			// catch up the term
			rf.currentTerm = reply.Term
			// transit back to follower state
			rf.voteIdFor = -1
			rf.voteCount = 0
			rf.currentState = FollowerState
			break
		}
		// check if get majority votes
		if rf.currentState == CandidateState && rf.voteCount > (len(rf.peers) / 2) {
			rf.voteIdFor = -1
			rf.voteCount = 0
			for i := 0; i < len(rf.peers); i++ {
				rf.nextIndex[i] = 1
				if i == rf.me {
					rf.nextIndex[i] = len(rf.log)
				}
				rf.matchIndex[i] = 0
			}
			// transit to leader state	
			rf.currentState = LeaderState
			tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("become leader in term %d", rf.currentTerm), "")
		}

		rf.mu.Unlock()
	}
}

func (rf *Raft) appendEntriesReplyHandler() {
	for reply := range rf.appendEntriesReplyCh {
		rf.mu.Lock()

		if rf.killed() {
			rf.mu.Unlock()
			break
		}

		tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Get AE Reply in term %d", rf.currentTerm), fmt.Sprintf("reply term: %d", reply.Term))

		// transit to follower if discover newer term
		if reply.Term > rf.currentTerm && rf.killed() == false {
			rf.currentTerm = reply.Term
			rf.voteIdFor = -1
			rf.voteCount = 0
			rf.currentState = FollowerState
		}

		if rf.currentState != LeaderState {
			rf.mu.Unlock()
			continue
		}

		if reply.Success {
			// update nextIndex and matchIndex for follower
			rf.nextIndex[reply.PeerId] = reply.PrevLogIndex + reply.EntriesLength + 1
			rf.matchIndex[reply.PeerId] = reply.PrevLogIndex + reply.EntriesLength
		} else {
			// decrement nextIndex
			rf.nextIndex[reply.PeerId] = reply.PrevLogIndex
		}

		// update commit index to N
		// if there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N
		// and log[N].term == currentTerm
		index := slices.Max(rf.matchIndex)
		for ; index > rf.commitIndex && rf.killed() == false; index-- {
			count := 0
			for i := 0; i < len(rf.peers) && rf.killed() == false; i++ {
				if rf.matchIndex[i] >= index {
					count += 1
				}
			}
			if count > len(rf.peers) / 2 && rf.log[index].Term == rf.currentTerm {
				tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Update commit index to %d in term %d with %d # of count", index, rf.currentTerm, count), "")
				rf.commitIndex = index
				break
			}
		}

		rf.mu.Unlock()
	}
}

// FIX: no AE req sent
func (rf *Raft) appendEntriesReqHandler() {
	for rf.killed() == false {
		// pause for a random amount of time between 150 and 250
		// milliseconds.
		ms := 150 + (rand.Int63() % 100)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		rf.mu.Lock()

		if rf.currentState != LeaderState {
			rf.mu.Unlock()
			continue
		}

		lastLogIndex := len(rf.log) - 1

		for i := 0; i < len(rf.peers) && rf.killed() == false; i++ {
		  // If last log index ≥ nextIndex for a follower: send AppendEntries RPC with log entries starting at nextIndex
			if i == rf.me || rf.nextIndex[i] > lastLogIndex { continue }

			tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Send AE Request in term %d", rf.currentTerm), "")

			entries := rf.log[rf.nextIndex[i]:]
			prevLogIndex := rf.nextIndex[i] - 1
			prevLogTerm := rf.log[rf.nextIndex[i] - 1].Term

			go func(term int, leaderId int, peer int, commitIndex int){
				args := &AppendEntriesArgs{
					Term: term,
					LeaderId: leaderId,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm: prevLogTerm,
					LeaderCommit: commitIndex,
					Entries: entries,
				}
				reply := new(AppendEntriesReply)
				ret := rf.sendAppendEntries(peer, args, reply)
				if ret { rf.appendEntriesReplyCh <- reply }
			}(rf.currentTerm, rf.me, i, rf.commitIndex)

		}
		
		rf.mu.Unlock()
	}
}

func (rf *Raft) committedLogHandler() {
	for rf.killed() == false {
		// pause for a random amount of time between 150 and 200
		// milliseconds.
		ms := 150 + (rand.Int63() % 50)
		time.Sleep(time.Duration(ms) * time.Millisecond)

		rf.mu.Lock()


		// Send each newly committed entry on applyCh on each peer
		for i := rf.lastApplied + 1; i <= rf.commitIndex && rf.killed() == false; i++ {
			applyMsg := raftapi.ApplyMsg {
				CommandValid: true,
				Command: rf.log[i].Command,
				CommandIndex: i,
			}
			rf.applyCh <- applyMsg
		}

		rf.mu.Unlock()
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
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.voteIdFor = -1
	rf.electionTimeoutLowerBound = 900 * time.Millisecond
	rf.lastHeartbeat = time.Now()
	rf.currentState = FollowerState
	rf.nextIndex = make([]int, len(peers), len(peers))
	rf.matchIndex = make([]int, len(peers), len(peers))
	rf.log = make([]Entry, 0, len(peers))
	rf.applyCh = applyCh
	
	// initialize nextIndex and matchIndex
	for i := 0; i < len(peers); i++ {
		rf.nextIndex[i] = 1
		rf.matchIndex[i] = 0
	}

	// add dummy entry as the first entry of the log
	rf.log = append(rf.log, Entry{Term: 0})

	rf.appendEntriesReplyCh = make(chan *AppendEntriesReply, 2 * len(rf.peers) * len(rf.peers))
	rf.requestVoteReplyCh = make(chan *RequestVoteReply, 2 * len(rf.peers) * len(rf.peers))

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	// start heartbeat goroutine to start hear beating
	go rf.heartbeat()

	// start reply handlers
	go rf.requestVoteReplyHandler()
	go rf.appendEntriesReplyHandler()

	// start commit handlers
	go rf.committedLogHandler()

	// start appendEntries request handler
	go rf.appendEntriesReqHandler()

	return rf
}
