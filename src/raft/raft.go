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
	"io"
	"log"
	rand2 "math/rand"
	"strconv"

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
	debug bool = false
)
const (
	follower statusT = iota
	leader
	candidate
)

const (
	// config
	minElectionTimeOut    int           = 300
	lengthElectionTimeOut int           = 600
	heartBeatTime         time.Duration = 75 * time.Millisecond
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
	logLatch  sync.RWMutex        // Lock to protect log rw
	peers     []*labrpc.ClientEnd // (RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's Index into peers[])
	dead      int32               // set by Kill()
	peerNum   int                 // 集群中机器数量
	// Your Data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	heartBeat    time.Duration
	status       statusT
	electionTime time.Time
	currentTerm  termT
	logs         []Log
	votedFor     int

	commitIndex indexT
	applyIndex  indexT

	nextIndex  []indexT
	matchIndex []indexT

	logger    *log.Logger
	applySign chan bool
	applyCh   chan ApplyMsg
	// [discard] logOffset int //  当logs为[0,3,4,5],logOffset为-2（原来下标为3的log移到了1的位置。），比如想找Index=5的log，需要
	// 计算Index+logOffset = 3,得到Index=5的log在下标3的位置
	// 该项记录在logs[0]中。

	snapshot []byte
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
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm termT
	var votedFor int
	var logs []Log
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&logs) != nil {
		log.Fatalln("Error Occur When Deserialize Raft State")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.logs = logs
	}
}
func (rf *Raft) readSnapshot(data []byte) {
	rf.snapshot = data
}

// CondInstallSnapshot A service wants to switch to snapshot.  Only do so if Raft hasn't
// had more recent info since it communicate the snapshot on applyCh.
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).
	rf.logger.Printf("[%d]  CondInstallSnapshot LastInclude Index[%d]\n", rf.me, lastIncludedIndex)
	lastIncludedIndex2 := indexT(lastIncludedIndex)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//if lastIncludedIndex2 <= rf.commitIndex {
	//	rf.logger.Printf("[%d] conflict when CondInstallSnapshot\nLast Include Index[%d]\tCommit Index[%d]\n", rf.me, lastIncludedIndex2, rf.commitIndex)
	//	return false
	//}
	rf.logLatch.Lock()
	defer rf.logLatch.Unlock()
	if lastIncludedTerm == int(rf.getLastIncludeTerm()) && lastIncludedIndex == int(rf.getLastIncludeIndex()) {
		rf.logger.Printf("replicate snapshot") // 重复安装的快照
		return false
	}
	if lastIncludedIndex2 < rf.getLastLogIndex() && rf.getLastLogTerm() == termT(lastIncludedTerm) {
		// 	If existing log entry has same index and Term as snapshot’s
		//	last included entry, retain log entries following it and reply
		rf.logs = append([]Log(nil), rf.logs[rf.getLogOffset()+lastIncludedTerm:]...)
	} else {
		// Discard the entire log
		rf.logs = append([]Log(nil), Log{
			Term:    termT(lastIncludedTerm),
			Index:   lastIncludedIndex2,
			Command: nil,
		})
	}
	rf.snapshot = snapshot
	rf.commitIndex = lastIncludedIndex2
	rf.applyIndex = lastIncludedIndex2
	rf.saveRaftStateAndSnapshot()
	if debug {
		rf.logger.Printf("[%d]Install Snapshot\nCommitIndex[%d]", rf.me, rf.commitIndex)
	}
	return true
}

