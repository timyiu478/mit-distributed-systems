package raft

// The file raftapi/raft.go defines the interface that raft must
// expose to servers (or the tester), but see comments below for each
// of these functions for more details.
//
// Make() creates a new raft peer that implements the raft interface.

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"slices"

	"6.5840/labgob"
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
	CurrentTerm 	int 
	VoteIdFor   	int
	voteCount     int
	commitIndex   int
	lastApplied   int

	LastIncludedIndex					int 
	LastIncludedTerm					int

	snapshot      []byte

	nextIndex 		[]int // for each server, index of the next log entry to send to that server
	matchIndex 		[]int // for each server, index of highest log entry known to be replicated on server

	Log 					[]Entry // 0-indexed

	lastHeartbeat		 						time.Time
	electionTimeoutLowerBound  	time.Duration

	currentState  State

	applyCh 							chan raftapi.ApplyMsg
	startCh 							chan struct{}
	commitCh 							chan struct{}
	committedCh 					chan struct{}

	wg					  sync.WaitGroup
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (3A).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	term = rf.CurrentTerm
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

	DPrintf(fmt.Sprintf("Server %d: start to persist in term %d", rf.me, rf.CurrentTerm))


	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)

	e.Encode(rf.VoteIdFor)
	e.Encode(rf.CurrentTerm)
	e.Encode(rf.LastIncludedIndex)
	e.Encode(rf.LastIncludedTerm)
	e.Encode(rf.Log)
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, rf.snapshot)
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
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var voteIdFor int
	var currentTerm int
	var lastIncludedIndex int
	var lastIncludedTerm int
	var log []Entry
	if d.Decode(&voteIdFor) != nil || d.Decode(&currentTerm) != nil || d.Decode(&lastIncludedIndex) != nil || d.Decode(&lastIncludedTerm) != nil || d.Decode(&log) != nil {
		panic("Failed to decode previously persisted state")
	} else {
		rf.VoteIdFor = voteIdFor
		rf.CurrentTerm = currentTerm
		rf.LastIncludedIndex = lastIncludedIndex
		rf.LastIncludedTerm  = lastIncludedTerm
		rf.Log = log
	}
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

	rf.mu.Lock()
	defer rf.mu.Unlock()
	
	indexInLog := index - rf.LastIncludedIndex - 1

	if indexInLog < 0 || indexInLog >= len(rf.Log) {
		DPrintf(fmt.Sprintf("Server %d: invalid Snapshot index %d", rf.me, index))
		return
	}

	rf.LastIncludedTerm  = rf.Log[indexInLog].Term
	trimedLog := rf.Log[indexInLog + 1:]
	rf.LastIncludedIndex = index
	rf.snapshot = snapshot
	
	rf.Log = make([]Entry, len(trimedLog))
	copy(rf.Log, trimedLog)

	rf.persist()

	if rf.lastApplied < rf.LastIncludedIndex {
		rf.lastApplied = rf.LastIncludedIndex
	}

	DPrintf(fmt.Sprintf("Server %d: snapshot index %d, trimmed log length is %d, lastIncludedIndex is %d", rf.me, index, len(rf.Log), rf.LastIncludedIndex))
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
	XTerm     		int
	XIndex				int
	XLen     			int
	Success				bool
}

// InstallSnapshot RPC request structure
// No offset mechanism
type InstallSnapshotArgs struct {
	Term 								int
	LeaderId						int
	LastIncludedIndex		int
	LastIncludedTerm    int
	Data								[]byte
}

// InstallSnapshot RPC reply structure
type InstallSnapshotReply struct {
	Term 								int
}



// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (3A, 3B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("recieved RV RPC in term %d", rf.CurrentTerm), "")

	// default reply
	reply.Term = rf.CurrentTerm
	reply.VoteGranted = false

	// deny request from older term
	if rf.CurrentTerm > args.Term { 
		DPrintf(fmt.Sprintf("Server %d: deny request vote for %d in term %d because the request is from older term %d", rf.me, args.CandidateId, rf.CurrentTerm, args.Term))
		return
	}

	// adopt the newer term before handle the RPC
	if rf.CurrentTerm < args.Term {
		rf.CurrentTerm = args.Term
		rf.VoteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		reply.Term = args.Term
	}

	// If votedFor is null or candidateId, and candidate’s log is at
	// least as up-to-date as receiver’s log, grant vote
	// See the section 5.4.1 of the paper for the definition of "up-to-date"
	lastLogIndex := len(rf.Log) - 1
	realLastLogIndex := lastLogIndex + rf.LastIncludedIndex + 1
	lastLogTerm  := rf.LastIncludedTerm
	if lastLogIndex >= 0 {
		lastLogTerm  = rf.Log[lastLogIndex].Term
	}
	upToDate := (args.LastLogTerm > lastLogTerm) || (lastLogTerm == args.LastLogTerm && realLastLogIndex <= args.LastLogIndex)
	if (rf.VoteIdFor == -1 || rf.VoteIdFor == args.CandidateId) && upToDate {
		// vote for first valid candidate 
		rf.VoteIdFor = args.CandidateId
		// reset election timer
		rf.lastHeartbeat = time.Now()

		reply.VoteGranted = true

		// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("vote for %d in term %d", rf.VoteIdFor, rf.CurrentTerm), "")
		DPrintf(fmt.Sprintf("Server %d: vote for %d in term %d", rf.me, args.CandidateId, rf.CurrentTerm))
	} else {
		DPrintf(fmt.Sprintf("Server %d: deny request vote for %d in term %d", rf.me, args.CandidateId, rf.CurrentTerm))
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

// send a InstallSnapshot RPC to a server.
func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

// AppendEntries RPC handler
// invoked by a librpc call
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("recieved AE RPC in term %d", rf.CurrentTerm), "")

	// default reply
	reply.Term = rf.CurrentTerm
	reply.Success = false
	reply.PeerId = rf.me
	reply.PrevLogIndex = args.PrevLogIndex
	reply.EntriesLength = len(args.Entries)
	reply.XTerm = -1
	reply.XIndex = -1
	reply.XLen = -1

	// deny request from older term
	if rf.CurrentTerm > args.Term {
		DPrintf(fmt.Sprintf("Server %d: deny AE req because args.Term(%d) < currentTerm(%d)", rf.me, args.Term, rf.CurrentTerm))
		return
	}

	// update heartbeat
	rf.lastHeartbeat = time.Now()

	// transit to follower state if discover higher term
	if rf.CurrentTerm < args.Term {
		rf.CurrentTerm = args.Term
		rf.VoteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		reply.Term = args.Term

		rf.persist()
	} else if rf.currentState == CandidateState && rf.CurrentTerm == args.Term { // transit to follower state if discover leader in this term
		// DO NOT reset the voteIdFor
		// if a server clears voteIdFor while staying in the same term,
		// it can later grant a second vote in the same term to another candidate (where its id != args.LeaderId)
		rf.VoteIdFor = args.LeaderId
		rf.voteCount = 0
		rf.currentState = FollowerState

		rf.persist()
	}

	lastLogIndex := len(rf.Log) - 1
	realLastLogIndex := lastLogIndex + rf.LastIncludedIndex + 1

	// deny request if the log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm
	if args.PrevLogIndex > realLastLogIndex {
		reply.XLen = len(rf.Log) + rf.LastIncludedIndex + 1

		DPrintf(fmt.Sprintf("Server %d: deny AE req because args.PrevLogIndex > realLastLogIndex and set XLen to %d", rf.me, reply.XLen))

		return
	}
	// deny request if args.PrevLogIndex < rf.LastIncludedIndex
	// TODO
	if args.PrevLogIndex < rf.LastIncludedIndex {
		return
	}

	// prevLogIndex refers to the index in trimmed log
	prevLogIndex := args.PrevLogIndex - rf.LastIncludedIndex - 1
	prevLogTerm := rf.LastIncludedTerm
	if prevLogIndex >= 0 {
		prevLogTerm = rf.Log[prevLogIndex].Term
	}

	if args.PrevLogTerm != prevLogTerm {
		reply.XTerm = prevLogTerm 

		// search for the first index that its entry term == reply.XTerm
		// by searching the last index that its entry term != reply.XTerm 
		i := prevLogIndex - 1
		for ; i >= 0 && rf.killed() == false; i-- {
			if rf.Log[i].Term != rf.Log[prevLogIndex].Term {
				break
			}
		}
		reply.XIndex = rf.LastIncludedIndex + 1 + i

		DPrintf(fmt.Sprintf("Server %d: deny AE req because terms are different and set XTerm to %d and XIndex to %d", rf.me, reply.XTerm, reply.XIndex))

		return
	}

	// handle Entries
	i := 0

	for ; i < len(args.Entries) && rf.killed() == false; i++ {
		logIndex := args.PrevLogIndex - rf.LastIncludedIndex + i
		if logIndex > len(rf.Log) - 1 {
			break
		}
		// delete conflict existing entrie(s)
		if logIndex < len(rf.Log) && rf.Log[logIndex].Term != args.Entries[i].Term {
			rf.Log = rf.Log[:logIndex]
			break
		}
	}
	// append any new entries not already in the log
	logIndex := args.PrevLogIndex - rf.LastIncludedIndex + i + 1
	if logIndex > len(rf.Log) - 1 {
		rf.Log = append(rf.Log, args.Entries[i:]...)
	}
	// persist once after all modifications
	rf.persist()

	// update commit index
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, args.PrevLogIndex + len(args.Entries))
		DPrintf(fmt.Sprintf("Server %d: update commit index to %d, length of log is %d", rf.me, rf.commitIndex, len(rf.Log)))
		// signal commit log handler
		if len(rf.commitCh) < cap(rf.commitCh) {
			rf.commitCh <- struct{}{}
		}
	} else {
		DPrintf(fmt.Sprintf("Server %d received AE request from %d in term %d, entries length is %d", rf.me, args.LeaderId, rf.CurrentTerm, len(args.Entries)))
	}

	reply.Success = true
}

