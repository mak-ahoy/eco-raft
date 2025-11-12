package raft

import (
	// "fmt"
	"labrpc"
	"math/rand"
	"sync"
	"time"
	"encoding/gob"
	"bytes"
)

const (
	Follower = iota
	Candidate
	Leader
)

type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for Assignment2; only used in Assignment3
	Snapshot    []byte // ignore for Assignment2; only used in Assignment3
}



type LogEntry struct {
	Command interface{}
	Term    int
}

type Raft struct {
	mu               sync.Mutex
	peers            []*labrpc.ClientEnd
	persister        *Persister
	me               int // index into peers[]

	currentTerm     int
	votedFor        int
	state           int // Follower, Candidate, Leader
	electionTimeout  time.Duration
	votesReceived    int
	heartbeatReceived chan struct{} // channel for heartbeat signals
	newCommitReadyChan chan struct{}  // sending to this chanel when a new commit is ready. 
	applyCh          chan ApplyMsg
	logs 			[]LogEntry 
	nextIndex       []int
	matchIndex      []int
	commitIndex     int 
    lastApplied     int 
	numValidResponses int
}

func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	return rf.currentTerm, rf.state == Leader
}

func (rf *Raft) persist() {
	// Your code here.
	w:= new(bytes.Buffer)
	e:= gob.NewEncoder(w)
	// rf.mu.Lock()
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)
	// rf.mu.Unlock()
	data := w.Bytes()
	rf.persister.SaveRaftState(data)

}

func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	r:= bytes.NewBuffer(data)
	d:= gob.NewDecoder(r)
	// rf.mu.Lock()
	d.Decode(&rf.currentTerm)
	d.Decode(&rf.votedFor)
	d.Decode(&rf.logs)
	// rf.mu.Unlock()

}

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

type RequestVoteArgs struct {
	Term        int
	CandidateId int
	LastLogIndex int
	LastLogTerm  int

}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	// rf.mu.Lock()
	// defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.state = Follower
		rf.persist()

	}
	
	lastLogIndexV := len(rf.logs) - 1
	// fmt.Printf("Last log index at follower %d \n", lastLogIndexV)
	var lastLogTermV int

	if lastLogIndexV == -1 {
		lastLogTermV = -1
	} else {
		lastLogTermV = rf.logs[lastLogIndexV].Term
	}

	// vote rejection logic for log in consistancy  -- new addition to handle election case. 
	if (lastLogTermV > args.LastLogTerm) ||
	(lastLogTermV == args.LastLogTerm) && (lastLogIndexV > args.LastLogIndex) {
		reply.VoteGranted = false
		return

	}

	// voting logic
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && args.Term == rf.currentTerm {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
	} else {
        reply.VoteGranted = false
    }

	rf.persist()

}

func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
    defer rf.mu.Unlock()

    if rf.state != Leader {
        return -1, -1, false
    }

	
	if len(rf.logs) != 0 {
		if rf.logs[len(rf.logs)-1].Command == command {
			return len(rf.logs) , rf.currentTerm, true
		}
	}
	
	index := len(rf.logs) + 1  // this will be the next index to append the new entry
	term := rf.currentTerm
	rf.logs = append(rf.logs, LogEntry{
		Command: command,
		Term:    term,
	})

	rf.persist()

   
    return index, term, true
}



func (rf *Raft) Kill() {
	// Optional cleanup logic
}

func Make(peers []*labrpc.ClientEnd, me int, persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{
		peers:           peers,
		persister:       persister,
		me:              me,
		votedFor:        -1,
		currentTerm:     0,
		state:           Follower,
		electionTimeout: time.Duration(rand.Int63n(400)+300) * time.Millisecond,
		applyCh:         applyCh,
		heartbeatReceived: make(chan struct{}),
		newCommitReadyChan: make(chan struct{}, 100),
		logs:             []LogEntry{}, 
		nextIndex:        make([]int, len(peers)), 
        matchIndex:       make([]int, len(peers)), 
		commitIndex:      0, 
        lastApplied:      0, 

	}


	for i := range rf.nextIndex {
        if i != rf.me { // exclude leader
            rf.nextIndex[i] = len(rf.logs) + 1 
        }
    }

    for i := range rf.matchIndex {
        rf.matchIndex[i] = 0
    }

	rf.readPersist(persister.ReadRaftState())
	go rf.startElectionTimer()
	go rf.commitListenAndSendToSm()

	return rf
}

