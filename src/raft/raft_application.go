package raft

func (rf *Raft) applyTicker() {
	for !rf.killed() {
		rf.mu.Lock()
		//wait() will release the rf's lock, liking spaining lock
		LOG(rf.me, rf.currentTerm, DDebug, "Sending msg to application")
		rf.applyCond.Wait()
		//condition is satisfied and going to apply
		entries := make([]LogEntry, 0)
		for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
			entries = append(entries, rf.log[i])
		}
		rf.mu.Unlock()

		//Now sending the command to application layer.
		//Tip: this will be going out of the critical are
		for i, entry := range entries {
			rf.applyCh <- ApplyMsg{
				CommandValid: entry.CommandValid,
				Command:      entry.Command,
				CommandIndex: rf.lastApplied + 1 + i,
			}
		}
		rf.mu.Lock()
		LOG(rf.me, rf.currentTerm, DApply, "Apply log for [%d, %d]",
			rf.lastApplied+1, rf.lastApplied+len(entries))
		rf.lastApplied += len(entries)
		//there is not suitable for using defer as the defer is function level.
		//we using in loop
		rf.mu.Unlock()
	}
}