// InstallSnapshot RPC
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// default reply
	reply.Term = rf.CurrentTerm

	// deny request if term < currentTerm
	if args.Term < rf.CurrentTerm {
		DPrintf(fmt.Sprintf("Server %d: deny InstallSnapshot req because req.Term(%d) < rf.CurrentTerm(%d)", rf.me, args.Term, rf.CurrentTerm))
		return
	}

	// discover new term
	if args.Term > rf.CurrentTerm {
		// catch up the term
		rf.CurrentTerm = args.Term
		// transit back to follower state
		rf.VoteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		reply.Term = rf.CurrentTerm
		
		rf.persist()
	}

	// ignore an InstallSnapshot if the recipient is already ahead of that snapshot
	if args.LastIncludedIndex <= rf.LastIncludedIndex {
		return
	}

	indexInLog := args.LastIncludedIndex - rf.LastIncludedIndex

	if indexInLog < len(rf.Log) {
		trimedLog := rf.Log[indexInLog:]
		rf.Log = make([]Entry, len(trimedLog))
		copy(rf.Log, trimedLog)
		
	} else {
		rf.Log = make([]Entry, 0)
	}

	DPrintf(fmt.Sprintf("Server %d: install snapshot, trimmed log length is %d, lastIncludedIndex is %d", rf.me, len(rf.Log), rf.LastIncludedIndex))

	rf.LastIncludedIndex = args.LastIncludedIndex
	rf.LastIncludedTerm = args.LastIncludedTerm
	rf.snapshot = args.Data

	// save new log and snapshot
	rf.persist()

	if rf.lastApplied < rf.LastIncludedIndex {
		rf.lastApplied = rf.LastIncludedIndex

		applyMsg := raftapi.ApplyMsg {
			SnapshotValid: true,
			CommandValid: false,
			Snapshot: rf.snapshot,
			SnapshotTerm: rf.LastIncludedTerm,
			SnapshotIndex: rf.LastIncludedIndex,
		}
		rf.applyCh <- applyMsg
	}
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

	term 	= rf.CurrentTerm

	entry := Entry{Command: command, Term: term}

	// append entry to local log
	rf.Log = append(rf.Log, entry)
	index = len(rf.Log) + rf.LastIncludedIndex
	rf.matchIndex[rf.me] = index

	DPrintf(fmt.Sprintf("Server %d received command in term %d, matchIndex is %d", rf.me, rf.CurrentTerm, index))

	rf.persist()

	// trigger appendEntriesReqHandler to send AE Req immediately
	// by signaling the handler that it has >= 1 commands to be replicated
	if len(rf.startCh) < cap(rf.startCh) {
		rf.startCh <- struct{}{}
	}

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

	// Assume Kill() will be called exactly once only
	// Unblock the killing process
	go func(applyCh chan raftapi.ApplyMsg, committedCh chan struct{}) {
		for {
			_, ok1 := <- applyCh
			_, ok2 := <- committedCh
			if !ok1 && !ok2 { break }
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
	}(rf.applyCh, rf.committedCh)

	DPrintf(fmt.Sprintf("Server %d is killed", rf.me))
}

