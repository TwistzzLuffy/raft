package raft

import (
	"sort"
	"time"
)

// it is according to the ApplyMsg struct
type LogEntry struct {
	Term         int
	CommandValid bool
	Command      interface{}
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int

	PrevLogIndex int //// PrevLogIndex and PrevLogTerm is used to match the
	PrevLogTerm  int
	Entries      []LogEntry // Entries is used to append when matched

	//for log application
	LeaderCommit int
}
type AppendEntriesReply struct {
	Term    int
	Success bool
}

/************************Replication*********/
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	//For debug partB.1
	LOG(rf.me, rf.currentTerm, DDebug, "<- S%d, Receive log, Pre=[%d]T%d, Len()=%d",
		args.LeaderId, args.PrevLogIndex, args.PrevLogTerm, len(args.Entries))
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
	//return failure if the pre log not matched
	if args.PrevLogIndex >= len(rf.log) {
		//out of the index of this peer
		LOG(rf.me, rf.currentTerm, DLog2, "<- S%d, Reject Log, Follower log too short, Len:%d <= Prev:%d",
			args.LeaderId, len(rf.log), args.PrevLogIndex)
		return
	}
	if rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		LOG(rf.me, rf.currentTerm, DLog2, "<- S%d, Reject Log, Prev log not match, [%d]: T%d != T%d",
			args.LeaderId, args.PrevLogIndex, rf.log[args.PrevLogIndex].Term, args.PrevLogTerm)
		return
	}
	//append the leader log to the local
	rf.log = append(rf.log[:args.PrevLogIndex+1], args.Entries...)
	LOG(rf.me, rf.currentTerm, DLog2, "Follower append logs: (%d, %d]",
		args.PrevLogIndex, args.PrevLogIndex+len(args.Entries))
	reply.Success = true
	//TODO: handle the args.LeaderCommit
	if args.LeaderCommit > rf.commitIndex {
		//说明leader可能发生变更
		LOG(rf.me, rf.currentTerm, DApply, "Follower update the commit index %d->%d",
			rf.commitIndex, args.LeaderCommit)
		rf.commitIndex = args.LeaderCommit
		rf.applyCond.Signal()
	}
	//reset the timer
	rf.resetElectionTimeLocked()

}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
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
		//check context lost
		if rf.contextCheckLocked(Leader, term) {
			LOG(rf.me, rf.currentTerm, DLog, "-> S%d, Context Lost, T%d:Leader->T%d:%s", peer, term, rf.currentTerm, rf.role)
		}
		//probe the lower index if the pre log not matched
		if !reply.Success {
			idx := rf.nextIndex[peer] - 1
			term := rf.log[idx].Term
			for idx > 0 && rf.log[idx].Term == term {
				//Here's an optimisation, send the AppendRPC for each term, instead of each entry
				idx--
			}
			rf.nextIndex[peer] = idx + 1
			LOG(rf.me, rf.currentTerm, DLog, "->S%d, Not matched in %d, Update next=%d",
				peer, args.PrevLogIndex, rf.nextIndex[peer])
			return
		}
		//update the match/next index if log append successfully
		rf.matchIndex[peer] = args.PrevLogIndex + len(args.Entries)
		rf.nextIndex[peer] = rf.matchIndex[peer] + 1
		// TODO: need compute the new commitIndex here
		majorityMatched := rf.getMajorityIndexLocked()
		if majorityMatched > rf.commitIndex {
			LOG(rf.me, rf.currentTerm, DApply, "Leader update the commit index %d->%d",
				rf.commitIndex, majorityMatched)
			rf.commitIndex = majorityMatched
			rf.applyCond.Signal()
		}
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.contextCheckLocked(Leader, term) {
		LOG(rf.me, rf.currentTerm, DLeader, "Leader[T%d] -> %s[T%d]", term, rf.role, rf.currentTerm)
		//current term is ending, and this round of heartbeat is fault
		return false
	}
	//add var to debug PartB
	countfunc := 0

	for peer := 0; peer < len(rf.peers); peer++ {
		if peer == rf.me {
			//send the heartbeat to everyone, expect itself
			//update the leader log view.
			//the leader's latest log index
			rf.matchIndex[peer] = len(rf.log) - 1
			rf.nextIndex[peer] = len(rf.log)
			continue
		}
		prevIdx := rf.nextIndex[peer] - 1
		prevTerm := rf.log[prevIdx].Term

		args := &AppendEntriesArgs{
			Term:         term,
			LeaderId:     rf.me,
			PrevLogIndex: prevIdx,
			PrevLogTerm:  prevTerm,
			Entries:      rf.log[prevIdx+1:],
		}
		countfunc++
		LOG(rf.me, rf.currentTerm, DDebug, "-> Server: %d, Count time: %d", peer, countfunc)
		go replicationToPeer(peer, args)
	}

	return true
}

func (rf *Raft) getMajorityIndexLocked() int {
	tmpIndex := make([]int, len(rf.matchIndex))
	copy(tmpIndex, rf.matchIndex)
	sort.Ints(sort.IntSlice(tmpIndex))
	majorityIdx := (len(tmpIndex) - 1) / 2
	LOG(rf.me, rf.currentTerm, DDebug, "Match index after sort: %v, majority[%d]=%d",
		tmpIndex, majorityIdx, tmpIndex[majorityIdx])
	return tmpIndex[majorityIdx]
}
