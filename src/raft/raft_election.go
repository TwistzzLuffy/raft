package raft

import (
	"math/rand"
	"time"
)

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

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (PartA, PartB).
	Term        int //candidate term
	CandidateId int //which candidate requesting vote
	//PartB.1
	LastLogIndex int //last index of candidate's last log entry
	LastLogTerm  int //term of candidate's last log entry
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
	reply.VotGranted = false
	if args.Term < rf.currentTerm {
		LOG(rf.me, rf.currentTerm, DVote, "-> S%d, Reject Vote, higher term, T%d>T%d",
			args.CandidateId, rf.currentTerm, args.Term)
		return
	}
	if args.Term > rf.currentTerm {
		rf.becomeFollowerLocked(args.Term)
	}
	if rf.voteFor != -1 {
		LOG(rf.me, rf.currentTerm, DVote, "-> S%d, Reject Vote, Already voted S%d",
			args.CandidateId, rf.voteFor)
		return
	}
	//compare the log
	if rf.isMoreUpToDataLocked(args.LastLogIndex, args.LastLogTerm) {
		LOG(rf.me, rf.currentTerm, DVote, "-> S%d, Reject Vote, S%d's log less up-to-date",
			args.CandidateId)
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

// check who log is more update
func (rf *Raft) isMoreUpToDataLocked(candidateIndex, candidateTerm int) bool {
	l := len(rf.log)
	lastIndex, lastTerm := l-1, rf.log[l-1].Term
	LOG(rf.me, rf.currentTerm, DVote, "Compare last log, Me: [%d]T%d, Candidate: [%d]T%d",
		lastIndex, lastTerm, candidateIndex, candidateTerm)
	//1. If the logs have last entries with different terms,
	//then the log with the later term is more up-to-date.
	//2. If the logs end with the same term,
	//then whichever log is longer is more up-to-da
	if lastTerm != candidateTerm {
		return lastTerm > candidateTerm
	}
	return lastIndex > candidateIndex

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

	l := len(rf.log)
	for peer := 0; peer < len(rf.peers); peer++ {
		if peer == rf.me {
			voteNumber++
			continue
		}
		arg := &RequestVoteArgs{
			Term:         term,
			CandidateId:  rf.me,
			LastLogIndex: l - 1,
			LastLogTerm:  rf.log[l-1].Term,
		}
		go askVoteFromPeer(peer, arg)
	}
	return true
}