func (rf *Raft) closeChannel() {
	rf.wg.Wait()

	DPrintf(fmt.Sprintf("Server %d starts to close channels in term %d", rf.me, rf.CurrentTerm))

	close(rf.startCh)
	close(rf.applyCh)
	close(rf.commitCh)
	close(rf.committedCh)
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) ticker() {
	defer rf.wg.Done()

	for rf.killed() == false {
		// Your code here (3A)
		time.Sleep(time.Duration(10) * time.Millisecond)

		rf.mu.Lock()

		// Check if a leader election should be started.
		electionTimeout := rf.electionTimeoutLowerBound + time.Duration(rand.Int63() % 200) * time.Millisecond
		if rf.currentState == LeaderState || time.Now().Before(rf.lastHeartbeat.Add(electionTimeout)) {
			rf.mu.Unlock()
			continue
		}

		// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("start election for term %d", rf.CurrentTerm + 1), "")


		// transit to candidate state
		rf.currentState = CandidateState
		// increament current term
		rf.CurrentTerm += 1
		// vote for itself
		rf.VoteIdFor = rf.me
		rf.voteCount = 1 
		// reset election timer
		rf.lastHeartbeat = time.Now()
		// persist
		rf.persist()

		DPrintf(fmt.Sprintf("Server %d start election in term %d", rf.me, rf.CurrentTerm))

		// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("sending votes in term %d", rf.CurrentTerm), "")

		lastLogIndex := rf.LastIncludedIndex
		lastLogTerm := rf.LastIncludedTerm

		if len(rf.Log) > 0 {
			lastLogIndex += len(rf.Log) 
			lastLogTerm = rf.Log[len(rf.Log)-1].Term
		}

		for i := 0; i < len(rf.peers) && rf.killed() == false && rf.currentState == CandidateState; i++ {
			if i == rf.me { continue }
			go func(term int, candId int, peer int){
				args := &RequestVoteArgs{
					Term: term,
					CandidateId: candId,
					LastLogIndex: lastLogIndex,
					LastLogTerm: lastLogTerm,
				}
				reply := &RequestVoteReply{}
				ret := rf.sendRequestVote(peer, args, reply)

				if ret && rf.killed() == false { 
					go rf.requestVoteReplyHandler(*reply)
				}
			}(rf.CurrentTerm, rf.me, i)
		}

		rf.mu.Unlock()

	}
}

