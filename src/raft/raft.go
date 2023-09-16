package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(Command interface{}) (Index, Term, isleader)
//   start agreement on a new log entry
// rf.GetState() (Term, isLeader)
//   ask a Raft for its current Term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"6.824/labgob"
	"bytes"
	"log"
	rand2 "math/rand"

	//	"bytes"
	"sync"
	"sync/atomic"
	"time"

	//	"6.824/labgob"
	"6.824/labrpc"
)

type termT int
type indexT int
type statusT int

const (
	follower statusT = iota
	leader
	candidate
)

const (
	minElectionTimeOut    int = 300
	lengthElectionTimeOut int = 600
)

type Log struct {
	Term    termT
	Index   indexT
	Command interface{}
}

// ApplyMsg as each Raft peer becomes aware that successive log Entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// Raft peer: A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's Index into peers[]
	dead      int32               // set by Kill()
	peerNum   int                 // 集群中机器数量
	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	heartBeat    time.Duration
	status       statusT
	electionTime time.Time
	currentTerm  termT
	logs         []Log
	votedFor     int

	commitIndex indexT
	nextIndex   []indexT
	matchIndex  []indexT

	logger *log.Logger
}

// GetState return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	rf.mu.Lock()
	term = int(rf.currentTerm)
	isleader = rf.status == leader
	rf.mu.Unlock()
	// Your code here (2A).
	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	err := e.Encode(rf.currentTerm)
	if err != nil {
		return
	}
	err = e.Encode(rf.votedFor)
	if err != nil {
		return
	}
	err = e.Encode(rf.logs)
	if err != nil {
		return
	}
	rf.persister.SaveRaftState(w.Bytes())
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
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

// CondInstallSnapshot A service wants to switch to snapshot.  Only do so if Raft hasn't
// had more recent info since it communicate the snapshot on applyCh.
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).

	return true
}

// Snapshot the service says it has created a snapshot that has
// all info up to and including Index. this means the
// service no longer needs the log through (and including)
// that Index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

// RequestVoteArgs example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	CandidateTerm termT
	CandidateId   int
	LastLogEntry  indexT
	LastLogTerm   termT
}

// RequestVoteReply example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        termT
	VoteGranted bool
}

// RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {

	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.CandidateTerm > rf.currentTerm {
		rf.status = follower
		rf.setTerm(args.CandidateTerm)
	}
	reply.Term = rf.currentTerm
	if rf.currentTerm > args.CandidateTerm {
		reply.VoteGranted = false
		return
	}
	lastLogTerm := rf.getLastLogTerm()
	lastLogIndex := rf.getLastLogIndex()
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) &&
		(lastLogTerm < args.LastLogTerm || (lastLogTerm == args.LastLogTerm && lastLogIndex <= args.LastLogEntry)) {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.logger.Printf("[%d] Vote To [%d] In term [%d]\n", rf.me, rf.votedFor, rf.currentTerm)
		rf.persist()
		rf.setElectionTime()
	} else {
		reply.VoteGranted = false
	}
}

// example code to send a RequestVote RPC to a server.
// server is the Index of the target server in rf.peers[].
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
// handler function on the server side does not return.  Thus, there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply, convert *sync.Once, countVote *int) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	if !ok {
		return ok
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if reply.Term > rf.currentTerm {
		rf.setTerm(reply.Term)
		// rf.setElectionTime()
		rf.status = follower
		return false
	}
	if reply.Term < rf.currentTerm {
		// 过期得rpc
		return false
	}
	if !reply.VoteGranted {
		return false
	}
	*countVote++
	if *countVote > rf.peerNum/2 &&
		rf.status == candidate &&
		rf.currentTerm == args.CandidateTerm {
		// 投票成功，转为leader
		convert.Do(func() {
			rf.logger.Printf("[%d] become leader in term [%d]", rf.me, rf.currentTerm)
			/**
			nextIndex for each server, Index of the next log entry to send to that server (initialized to leader last log Index + 1)
			*/
			nextIndex := rf.getLastLogIndex() + 1
			rf.status = leader
			for i := 0; i < rf.votedFor; i++ {
				rf.nextIndex[i] = nextIndex
				rf.matchIndex[i] = 0
				rf.matchIndex[i] = 0
			}
			rf.appendEntries(true)
		})
	}
	return ok
}

// Start the service using Raft (e.g. a k/v server) wants to start
// agreement on the next Command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// Command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the Index that the Command will appear at
// if it's ever committed. the second return value is the current
// Term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

