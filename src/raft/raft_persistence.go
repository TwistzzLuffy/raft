package raft

import (
	"bytes"
	"course/labgob"
	"fmt"
)

func (rf *Raft) persisitString() string {
	return fmt.Sprintf("T%d, VoteFor:%d, Log: [0, %d)", rf.currentTerm, rf.voteFor, len(rf.log)-1)
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persistLocked() {
	// Your code here (PartC).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// raftstate := w.Bytes()
	// rf.persister.Save(raftstate, nil)
	// Your code here (PartC).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.voteFor)
	e.Encode(rf.log)
	raftstate := w.Bytes()
	rf.persister.Save(raftstate, nil)
	LOG(rf.me, rf.currentTerm, "Persisit: %v", rf.persisitString())
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (PartC).
	var currentTerm int
	var voteFor int
	var log []LogEntry
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	if err := d.Decode(&currentTerm); err != nil {
		LOG(rf.me, rf.currentTerm, DPersist, "Read currentTerm err: %v", err)
	}
	rf.currentTerm = currentTerm
	if err := d.Decode(&voteFor); err != nil {
		LOG(rf.me, rf.currentTerm, DPersist, "Read votFor err: %v", err)
	}
	rf.voteFor = voteFor
	if err := d.Decode(&log); err != nil {
		LOG(rf.me, rf.currentTerm, DPersist, "Read Log err: %v", err)
	}
	rf.log = log
	LOG(rf.me, rf.currentTerm, DPersist, "Read from persist disk: %v", rf.persisitString())
}