type InstallSnapshotArgs struct {
	Term              termT
	LeaderId          int
	LastIncludedIndex indexT
	LastIncludedTerm  termT
	// offset int
	Data []byte
	// done bool
}
type InstallSnapshotReply struct {
	Term termT
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	if debug {
		rf.logger.Printf("[%d] Receive Snapshot\n", rf.me)
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term > rf.currentTerm {
		rf.becomeFollower(args.Term)
	}
	reply.Term = rf.currentTerm
	rf.logLatch.RLock()
	if args.Term < rf.currentTerm || args.LastIncludedIndex <= rf.getLastIncludeIndex() {
		rf.logLatch.RUnlock()
		return
	}
	rf.setElectionTime()
	apply := ApplyMsg{
		CommandValid:  false,
		Command:       nil,
		CommandIndex:  -1,
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  int(args.LastIncludedTerm),
		SnapshotIndex: int(args.LastIncludedIndex),
	}
	rf.logLatch.RUnlock()
	go func() { rf.applyCh <- apply }()
	go rf.apply()
}

// Snapshot the service says it has created a snapshot that has
// all info up to and including Index. this means the
// service no longer needs the log through (and including)
// that Index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	if debug {
		rf.logger.Printf("[%d] Snapshot to index [%d]\n", rf.me, index)
	}
	idx := indexT(index)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.logLatch.Lock()
	defer rf.logLatch.Unlock()
	if idx <= rf.getLastIncludeIndex() {
		rf.logger.Println("Outdated Snapshot")
		return
	}
	for i, l := range rf.logs {
		if l.Index == idx {
			tmpLogs := make([]Log, 1)
			tmpLogs[0] = Log{
				Term:    l.Term,
				Index:   idx,
				Command: nil,
			}
			rf.logs = append(tmpLogs, rf.logs[i+1:]...)
		}
	}
	rf.snapshot = snapshot
	rf.commitIndex = indexT(index)
	rf.applyIndex = indexT(index)
	rf.saveRaftStateAndSnapshot()
}

// RequestVoteArgs example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your Data here (2A, 2B).
	CandidateTerm termT
	CandidateId   int
	LastLogEntry  indexT
	LastLogTerm   termT
}

// RequestVoteReply example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your Data here (2A).
	Term        termT
	VoteGranted bool
}

// RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.currentTerm >= args.CandidateTerm {
		reply.VoteGranted = false
		return
	}
	if args.CandidateTerm > rf.currentTerm {
		rf.status = follower
		rf.setTerm(args.CandidateTerm)
	}
	reply.Term = rf.currentTerm
	lastLogTerm := rf.getLastLogTerm()
	lastLogIndex := rf.getLastLogIndex()
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) &&
		(lastLogTerm < args.LastLogTerm || (lastLogTerm == args.LastLogTerm && lastLogIndex <= args.LastLogEntry)) {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		rf.logger.Printf("[%d] Vote To [%d] In Term [%d]\n", rf.me, rf.votedFor, rf.currentTerm)
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
			rf.logger.Printf("[%d] become leader in Term [%d]", rf.me, rf.currentTerm)
			/**
			nextIndex for each server, Index of the next log entry to send to that server (initialized to leader last log Index + 1)
			*/
			nextIndex := rf.getLastLogIndex() + 1
			rf.status = leader
			for i := 0; i < rf.peerNum; i++ {
				rf.nextIndex[i] = nextIndex
				rf.matchIndex[i] = 0
			}
			// no op
			//rf.logLatch.Lock()
			//rf.logs = append(rf.logs, Log{
			//	Term:    rf.currentTerm,
			//	Index:   nextIndex,
			//	Command: nil,
			//})
			//rf.persist()
			//// rf.logger.Printf("Leader [%d] \nLog [%v]\n", rf.me, rf.logs)
			//rf.logLatch.Unlock()
			go rf.appendEntries(true)
		})
	}
	return ok
}

