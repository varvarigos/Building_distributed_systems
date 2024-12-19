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

	"bytes"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	//	"6.824/labgob"

	"6.824/labgob"
	"6.824/labrpc"
)

const HeartbeatTimeout = 100 * time.Millisecond

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
	CommandTerm  int
}
type LogEntry struct {
	Term    int
	Command interface{}
	Index   int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu               sync.Mutex          // Lock to protect shared access to this peer's state
	peers            []*labrpc.ClientEnd // RPC end points of all peers
	persister        *Persister          // Object to hold this peer's persisted state
	me               int                 // this peer's index into peers[]
	dead             int32               // set by Kill()
	currentTerm      int
	votedFor         int
	log              []LogEntry
	state            string
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration
	heartbeatTime    *time.Timer
	electionTime     *time.Timer
	applyCh          chan ApplyMsg
	commitIndex      int
	nextIndex        []int
	matchIndex       []int
	lastApplied      int
	hasCommittedChan chan struct{}
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

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) respAppendEntries(server int, expectedNextIndex int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state == "Leader" && args.Term == rf.currentTerm {
		if reply.Term > rf.currentTerm {
			rf.persist()
			rf.state = "Follower"
			rf.votedFor = -1
			rf.currentTerm = reply.Term
			rf.electionTime.Reset(rf.electionTimeout)

			return
		}

		if reply.Success {
			if rf.nextIndex[server]+len(args.Entries) == expectedNextIndex {
				rf.nextIndex[server] = expectedNextIndex
				rf.matchIndex[server] = rf.nextIndex[server] - 1
			}
			numAgreesToCommit := 1
			for i := range rf.peers {
				if i != rf.me && rf.matchIndex[i] >= rf.matchIndex[server] {
					numAgreesToCommit++
				}
			}
			majorityThreshold := len(rf.peers) / 2
			isLogEntryInCurrentTerm := rf.log[rf.matchIndex[server]-rf.log[0].Index].Term == rf.currentTerm
			if numAgreesToCommit > majorityThreshold && isLogEntryInCurrentTerm {
				rf.commitIndex = max(rf.commitIndex, rf.matchIndex[server])
				rf.hasCommittedChan <- struct{}{}
			}

			return
		}

		rf.nextIndex[server] = 1

		go rf.handleAppendEntries(
			server,
			rf.nextIndex[server]+len(rf.log[rf.nextIndex[server]-rf.log[0].Index:]),
			&AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: rf.nextIndex[server] - 1,
				PrevLogTerm:  rf.log[rf.nextIndex[server]-1-rf.log[0].Index].Term,
				Entries:      rf.log[rf.nextIndex[server]-rf.log[0].Index:],
				LeaderCommit: rf.commitIndex,
			},
			&AppendEntriesReply{
				Term:    0,
				Success: false,
			},
		)
	}
}

func (rf *Raft) handleAppendEntries(server int, expectedNextIndex int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	defer rf.persist()
	if rf.state == "Leader" && args.Term == rf.currentTerm {
		rf.mu.Unlock()
		defer rf.mu.Lock()

		ok := rf.sendAppendEntries(server, args, reply)
		if !ok {
			return
		}
		rf.respAppendEntries(server, expectedNextIndex, args, reply)
	}
}

func (rf *Raft) sendHeartbeats() {
	for i := 0; i < len(rf.peers); i++ {
		if i != rf.me {
			prevLogIndex := rf.nextIndex[i] - 1
			prevLogTerm := rf.log[prevLogIndex-rf.log[0].Index].Term
			entries := rf.log[rf.nextIndex[i]-rf.log[0].Index:]
			leaderCommit := rf.commitIndex

			go rf.handleAppendEntries(
				i,
				rf.nextIndex[i]+len(entries),
				&AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm:  prevLogTerm,
					Entries:      entries,
					LeaderCommit: leaderCommit,
				},
				&AppendEntriesReply{
					Term:    0,
					Success: false,
				},
			)
		}
	}
}

func (rf *Raft) AppendEntries(request *AppendEntriesArgs, response *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	defer rf.persist()
	if request.Term < rf.currentTerm {
		response.Success = false
		response.Term = rf.currentTerm
	} else {
		if request.Term > rf.currentTerm {
			rf.votedFor = -1
			rf.currentTerm = request.Term
		}
		rf.state = "Follower"
		rf.electionTime.Reset(rf.electionTimeout)

		if request.PrevLogIndex < rf.log[0].Index-1 {
			response.Success = false
			response.Term = 0
		} else if (request.PrevLogIndex > rf.lastLogIndex()) ||
			(rf.log[request.PrevLogIndex-rf.log[0].Index].Term != request.PrevLogTerm) {
			response.Success = false
			response.Term = rf.currentTerm
		} else {
			for i, newEntry := range request.Entries {
				indexOutOfBounds := newEntry.Index-rf.log[0].Index >= len(rf.log)
				if indexOutOfBounds || rf.log[newEntry.Index-rf.log[0].Index].Term != newEntry.Term {
					newLog := make([]LogEntry, len(rf.log[:newEntry.Index-rf.log[0].Index])+len(request.Entries[i:]))
					copy(newLog, rf.log[:newEntry.Index-rf.log[0].Index])
					copy(newLog[len(rf.log[:newEntry.Index-rf.log[0].Index]):], request.Entries[i:])
					rf.log = newLog
					break
				}
			}
			if request.LeaderCommit > rf.commitIndex {
				if request.LeaderCommit < rf.lastLogIndex() {
					rf.commitIndex = request.LeaderCommit
				} else {
					rf.commitIndex = rf.lastLogIndex()
				}
				rf.hasCommittedChan <- struct{}{}
			}
			response.Term = rf.currentTerm
			response.Success = true
		}
	}
}

