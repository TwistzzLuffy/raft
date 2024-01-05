package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"course/labgob"
	"course/labrpc"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part PartD you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.

// Time
const (
	electionTimeoutMin  time.Duration = 250 * time.Millisecond
	electionTimeoutMax  time.Duration = 400 * time.Millisecond
	replicationInterval time.Duration = 200 * time.Millisecond
)

type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For PartD:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}
type Role string

const (
	Follower  Role = "Follower"
	Candidate Role = "Candidate"
	Leader    Role = "Leader"
)

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (PartA, PartB, PartC).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int // the term of current peer
	voteFor     int // vote for which peer, -1 means vote for none
	role        Role

	electionStartTime time.Time
	electionTimeout   time.Duration
}

// reset time
func (rf *Raft) resetElectionTimeLocked() {
	rf.electionStartTime = time.Now()
	timeRange := int64(electionTimeoutMax - electionTimeoutMin)
	rf.electionTimeout = electionTimeoutMin + time.Duration(rand.Int63()%timeRange)
}

// check if time out
func (rf *Raft) isElectionTimeoutLocked() bool {
	return time.Since(rf.electionStartTime) > rf.electionTimeout
}

// to the Follower status
func (rf *Raft) becomeFollowerLocked(term int) {
	if term < rf.currentTerm {
		//ignore this message
		LOG(rf.me, rf.currentTerm, DError, "Can't become Follower, lower term")
		return
	}
	//term > or ==
	LOG(rf.me, rf.currentTerm, DLog, "%s -> Follower, For T %d -> %d",
		rf.role, rf.currentTerm, term)
	// important! Could only reset the `votedFor` when term increased
	rf.role = Follower
	if term > rf.currentTerm {
		rf.voteFor = -1
	}
	// update to large term, which means current peer has invloved in current leader's selection
	rf.currentTerm = term
}

// to the Candidate Stauts
func (rf *Raft) becomeCandidateLocked() {
	if rf.role == Leader {
		//can't from leader to candidate
		LOG(rf.me, rf.currentTerm, DError, "Leader can't be candidate")
		return
	}
	LOG(rf.me, rf.currentTerm, DVote, "%s -> candidate, for T %d to T %d",
		rf.role, rf.currentTerm, rf.currentTerm+1)
	rf.currentTerm++
	rf.role = Candidate
	rf.voteFor = rf.me

}

// to be the Leader
func (rf *Raft) becomeLeaderLocked() {
	if rf.role != Candidate {
		//only the candidate can be leader
		LOG(rf.me, rf.currentTerm, DLeader,
			"%s, Only candidate can become Leader", rf.role)
		return
	}
	LOG(rf.me, rf.currentTerm, DLeader, "%s -> Leader, For T%d",
		rf.role, rf.currentTerm)
	rf.role = Leader
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (PartA).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.role == Leader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// Your code here (PartC).
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
	// Your code here (PartC).
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

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (PartD).

}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (PartA, PartB).
	Term        int //candidate term
	CandidateId int //which candidate requesting vote
	//LastLogIndex int //last index of candidate's last log entry
	//LastLogTerm  int //term of candidate's last log entry
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (PartA).
	Term       int  //current term, for candidate update itself
	VotGranted bool // true means candidate received vote
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (PartA, PartB).
	//Tip: there is handling the RPC request, will be automatically called by RPC
	//align the term
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		LOG(rf.me, rf.currentTerm, DVote, "-> S%d, Reject vote, higher term, T%d>T%d",
			args.CandidateId, rf.currentTerm, args.Term)
		reply.VotGranted = false
		return
	}
	if args.Term > rf.currentTerm {
		rf.becomeFollowerLocked(args.Term)
		return
	}
	if rf.voteFor != -1 {
		LOG(rf.me, rf.currentTerm, DVote, "-> S%d, Reject, Already voted S%d",
			args.CandidateId, rf.voteFor)
		reply.VotGranted = false
		return
	}
	reply.VotGranted = true
	rf.voteFor = args.CandidateId
	//reset the selectiontime
	rf.resetElectionTimeLocked()
	LOG(rf.me, rf.currentTerm, DVote, "-> S%d", args.CandidateId)
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

	// Your code here (PartB).

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