func (rf *Raft) requestVoteReplyHandler(reply RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.killed() {
		return
	}

	// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Get RV Reply in term %d", rf.CurrentTerm), fmt.Sprintf("reply term: %d, reply voteGranted: %t", reply.Term, reply.VoteGranted))

	// deny reply from older term
	if rf.CurrentTerm > reply.Term {
		return
	}

	if reply.VoteGranted { 
		rf.voteCount += 1 
	} 

	// discover new term
	if reply.Term > rf.CurrentTerm {
		// catch up the term
		rf.CurrentTerm = reply.Term
		// transit back to follower state
		rf.VoteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState
		
		rf.persist()
	}

	// check if get majority votes
	if rf.currentState == CandidateState && rf.voteCount > (len(rf.peers) / 2) {
		rf.VoteIdFor = rf.me // Don't reset VoteIdFor. Otherwise, the leader could vote for the candidate
		rf.voteCount = 0

		// reinitialize nextIndex & matchIndex after election
		for i := 0; i < len(rf.peers); i++ {
			rf.nextIndex[i] = len(rf.Log) + rf.LastIncludedIndex + 1
			rf.matchIndex[i] = 0
		}
		rf.matchIndex[rf.me] = len(rf.Log) + rf.LastIncludedIndex

		// transit to leader state	
		rf.currentState = LeaderState
		// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("become leader in term %d", rf.CurrentTerm), "")
		DPrintf(fmt.Sprintf("Server %d become leader in term %d, matchIndex is %d, len of log is %d, lastIncludedIndex is %d", rf.me, rf.CurrentTerm, rf.matchIndex[rf.me], len(rf.Log), rf.LastIncludedIndex))

		rf.persist()
	}
}

func (rf *Raft) appendEntriesReplyHandler(reply AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	DPrintf(fmt.Sprintf("Server %d received AE RPC Reply: { XTerm: %d, XIndex: %d, XLen: %d, PeerID: %d, Success: %t, Term: %d} in term %d", rf.me, reply.XTerm, reply.XIndex, reply.XLen, reply.PeerId, reply.Success, reply.Term, rf.CurrentTerm))

	// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Get AE Reply in term %d", rf.CurrentTerm), fmt.Sprintf("reply term: %d", reply.Term))

	// deny reply from older term
	if rf.CurrentTerm > reply.Term {
		DPrintf(fmt.Sprintf("Server %d: deny AE reply from %d because reply.Term %d < current term %d", rf.me, reply.PeerId, reply.Term, rf.CurrentTerm))
		return
	}

	// transit to follower if discover newer term
	if reply.Term > rf.CurrentTerm {
		rf.CurrentTerm = reply.Term
		rf.VoteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		rf.persist()
	}

	if rf.currentState != LeaderState {
		return
	}

	if reply.Success {
		// update nextIndex and matchIndex for follower
		rf.nextIndex[reply.PeerId] = reply.PrevLogIndex + reply.EntriesLength + 1
		rf.matchIndex[reply.PeerId] = reply.PrevLogIndex + reply.EntriesLength
		DPrintf(fmt.Sprintf("Server %d: update peer %d's match index to %d and next index to %d", rf.me, reply.PeerId, rf.matchIndex[reply.PeerId], rf.nextIndex[reply.PeerId]))
	} else {
		// TODO: findout why this can happen
		// when the reply.Term >= rf.currentTerm
		if reply.XTerm == -1 && reply.XIndex == -1 && reply.XLen == -1 {
			// normal log backtracking: decrement nextIndex by 1
			rf.nextIndex[reply.PeerId] = max(1, reply.PrevLogIndex)
		}  else if reply.XTerm == -1 { // log backtracking optimization
			// follower's log is too short
			rf.nextIndex[reply.PeerId] = reply.XLen
		} else {
			// leader does not have XTerm
			rf.nextIndex[reply.PeerId] = reply.XIndex

			for i := reply.PrevLogIndex - 1; i >= rf.LastIncludedIndex - 1 && rf.killed() == false; i-- {
				index := i - rf.LastIncludedIndex - 1
				term := rf.LastIncludedTerm
				if index >= 0 {
					term = rf.Log[index].Term
				}
				if term == reply.XTerm {
					// leader has XTerm -> nextIndex = (index of leader's last entry for XTerm) + 1
					rf.nextIndex[reply.PeerId] = i + 1
					break
				}
			}
		}
	}

	// update commit index to N
	// if there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N
	// and log[N].term == currentTerm
	// we find N in optimistic approach (believe N is close to max(rf.matchIndex))
	index := slices.Max(rf.matchIndex)
	for ; index >= rf.LastIncludedIndex && index > rf.commitIndex && rf.killed() == false; index-- {
		count := 0
		for i := 0; i < len(rf.peers) && rf.killed() == false; i++ {
			if rf.matchIndex[i] >= index {
				count += 1
			}
		}
		DPrintf(fmt.Sprintf("Server %d: check %d, %d", rf.me, index, rf.LastIncludedIndex))
		logTerm := rf.LastIncludedTerm
		if index > rf.LastIncludedIndex {
			logTerm = rf.Log[index - rf.LastIncludedIndex - 1].Term
		}
		if index > rf.LastIncludedIndex && count > len(rf.peers) / 2 && logTerm == rf.CurrentTerm {
			// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Update commit index to %d in term %d with %d # of count", index, rf.CurrentTerm, count), "")

			DPrintf(fmt.Sprintf("Server %d: update commit index to %d", rf.me, index))

			rf.commitIndex = index
			// signal commit log handler
			if len(rf.commitCh) < cap(rf.commitCh) {
				rf.commitCh <- struct{}{}
			}

			break
		}
	}
}