// Start : the service using Raft (e.g. a k/v server) wants to start
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

	var index indexT = -1
	// Your code here (2B).
	term, isLeader := rf.GetState()
	if !isLeader {
		return int(index), term, isLeader
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if debug {
		rf.logger.Printf("Leader [%d] Receive Log [%v]\n", rf.me, command)
	}
	rf.logLatch.Lock()
	index = rf.getLastLogIndex() + 1
	rf.logs = append(rf.logs, Log{
		Term:    termT(term),
		Index:   index,
		Command: command,
	})
	rf.persist()
	// rf.logger.Printf("Leader [%d] \nLog [%v]\n", rf.me, rf.logs)
	rf.logLatch.Unlock()
	// rf.appendEntries(false)
	return int(index), term, isLeader
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

/*
*
ticker :  go routine starts a new election if this peer hasn't received

	heartbeats recently.
*/
func (rf *Raft) ticker() {
	rf.mu.Lock()
	rf.setElectionTime()
	rf.mu.Unlock()
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		time.Sleep(rf.heartBeat)
		rf.mu.Lock()
		if rf.status == leader {
			// 如果是leader状态,发送空包
			rf.setElectionTime()
			rf.mu.Unlock()
			rf.appendEntries(true)
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
	if !debug {
		rf.logger.SetOutput(io.Discard)
	}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// rf.peers[me].Call()
	// Your initialization code here (2A, 2B, 2C).
	rf.votedFor = -1 // 2a
	// initialize from state persisted before a crash
	if persister != nil {
		rf.readPersist(persister.ReadRaftState())
		rf.readSnapshot(persister.ReadSnapshot())
	}
	if rf.logs == nil || len(rf.logs) == 0 {
		// 添加一条空log
		rf.logs = append(rf.logs, Log{
			Term:    0,
			Index:   0,
			Command: nil,
		})
		rf.persist()
	}
	rf.heartBeat = heartBeatTime
	rf.peerNum = len(peers)
	rf.status = follower

	rf.nextIndex = make([]indexT, rf.peerNum)
	rf.matchIndex = make([]indexT, rf.peerNum)
	rf.applyCh = applyCh
	rf.applySign = make(chan bool)

	rf.commitIndex = rf.getLastIncludeIndex()
	rf.applyIndex = rf.getLastIncludeIndex()
	lastLogIndex := rf.getLastLogIndex()

	for i := range rf.nextIndex {
		rf.nextIndex[i] = lastLogIndex + 1
	}
	// start ticker goroutine to start elections
	go rf.ticker()
	go rf.doApply()
	return rf
}

func (rf *Raft) setElectionTime() {
	// rf.logger.Printf("[%d] Set ElectionTime", rf.me)
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
		log.Fatalln("Log Len is Zero")
	}
	return rf.logs[len(rf.logs)-1].Index
}

// getLastLogIndex returns the last Term of logs
func (rf *Raft) getLastLogTerm() termT {
	if len(rf.logs) == 0 {
		log.Fatalln("Log Len is Zero")
	}
	return rf.logs[len(rf.logs)-1].Term
}
func (rf *Raft) setTerm(t termT) {
	rf.currentTerm = t
	rf.votedFor = -1
	rf.persist()
}
func (rf *Raft) appendEntries(heartBeat bool) {
	for i := range rf.peers {
		rf.mu.Lock()
		lastIndex := rf.getLastLogIndex()
		if i != rf.me && (heartBeat || lastIndex >= rf.nextIndex[i]) {
			rf.logLatch.RLock()
			if rf.nextIndex[i] > lastIndex {
				rf.nextIndex[i] = lastIndex + 1
			}
			if debug {
				rf.logger.Printf("Leader[%d]\t handling[%d]\tLastIncludeIndex:%d\n ApplyIndex:[%d]\tCommitIndex:[%d]\nLog:[%v]\n NextIndex:[%d]\n", rf.me, i, rf.getLastIncludeIndex(), rf.applyIndex, rf.commitIndex, rf.logs, rf.nextIndex[i])
			}
			if rf.nextIndex[i] <= rf.getLastIncludeIndex() {
				rf.logLatch.RUnlock()
				// 已经不包含需要的Log了
				go func(rf *Raft, peerId int) {
					// 发送快照
					rf.mu.Lock()
					rf.logLatch.RLock()

					args := InstallSnapshotArgs{
						Term:              rf.currentTerm,
						LeaderId:          rf.me,
						LastIncludedIndex: rf.getLastIncludeIndex(),
						LastIncludedTerm:  rf.getLastIncludeTerm(),
						Data:              rf.snapshot,
					}
					if debug {
						rf.logger.Printf("%d send snapshot to %d\n", rf.me, peerId)
					}
					rf.logLatch.RUnlock()
					rf.mu.Unlock()
					var reply InstallSnapshotReply
					rf.peers[peerId].Call("Raft.InstallSnapshot", &args, &reply)
					rf.mu.Lock()
					defer rf.mu.Unlock()
					if reply.Term < rf.currentTerm {
						rf.logger.Println("Receive Outdated Install Snapshot RPC reply")
						return
					}
					if reply.Term > rf.currentTerm {
						rf.becomeFollower(reply.Term)
						return
					}
					rf.nextIndex[peerId] = args.LastIncludedIndex + 1
					rf.matchIndex[peerId] = args.LastIncludedIndex
					if debug {
						rf.logger.Printf("%d send snapshot to %d success\n", rf.me, peerId)
						rf.logger.Printf("Leader[%d] NextIndex Of [%d] Update To [%d]\n", rf.me, peerId, rf.nextIndex[peerId])
					}

				}(rf, i)
				rf.mu.Unlock()
				continue
			}
			prevLog := rf.getIthIndex(rf.nextIndex[i] - 1)
			rf.logLatch.RUnlock()
			args := appendEntryArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLog.Index,
				PrevLogTerm:  prevLog.Term,
				LeaderCommit: rf.commitIndex,
				Entries:      make([]Log, lastIndex-rf.nextIndex[i]+1),
			}
			rf.logLatch.RLock()
			copy(args.Entries, rf.logs[int(rf.nextIndex[i])+rf.getLogOffset():])
			rf.logLatch.RUnlock()
			go func(rf *Raft, args *appendEntryArgs, peerId int) {
				rf.mu.Lock()
				client := rf.peers[peerId]
				var reply appendEntryReply
				if debug {
					if len(args.Entries) > 0 {
						rf.logger.Printf("[%d] Send AppendRPC To [%d]\n[%s]\nLogs: [%v]", rf.me, peerId, args.String(), args.Entries)
					} else {
						rf.logger.Printf("[%d] Send Heartbeat To [%d]\n[%s]\npTerm: %d\tpIndex: %d\n", rf.me, peerId, args.String(), args.PrevLogTerm, args.PrevLogIndex)
					}
				}

				rf.mu.Unlock()
				client.Call("Raft.AppendEntriesRPC", args, &reply)
				rf.mu.Lock()
				defer rf.mu.Unlock()
				if reply.Term > rf.currentTerm {
					rf.logger.Printf("I'm Old\n")
					rf.setTerm(reply.Term)
					rf.setElectionTime()
					rf.status = follower
					return
				}
				if reply.Term < rf.currentTerm || rf.status != leader {
					if debug {
						rf.logger.Printf("Leader Receive Outdated RPC\n")
					}
					return
				}
				if debug {
					rf.logger.Printf("Leader[%d] receive from [%d]reply : success:[%v] conflict[%v] hearBeat[%v]\n XTerm: %d\tXIndex: %d\n", rf.me, peerId, reply.Success, reply.Conflict,
						heartBeat, reply.XTerm, reply.XIndex)
				}
				rf.logLatch.RLock()
				defer rf.logLatch.RUnlock()
				if reply.Conflict {
					if reply.XTerm != -1 {
						lastIndexInXTerm := rf.getLastLogIndexInXTerm(reply.XTerm, int(reply.XIndex))
						if debug {
							rf.logger.Printf("Leader[%d] Logs:[\n%v]\n LastIndex In X Term: %d", rf.me, rf.logs, lastIndexInXTerm)
							rf.logger.Println("Term Conflict")
						}
						if lastIndexInXTerm == -1 {
							rf.nextIndex[peerId] = reply.XIndex
						} else {
							rf.nextIndex[peerId] = indexT(lastIndexInXTerm + 1)
						}
					} else {
						rf.logger.Println("Follower Too Short")
						rf.logger.Printf("Leader[%d] XIndex: %d\n", rf.me, reply.XIndex)
						rf.nextIndex[peerId] = reply.XIndex + 1
					}
					rf.logger.Printf("Leader[%d] NextIndex Of [%d] Update To [%d]\n", rf.me, peerId, rf.nextIndex[peerId])
				} else if reply.Success && len(args.Entries) > 0 {
					rf.logger.Println("Success Update")
					rf.matchIndex[peerId] = max(args.PrevLogIndex+indexT(len(args.Entries)), rf.matchIndex[peerId])
					rf.nextIndex[peerId] = max(rf.nextIndex[peerId], rf.matchIndex[peerId]+1)
					rf.logger.Printf("Leader[%d] NextIndex Of [%d] Update To [%d]\n", rf.me, peerId, rf.nextIndex[peerId])
				}
				go rf.checkCommit()
			}(rf, &args, i)
		}
		rf.mu.Unlock()
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
	Term     termT // currentTerm, for leader to update itself
	Success  bool  // true if follower contained entry matching PrevLogIndex and PrevLogTerm
	Conflict bool
	XTerm    termT
	XIndex   indexT
}

func (rf *Raft) AppendEntriesRPC(args *appendEntryArgs, reply *appendEntryReply) {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Success = false
	reply.Conflict = false
	reply.Term = rf.currentTerm
	if args.Term > rf.currentTerm {
		rf.status = follower
		rf.setElectionTime() // check(AntiO2)
		rf.setTerm(args.Term)
		rf.logger.Printf("[%d] become a follower\n", rf.me)
		return
	}
	if args.Term < rf.currentTerm {
		rf.logger.Printf("[%d] Receive Outdated RPC\n", rf.me)
		return
	}
	rf.setElectionTime()
	if rf.status == candidate {
		rf.status = follower
	}
	reply.Success = false
	rf.logLatch.RLock()
	lastIndex := rf.getLastLogIndex()
	// rf.logger.Printf("[%d] Receive Append Entry \nLastLogIs:[%d]\n%s\nLogs: %v\n", rf.me, lastIndex, args.String(), rf.logs)
	if args.PrevLogIndex > lastIndex {
		if debug {
			rf.logger.Printf("[%d] receive beyond conflict logs\nlogs[%v]", rf.me, rf.logs)
		}
		reply.Conflict = true
		reply.XTerm = -1
		reply.XIndex = rf.getLastLogIndex()
		rf.logLatch.RUnlock()
		return
	}
	if rf.getIthIndex(args.PrevLogIndex).Term != args.PrevLogTerm {
		rf.logger.Printf("[%d] receive prev Term conflict logs", rf.me)
		reply.Conflict = true
		reply.XTerm = rf.getIthIndex(args.PrevLogIndex).Term
		reply.XIndex = rf.getFirstLogIndexInXTerm(reply.XTerm, args.PrevLogIndex)
		rf.logLatch.RUnlock()
		return
	}

	rf.logLatch.RUnlock()
	rf.logLatch.Lock()
	//  if args.Entries != nil && len(args.Entries) != 0 {
	//		rf.logger.Printf("[%d] append [%d] logs\nprev rf's logs: [%v]\nnew logs: [%v]", rf.me, len(args.Entries), rf.logs, args.Entries)
	// 	s}
	if debug {
		rf.logger.Printf("[%d] receive no conflict logs\nApplyIndex is [%d]\tcommitIndex is [%d]\tleaderCommit [%d]\n", rf.me, rf.applyIndex, rf.commitIndex, args.LeaderCommit)
	}

	lastIndex = rf.getLastLogIndex()
	for i, entry := range args.Entries {
		if entry.Index <= rf.getLastLogIndex() && entry.Term != rf.getIthIndex(entry.Index).Term {
			// conflict
			rf.logs = append(rf.logs[:int(entry.Index)+rf.getLogOffset()], entry)
			rf.persist()
		}
		if entry.Index > rf.getLastLogIndex() {
			// Append any new entries not already in the log
			rf.logs = append(rf.logs, args.Entries[i:]...)
			break
		}
	}
	if args.Entries != nil && len(args.Entries) != 0 && debug {
		rf.logger.Printf("[%d] append [%d] logs\nrf's logs: [%v]\nnew logs: [%v]", rf.me, len(args.Entries), rf.logs, args.Entries)
	}
	rf.persist()
	rf.logLatch.Unlock()
	reply.Success = true
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, rf.getLastLogIndex())
		go rf.apply()
	}
	return
}