func (rf *Raft) applyEntries() {
	for rf.killed() == false {
		var entriesToApply []LogEntry
		var newCommitIndex int

		rf.mu.Lock()
		if rf.lastApplied < rf.commitIndex {
			newCommitIndex = rf.commitIndex
			startIndex := rf.lastApplied + 1 - rf.log[0].Index
			endIndex := rf.commitIndex + 1 - rf.log[0].Index
			entriesToApply = append([]LogEntry{}, rf.log[startIndex:endIndex]...)
			rf.mu.Unlock()
		} else {
			rf.mu.Unlock()
			<-rf.hasCommittedChan
			continue
		}

		if len(entriesToApply) != 0 {
			rf.lastApplied = newCommitIndex
			for _, entry := range entriesToApply {
				rf.applyCh <- ApplyMsg{
					CommandValid: true,
					Command:      entry.Command,
					CommandIndex: entry.Index,
					CommandTerm:  entry.Term,
				}
			}
		}
	}
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	var term int
	var isleader bool

	term = rf.currentTerm
	isleader = rf.state == "Leader"

	return term, isleader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (3C).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var log []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&log) != nil {
		panic("Error decoding persisted state")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = log
	}
}

// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (3A, 3B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (3A).
	Term        int
	VoteGranted bool
}

func (rf *Raft) lastLogTerm() int {
	if len(rf.log) == 0 {
		return -1
	}
	return rf.log[len(rf.log)-1].Term
}

func (rf *Raft) lastLogIndex() int {
	return rf.log[len(rf.log)-1].Index
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()

	reply.Term = rf.currentTerm
	shouldGrantVote := false

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = "Follower"
		rf.votedFor = -1
	}

	if (args.LastLogTerm > rf.lastLogTerm() || (args.LastLogTerm == rf.lastLogTerm() && rf.lastLogIndex() <= args.LastLogIndex)) &&
		(rf.votedFor == -1 || rf.votedFor == args.CandidateId) {
		shouldGrantVote = true
		rf.votedFor = args.CandidateId
		rf.electionTime.Reset(rf.electionTimeout)
	}

	rf.mu.Unlock()

	reply.VoteGranted = shouldGrantVote
}

func (rf *Raft) conductElection(request *RequestVoteArgs) {
	totalVotes := 1

	for i := range rf.peers {
		if i != rf.me {
			go func(peer int) {
				response := &RequestVoteReply{
					Term:        0,
					VoteGranted: false,
				}

				if !rf.sendRequestVote(peer, request, response) {
					return
				}

				rf.mu.Lock()
				defer rf.mu.Unlock()

				if rf.state == "Candidate" && rf.currentTerm == request.Term {
					if response.Term > rf.currentTerm {
						rf.persist()
						rf.state = "Follower"
						rf.currentTerm = response.Term
						rf.votedFor = -1
						rf.electionTime.Reset(rf.electionTimeout)
					} else if response.VoteGranted {
						totalVotes++
						if totalVotes > len(rf.peers)/2 {
							for idx := 0; idx < len(rf.nextIndex); idx++ {
								rf.nextIndex[idx] = rf.commitIndex + 1
							}
							rf.state = "Leader"
							rf.sendHeartbeats()
							rf.heartbeatTime.Reset(rf.heartbeatTimeout)
						}
					}
				}
			}(i)
		}
	}
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

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state == "Leader" {
		rf.persist()
		term = rf.currentTerm
		index = rf.lastLogIndex() + 1
		rf.log = append(rf.log, LogEntry{
			Term:    term,
			Command: command,
			Index:   index,
		})
		rf.sendHeartbeats()
	} else {
		isLeader = false
	}

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

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	for rf.killed() == false {
		select {
		case <-rf.heartbeatTime.C:
			rf.mu.Lock()
			if rf.state == "Leader" {
				rf.sendHeartbeats()
				rf.heartbeatTime.Reset(HeartbeatTimeout)
			}
			rf.mu.Unlock()
		case <-rf.electionTime.C:
			rf.mu.Lock()
			if rf.state != "Leader" {
				rf.persist()
				rf.currentTerm++
				rf.state = "Candidate"
				rf.votedFor = rf.me
				rf.conductElection(&RequestVoteArgs{
					Term:         rf.currentTerm,
					CandidateId:  rf.votedFor,
					LastLogIndex: rf.lastLogIndex(),
					LastLogTerm:  rf.lastLogTerm(),
				})
				rf.electionTime.Reset(rf.electionTimeout)
			}
			rf.mu.Unlock()
		}
	}
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
	rf.dead = 0
	rf.votedFor = -1
	rf.state = "Follower"
	rf.electionTimeout = time.Duration(rand.Intn(150)+150) * time.Millisecond
	rf.heartbeatTimeout = HeartbeatTimeout
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.log = make([]LogEntry, 1)
	rf.log[0] = LogEntry{Command: nil, Term: 0}
	rf.applyCh = applyCh
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))
	rf.heartbeatTime = time.NewTimer(HeartbeatTimeout)
	rf.electionTime = time.NewTimer(rf.electionTimeout)
	rf.hasCommittedChan = make(chan struct{}, 1000)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	go rf.applyEntries()

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