//check if time is out and start election
/*
1.regularly check if time is out
2.check the condition of start selection
*/
func (rf *Raft) electionTicker() {
	for !rf.killed() {
		// Your code here (PartA)
		// Check if a leader election should be started.
		//when time is right to start selection
		rf.mu.Lock()
		if rf.role != Leader && rf.isElectionTimeoutLocked() {
			rf.becomeCandidateLocked()
			go rf.startElection(rf.currentTerm)
		}
		rf.mu.Unlock()
		// pause for a random amount of time between 50 and 350
		// milliseconds.
		//Tip: 随机检测点，如果检测点不随机回导致超时机制的随机没有意义
		ms := 50 + (rand.Int63() % 300)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

/*
1.send request to all other peer
2.account the vote
*/
func (rf *Raft) startElection(term int) bool {
	voteNumber := 0
	askVoteFromPeer := func(peer int, arg *RequestVoteArgs) {
		//send rpc
		reply := &RequestVoteReply{}
		ok := rf.sendRequestVote(peer, arg, reply)

		rf.mu.Lock()
		defer rf.mu.Unlock()
		if !ok {
			//send fault
			//LOG(rf.me, rf.currentTerm, DDebug, "Ask vote from %d, Lost or error", peer)
			return
		}
		// handle the reply
		//align term
		//LOG(rf.me, rf.currentTerm, DDebug, "I am here")
		if reply.Term > term {
			rf.becomeFollowerLocked(reply.Term)
			return
		}

		// check the context
		if rf.contextCheckLocked(Candidate, rf.currentTerm) {
			LOG(rf.me, rf.currentTerm, DVote, "Lost context, abort RequestVoteReply in T%d", rf.currentTerm)
			return
		}
		//LOG(rf.me, rf.currentTerm, DDebug, "I am here")
		if reply.VotGranted {
			//success for receiving the vote
			LOG(rf.me, rf.currentTerm, DDebug, "%d receive the vote", rf.me)
			voteNumber++
		}

		if voteNumber > len(rf.peers)/2 {
			rf.becomeLeaderLocked()
			go rf.replicationTicker(term)
		}
	}

	//after that, we modify the public variable, so need using lock
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.contextCheckLocked(Candidate, term) {
		return false
	}

	for peer := 0; peer < len(rf.peers); peer++ {
		if peer == rf.me {
			voteNumber++
			continue
		}
		arg := &RequestVoteArgs{
			Term:        term,
			CandidateId: rf.me,
		}
		go askVoteFromPeer(peer, arg)
	}
	return true
}

func (rf *Raft) contextCheckLocked(role Role, term int) bool {
	return !(rf.currentTerm == term && rf.role == role)
}

/************************Replication*********/
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.Success = false

	//align the term
	if args.Term < rf.currentTerm {
		LOG(rf.me, rf.currentTerm, DLog2, "<- S%d, Reject log", args.LeaderId)
		return
	}
	if args.Term >= rf.currentTerm {
		rf.becomeFollowerLocked(args.Term)
	}
	//reset the timer
	rf.resetElectionTimeLocked()
	reply.Success = true
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int
}
type AppendEntriesReply struct {
	Term    int
	Success bool
}

func (rf *Raft) replicationTicker(term int) {
	for !rf.killed() {
		ok := rf.startReplication(term)
		if !ok {
			//check the success of heartbeats
			return
		}
		time.Sleep(replicationInterval)
	}
}

func (rf *Raft) startReplication(term int) bool {
	replicationToPeer := func(peer int, args *AppendEntriesArgs) {
		reply := &AppendEntriesReply{}
		ok := rf.sendAppendEntries(peer, args, reply)

		rf.mu.Lock()
		defer rf.mu.Unlock()
		if !ok {
			LOG(rf.me, rf.currentTerm, DLog, "-> S%d, Lost or crashed", peer)
			return
		}
		//align the term
		if reply.Term > rf.currentTerm {
			rf.becomeFollowerLocked(reply.Term)
			return
		}
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.contextCheckLocked(Leader, term) {
		LOG(rf.me, rf.currentTerm, DLeader, "Leader[T%d] -> %s[T%d]", term, rf.role, rf.currentTerm)
		//current term is ending, and this round of heartbeat is fault
		return false
	}

	for peer := 0; peer < len(rf.peers); peer++ {
		if peer == rf.me {
			//send the heartbeat to everyone, expect itself
			continue
		}

		args := &AppendEntriesArgs{
			Term:     term,
			LeaderId: rf.me,
		}

		go replicationToPeer(peer, args)
	}

	return true
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
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (PartA, PartB, PartC).
	rf.currentTerm = 0
	rf.voteFor = -1
	rf.role = Follower

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	// start ticker goroutine to start elections
	go rf.electionTicker()

	return rf
}