func (rf *Raft) appendEntriesReqHandler() {
	defer rf.wg.Done()

	for rf.killed() == false {
		_, ok := <- rf.startCh
		if !ok { return }

		rf.mu.Lock()

		if rf.currentState != LeaderState {
			rf.mu.Unlock()
			continue
		}

		lastLogIndex := len(rf.Log) + rf.LastIncludedIndex

		for i := 0; i < len(rf.peers) && rf.killed() == false; i++ {
			if i == rf.me { continue }

			// tester.Annotate(fmt.Sprintf("Server %d", rf.me), fmt.Sprintf("Send AE Request in term %d", rf.CurrentTerm), "")

			// send Instnall RPC if rf.nextIndex[i] <= rf.LastIncludedIndex
			if rf.nextIndex[i] <= rf.LastIncludedIndex {
				
				DPrintf(fmt.Sprintf("Server %d send Install RPC to peer %d in term %d", rf.me, i, rf.CurrentTerm))

				go func(peer int, term int, leaderId int, lastIncludedIndex int, lastIncludedTerm int, data []byte) {
					args := &InstallSnapshotArgs{
						Term: term,
						LeaderId: leaderId,
						LastIncludedIndex:		lastIncludedIndex,
						LastIncludedTerm:    lastIncludedTerm,
						Data: 	data,
					}
					reply := &InstallSnapshotReply{}
					ret := rf.sendInstallSnapshot(peer, args, reply)
					if ret && rf.killed() == false { 
						go rf.installSnapshotReplyHandler(*reply)
					}

				}(i, rf.CurrentTerm, rf.me, rf.LastIncludedIndex, rf.LastIncludedTerm, rf.snapshot)

				continue
			}

			// Heartbeat with empty entries
			entries := make([]Entry, 0)

		  // else if last log index ≥ nextIndex for a follower: send AppendEntries RPC with log entries starting at nextIndex
			if rf.nextIndex[i] <= lastLogIndex {
				subLog := rf.Log[rf.nextIndex[i] - rf.LastIncludedIndex - 1:]
				entries = make([]Entry, len(subLog))
				copy(entries, subLog)
			}

			prevLogIndex := rf.nextIndex[i] - 1
			prevLogTerm := rf.LastIncludedTerm
			if prevLogIndex > rf.LastIncludedIndex {
				prevLogTerm = rf.Log[prevLogIndex - rf.LastIncludedIndex - 1].Term
			}

			go func(term int, leaderId int, prevLogIndex int, prevLogTerm int, commitIndex int, entries []Entry, peer int){
				args := &AppendEntriesArgs{
					Term: term,
					LeaderId: leaderId,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm: prevLogTerm,
					LeaderCommit: commitIndex,
					Entries: entries,
				}
				reply := &AppendEntriesReply{}
				ret := rf.sendAppendEntries(peer, args, reply)

				DPrintf(fmt.Sprintf("Server %d send AE RPC to peer %d in term %d", rf.me, peer, term))

				if ret && rf.killed() == false { 
					go rf.appendEntriesReplyHandler(*reply)
				}
			}(rf.CurrentTerm, rf.me, prevLogIndex, prevLogTerm, rf.commitIndex, entries, i)

		}

		rf.mu.Unlock()
	}
}