func (rf *Raft) apply() {
	rf.applySign <- true
}
func (rf *Raft) doApply() {

	for !rf.killed() {
		<-rf.applySign
		rf.mu.Lock()
		lastLogIndex := rf.getLastLogIndex()
		for lastLogIndex > rf.applyIndex && rf.commitIndex > rf.applyIndex {
			rf.applyIndex++
			msg := ApplyMsg{
				CommandValid:  true,
				Command:       rf.getIthIndex(rf.applyIndex).Command,
				CommandIndex:  int(rf.applyIndex),
				SnapshotValid: false,
				Snapshot:      nil,
				SnapshotTerm:  0,
				SnapshotIndex: 0,
			}
			rf.mu.Unlock()
			rf.applyCh <- msg
			rf.mu.Lock()
			rf.logger.Printf("[%d] Apply [%d]\n", rf.me, msg.CommandIndex)
		}
		rf.mu.Unlock()
	}
}

func (rf *Raft) checkCommit() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.status != leader {
		return
	}
	lastIndex := rf.getLastLogIndex()
	rf.logLatch.RLock()
	defer rf.logLatch.RUnlock()
	for i := rf.commitIndex + 1; i <= lastIndex; i++ {
		rf.logger.Printf("[%d] Check if [%d] Could Commit", rf.me, i)
		if rf.getIthIndex(i).Term != rf.currentTerm {
			// 5.4.2 Committing entries from previous terms
			continue
		}
		count := 0
		for j := 0; j < rf.peerNum; j++ {
			if j == rf.me {
				count++
			} else {
				if rf.matchIndex[j] >= i {
					count++
				}
			}
		}
		if count > rf.peerNum/2 {
			rf.logger.Printf("[%d]'s commitIndex Update To [%d]\n", rf.me, i)
			rf.commitIndex = i
			go rf.apply()
		}
	}
}