func (rf *Raft) startElectionTimer() {
	for {
		rf.mu.Lock()
		timeout := rf.electionTimeout
		rf.mu.Unlock()
		electionTimer := time.After(timeout)

		select {
		case <-electionTimer: // timeout case
			rf.mu.Lock()
			if rf.state != Leader {
				rf.state = Candidate
				rf.startElection()
			}
			rf.mu.Unlock()

		case <-rf.heartbeatReceived: // reset the timer
			continue
		}
	}
}

func (rf *Raft) startElection() {
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.votesReceived = 1
	rf.persist()

	for i := range rf.peers {
		if i != rf.me {
			go rf.initiateRequest(i)
		}
	}
}

func (rf *Raft) initiateRequest(server int) {
	rf.mu.Lock()
	
	lastLogIndexC := len(rf.logs) - 1  

	// fmt.Printf("LastLogIndex at initiate request: %d \n", lastLogIndexC)

	var lastLogTermC int
	if lastLogIndexC == -1 {
		lastLogTermC = -1
	} else {
		lastLogTermC = rf.logs[lastLogIndexC].Term
	}

	args := RequestVoteArgs{
		Term:        rf.currentTerm,
		CandidateId: rf.me,
		LastLogIndex: lastLogIndexC,
		LastLogTerm:  lastLogTermC,
		
	}

	reply := RequestVoteReply{}
	rf.mu.Unlock()
	if rf.sendRequestVote(server, args, &reply) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.votedFor = -1
			rf.state = Follower
			rf.persist()

			return
		}

		if reply.VoteGranted {
			rf.votesReceived++
			threshold := (len(rf.peers) + 1) / 2
			if rf.votesReceived > threshold {
				if rf.state != Leader {
					rf.state = Leader
					go rf.sendHeartbeats()
				}
			}
		}
	}
}

func (rf *Raft) sendHeartbeats() {
	for {
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			break
		}
		rf.mu.Unlock()

		for i := range rf.peers {
			if i != rf.me {
				go rf.sendTo(i)
			}
		}

		time.Sleep(100 * time.Millisecond) // heartbeat interval = 100ms
	}
}

func (rf *Raft) sendTo(server int) {
	rf.mu.Lock()
	term := rf.currentTerm
	leaderId := rf.me
    leaderCommit := rf.commitIndex // The leader's commit index  ????? meaning till here log has been notified to state machine
	entries := rf.logs // add any new log entries that need to be replicated ?? or entire log ??
	rf.numValidResponses = 1
	rf.mu.Unlock()

	args := AppendEntriesArgs{
		Term:      term,
		LeaderId:  leaderId,
		Entries:      entries,
		LeaderCommit: leaderCommit,

	}

	reply := AppendEntriesReply{}
	if rf.sendAppendEntries(server, args, &reply) {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		// in case leader had a lower term it would transition to follower
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.state = Follower
			rf.votedFor = -1
			rf.persist()
			return
		}


		if rf.state == Leader && term == reply.Term {

			if reply.Success {
					rf.numValidResponses++
					threshold := (len(rf.peers) + 1) / 2
					if rf.numValidResponses > threshold {
						if len(entries) != 0 && entries[len(entries)-1].Term == rf.currentTerm {
							rf.commitIndex = len(entries)
							rf.newCommitReadyChan <- struct{}{}
						}  
					}
					
				}
			
			}
		}
}
    


func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm

	if args.Term < rf.currentTerm {
		reply.Success = false
		rf.persist()

		return
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = -1
		rf.state = Follower
	}


	// // case 3: check if log is consistent -- log inconsistancy check slides for complete logic
	// if args.PrevLogIndex >= 0 && (args.PrevLogIndex >= len(rf.logs) ||
	// 	rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm) {
	// 	reply.Success = false
	// 	reply.Term = rf.currentTerm
	// 	return
	// }


	// case 4: append entries
	rf.logs = args.Entries

	// fmt.Printf("logs ats follower %d \n", rf.logs)

	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = min(args.LeaderCommit, len(rf.logs))
		rf.newCommitReadyChan <- struct{}{}
	}

	rf.persist()

	reply.Success = true
	rf.heartbeatReceived <- struct{}{}

}


func (rf *Raft) commitListenAndSendToSm() {
    for range rf.newCommitReadyChan {
        rf.mu.Lock()
		for ; rf.commitIndex > rf.lastApplied ; {
			rf.applyCh <- ApplyMsg{
                Command: rf.logs[rf.lastApplied].Command,
                Index:   rf.lastApplied + 1 ,
            }
			rf.lastApplied++
		}
        
        rf.mu.Unlock()

    }
    // fmt.Println("commitListenAndSendToSm done")
}
