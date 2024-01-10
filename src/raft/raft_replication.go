package raft

import "time"

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