func (rf *Raft) installSnapshotReplyHandler(reply InstallSnapshotReply) {
	rf.mu.Lock()
	defer	rf.mu.Unlock()

	// transit to follower if discover newer term
	if reply.Term > rf.CurrentTerm {
		rf.CurrentTerm = reply.Term
		rf.VoteIdFor = -1
		rf.voteCount = 0
		rf.currentState = FollowerState

		rf.persist()
	}
}

func (rf *Raft) committedLogHandler() {
	defer rf.wg.Done()

	for rf.killed() == false {
		// wait for all "in-flight" applyMsgs are handed 
		// to applyCh's receiver before prepare the 
		// next batch of applyMsgs
		_, ok1 := <- rf.committedCh
		_, ok2 := <- rf.commitCh
		if !ok1 || !ok2 { return }

		rf.mu.Lock()

		if len(rf.Log) + rf.LastIncludedIndex < rf.commitIndex {
			rf.mu.Unlock()
			continue
		}

		if rf.lastApplied < rf.LastIncludedIndex {
			rf.lastApplied = rf.LastIncludedIndex
		}

		msgs := make([]raftapi.ApplyMsg, 0)

		for i := rf.lastApplied + 1; i <= rf.commitIndex && rf.killed() == false; i++ {
			DPrintf(fmt.Sprintf("Server %d makes applyMsg for command index %d in term %d, lastIncludedIndex is %d", rf.me, i, rf.CurrentTerm, rf.LastIncludedIndex))
			applyMsg := raftapi.ApplyMsg {
				CommandValid: true,
				Command: rf.Log[i - rf.LastIncludedIndex - 1].Command,
				CommandIndex: i,
			}
			msgs = append(msgs, applyMsg)
		}

		rf.lastApplied = rf.commitIndex

		// Send each newly committed entry on applyCh on each peer
		rf.wg.Add(1)
		go func(msgs []raftapi.ApplyMsg, applyCh chan raftapi.ApplyMsg, committedCh chan struct{}){
			defer rf.wg.Done()
			for _, msg := range msgs {
				applyCh <- msg
			}
			committedCh <- struct{}{}
		}(msgs, rf.applyCh, rf.committedCh)

		rf.mu.Unlock()

	}
}

func (rf *Raft) heartbeat() {
	defer rf.wg.Done()

	for !rf.killed() {
		if len(rf.startCh) < cap(rf.startCh) {
			rf.startCh <- struct{}{}
		}
		time.Sleep(time.Duration(100) * time.Millisecond)
	}
}

func (rf *Raft) signal() {
	defer rf.wg.Done()

	rf.committedCh <- struct{}{}

	for !rf.killed() {
		if len(rf.commitCh) < cap(rf.commitCh) {
			rf.commitCh <- struct{}{}
		}
		time.Sleep(time.Duration(100) * time.Millisecond)
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
	rf.CurrentTerm = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.VoteIdFor = -1
	rf.electionTimeoutLowerBound = 700 * time.Millisecond
	rf.lastHeartbeat = time.Now()
	rf.currentState = FollowerState
	rf.nextIndex = make([]int, len(peers), len(peers))
	rf.matchIndex = make([]int, len(peers), len(peers))
	rf.Log = make([]Entry, 0, len(peers))
	rf.applyCh = applyCh
	rf.startCh = make(chan struct{}, 1)
	rf.commitCh = make(chan struct{}, 1)
	rf.committedCh = make(chan struct{}, 1)
	
	rf.LastIncludedIndex       	= 0
	rf.LastIncludedTerm       	= 0


	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	rf.snapshot = persister.ReadSnapshot()

	// start ticker goroutine to start elections
	rf.wg.Add(1)
	go rf.ticker()

	// start commit handlers
	rf.wg.Add(1)
	go rf.committedLogHandler()

	// start appendEntries request handler
	rf.wg.Add(1)
	go rf.appendEntriesReqHandler()

	// signal AE req handler per 100 Millisecond
	rf.wg.Add(1)
	go rf.heartbeat()

	// signal commit log handler per 20 Millisecond
	rf.wg.Add(1)
	go rf.signal()

	// close channels when all goroutines are finished
	go rf.closeChannel()

	return rf
}