// Kill the tester doesn't halt goroutines created by Raft after each test,
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

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	rf.setElectionTime()
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		time.Sleep(rf.heartBeat)
		rf.mu.Lock()
		if rf.status == leader {
			// 如果是leader状态,发送空包
			rf.appendEntries(true)
			rf.setElectionTime()
			rf.mu.Unlock()
			continue
		}
		if time.Now().After(rf.electionTime) {
			// 如果已经超时， 开始选举
			go rf.startElection()
		}
		rf.mu.Unlock()
	}
}

// Make the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.logger = log.New(log.Writer(), "", log.LstdFlags|log.Lmicroseconds)
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// rf.peers[me].Call()
	// Your initialization code here (2A, 2B, 2C).
	rf.votedFor = -1 // 2a
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	rf.heartBeat = 100 * time.Millisecond
	rf.peerNum = len(peers)
	rf.status = follower

	rf.nextIndex = make([]indexT, rf.peerNum)
	rf.matchIndex = make([]indexT, rf.peerNum)
	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}

func (rf *Raft) setElectionTime() {
	timeOut := minElectionTimeOut + rand2.Intn(lengthElectionTimeOut) // 生成150~300随机数
	rf.electionTime = time.Now().Add(time.Duration(timeOut) * time.Millisecond)
}

func (rf *Raft) startElection() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.setElectionTime()
	rf.status = candidate
	rf.currentTerm++

	rf.logger.Printf("[%d] Time Out,Start Election Term [%d]", rf.me, rf.currentTerm)
	rf.votedFor = rf.me
	rf.persist()
	var convert sync.Once
	var countVote int = 1
	for i := range rf.peers {
		if i != rf.me {
			var reply RequestVoteReply
			request := RequestVoteArgs{
				CandidateTerm: rf.currentTerm,
				CandidateId:   rf.me,
				LastLogEntry:  rf.getLastLogIndex(),
				LastLogTerm:   rf.getLastLogTerm(),
			}
			go rf.sendRequestVote(i, &request, &reply, &convert, &countVote)
		}
	}
}

// getLastLogIndex returns the last Index of logs
func (rf *Raft) getLastLogIndex() indexT {
	if len(rf.logs) == 0 {
		return 0
	} else {
		return rf.logs[len(rf.logs)-1].Index
	}
}

// getLastLogIndex returns the last Term of logs
func (rf *Raft) getLastLogTerm() termT {
	if len(rf.logs) == 0 {
		return 0
	} else {
		return rf.logs[len(rf.logs)-1].Term
	}
}
func (rf *Raft) setTerm(t termT) {
	rf.currentTerm = t
	rf.votedFor = -1
	rf.persist()
}
func (rf *Raft) appendEntries(heartBeat bool) {

	for i := range rf.peers {
		if i != rf.me {
			args := appendEntryArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: rf.getLastLogIndex(),
				PrevLogTerm:  rf.getLastLogTerm(),
				Entries:      nil,
				LeaderCommit: rf.commitIndex,
			}
			if heartBeat {
				go func(rf *Raft, args *appendEntryArgs, peerId int) {
					client := rf.peers[peerId]
					var reply appendEntryReply
					client.Call("Raft.AppendEntriesRPC", args, &reply)
					rf.mu.Lock()
					defer rf.mu.Unlock()
					if reply.Term > rf.currentTerm {
						rf.setTerm(reply.Term)
						rf.setElectionTime()
						rf.status = follower
						return
					}
				}(rf, &args, i)
			}
		}
	}
}

type appendEntryArgs struct {
	Term         termT  //leader’s Term
	LeaderId     int    // so follower can redirect clients
	PrevLogIndex indexT //Index of log entry immediately preceding new ones
	PrevLogTerm  termT  // Term of PrevLogIndex entry
	Entries      []Log  // log Entries to store (empty for heartbeat; may send more than one for efficiency)
	LeaderCommit indexT // leader’s commitIndex
}

type appendEntryReply struct {
	Term    termT // currentTerm, for leader to update itself
	Success bool  // true if follower contained entry matching PrevLogIndex and PrevLogTerm
}

func (rf *Raft) AppendEntriesRPC(args *appendEntryArgs, reply *appendEntryReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Success = false
	reply.Term = rf.currentTerm
	if args.Term > rf.currentTerm {
		rf.status = follower
		rf.setElectionTime() // check(AntiO2)
		rf.setTerm(args.Term)
		return
	}
	if args.Term < rf.currentTerm {
		return
	}
	rf.setElectionTime()
	if rf.status == candidate {
		rf.status = follower
	}
	if args.Entries == nil || len(args.Entries) == 0 {
		// do heartBeat
		reply.Success = true
		return
	}
}