func (a appendEntryArgs) String() string {
	var args string
	args = "Leader: " + strconv.Itoa(a.LeaderId) + "\n" +
		"Term: " + strconv.Itoa(int(a.Term)) + "\n" +
		"PrevLogTerm" + strconv.Itoa(int(a.PrevLogTerm)) + "\n" +
		"PrevLogIndex" + strconv.Itoa(int(a.PrevLogIndex)) + "\n"
	return args
}
func (rf *Raft) getIthIndex(t indexT) *Log {
	idx := int(t) + rf.getLogOffset()
	if idx >= len(rf.logs) {
		rf.logger.Fatalf("Try get index [%d] Out of range\n", t)
	}
	return &rf.logs[idx]
}

// getLastLogIndexInXTerm 返回rf.logs在xTerm中最后一条log的下标
// 如果完全没有xTerm,返回-1
func (rf *Raft) getLastLogIndexInXTerm(xTerm termT, xIndex int) int {
	idx := rf.getLogOffset() + xIndex
	if rf.logs[idx].Term != xTerm {
		return -1
	}
	for idx < len(rf.logs)-1 {
		if rf.logs[idx+1].Term != xTerm {
			return idx
		}
		idx++
	}
	return idx + int(rf.getLastIncludeIndex())
}

func (rf *Raft) getFirstLogIndexInXTerm(xTerm termT, prevIndex indexT) indexT {
	// rf.logger.Printf("In Get FirstLogIndexInXTerm\nS[%d]\nlogs: %v", rf.me, rf.logs)
	idx := min(prevIndex, rf.getLastLogIndex())
	if rf.getIthIndex(idx).Term != xTerm {
		log.Fatalln("Error Use getFirstLogIndexInXTerm") // 初始状态下，logs[idx]一定等于xTerm
	}
	for int(idx)+rf.getLogOffset() > 0 {
		if rf.getIthIndex(idx-1).Term != xTerm {
			break
		}
		idx--
	}
	return idx
}

func (rf *Raft) getLastIncludeIndex() indexT {
	return rf.logs[0].Index
}
func (rf *Raft) getLastIncludeTerm() termT {
	return rf.logs[0].Term
}

func (rf *Raft) getLogOffset() int {
	return int(-rf.logs[0].Index)
}
func (rf *Raft) becomeFollower(t termT) {
	rf.status = follower
	rf.setTerm(t)
}
func (rf *Raft) saveRaftStateAndSnapshot() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(rf.currentTerm) != nil ||
		e.Encode(rf.votedFor) != nil ||
		e.Encode(rf.logs) != nil {
		rf.logger.Fatalln("Error When Encode State")
	}
	rf.persister.SaveStateAndSnapshot(w.Bytes(), rf.snapshot)
}
