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
	replicationInterval time.Duration = 70 * time.Millisecond
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

	//log in Peer's local
	log []LogEntry
	//these item are only using by leader
	//log view for every peer
	nextIndex  []int
	matchIndex []int

	//commit index and last applied
	commitIndex int //record the global apply point
	lastApplied int // record the peer's apply point individualy
	applyCond   *sync.Cond
	applyCh     chan ApplyMsg
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
	shouldPersist := rf.currentTerm != term
	if term > rf.currentTerm {
		rf.voteFor = -1
	}
	// update to large term, which means current peer has invloved in current leader's selection
	rf.currentTerm = term
	if shouldPersist {
		rf.persistLocked()
	}
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
	rf.persistLocked()

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
	for peer := 0; peer < len(rf.peers); peer++ {
		rf.nextIndex[peer] = len(rf.log)
		rf.matchIndex[peer] = 0
	}
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (PartA).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.role == Leader
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (PartD).

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
	//Part B
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.role != Leader {
		return 0, 0, false
	}
	rf.log = append(rf.log, LogEntry{
		CommandValid: true,
		Command:      command,
		Term:         rf.currentTerm,
	})
	LOG(rf.me, rf.currentTerm, DLeader, "Leader accept log [%d]T%d", len(rf.log)-1, rf.currentTerm)
	rf.persistLocked()
	return len(rf.log) - 1, rf.currentTerm, true
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

func (rf *Raft) contextCheckLocked(role Role, term int) bool {
	return !(rf.currentTerm == term && rf.role == role)
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

	//Part b.1
	rf.log = append(rf.log, LogEntry{})

	rf.matchIndex = make([]int, len(rf.peers))
	rf.nextIndex = make([]int, len(rf.peers))
	//Part b.3
	rf.applyCh = applyCh
	rf.applyCond = sync.NewCond(&rf.mu)
	rf.commitIndex = 0
	rf.lastApplied = 0

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	// start ticker goroutine to start elections
	go rf.electionTicker()
	go rf.applyTicker()

	return rf
}
