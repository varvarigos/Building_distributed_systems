package raft

import (
	"math/rand"
	"testing"
	"time"
)

func TestHeartbeatMechanism(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Heartbeat Mechanism")

	cfg.checkOneLeader()

	time.Sleep(2 * RaftElectionTimeout)
	term1 := cfg.checkTerms()
	time.Sleep(2 * RaftElectionTimeout)
	term2 := cfg.checkTerms()

	if term1 != term2 {
		t.Fatalf("Term changed even though there were no failures")
	}

	cfg.end()
}

func TestElectionTimeoutRandomization(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Election Timeout Randomization")

	leader := cfg.checkOneLeader()
	cfg.disconnect(leader)

	cfg.checkOneLeader()

	cfg.disconnect((leader + 1) % servers)
	cfg.disconnect((leader + 2) % servers)
	time.Sleep(2 * RaftElectionTimeout)
	cfg.checkNoLeader()

	cfg.connect((leader + 1) % servers)
	cfg.connect((leader + 2) % servers)
	cfg.checkOneLeader()

	cfg.end()
}

func TestRapidLeaderDisconnectRejoin(t *testing.T) {
	servers := 5
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Rapid Leader Disconnect/Rejoin Cycle")

	for i := 0; i < 10; i++ {
		leader := cfg.checkOneLeader()
		cfg.disconnect(leader)
		time.Sleep(RaftElectionTimeout / 2)

		cfg.checkOneLeader()
		cfg.connect(leader)
		time.Sleep(RaftElectionTimeout / 2)
	}

	cfg.checkOneLeader()
	cfg.end()
}

func TestLeadershipTransferOnCrash(t *testing.T) {
	servers := 5
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Leadership Transfer on Crash")

	for i := 0; i < 3; i++ {
		leader := cfg.checkOneLeader()
		cfg.crash1(leader)
		cfg.one(rand.Intn(1000), servers-1, true)

		cfg.start1(leader, cfg.applier)
		cfg.connect(leader)
	}

	cfg.checkOneLeader()
	cfg.end()
}

func TestDelayedCommitWithReconnections(t *testing.T) {
	servers := 5
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Delayed Commit with Reconnections")

	for i := 0; i < 5; i++ {
		leader := cfg.checkOneLeader()
		cfg.disconnect(leader)

		for j := 0; j < 5; j++ {
			cfg.one(rand.Intn(1000), servers-1, false)
			time.Sleep(RaftElectionTimeout / 5)
		}

		cfg.connect(leader)
		cfg.one(rand.Intn(1000), servers, true)
	}

	cfg.checkOneLeader()
	cfg.end()
}

func TestRapidLeaderChurn(t *testing.T) {
	servers := 5
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Rapid Leader Churn")

	for i := 0; i < 20; i++ {
		leader := cfg.checkOneLeader()
		cfg.disconnect(leader)
		time.Sleep(100 * time.Millisecond)
		cfg.checkOneLeader()
		cfg.connect(leader)
	}

	cfg.end()
}

func TestRecoveryAfterAllCrash(t *testing.T) {
	servers := 3
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Full System Crash and Recovery")

	cfg.checkOneLeader()
	for i := 0; i < servers; i++ {
		cfg.crash1(i)
	}

	time.Sleep(500 * time.Millisecond)

	for i := 0; i < servers; i++ {
		cfg.start1(i, cfg.applier)
		cfg.connect(i)
	}

	cfg.checkOneLeader()
	cfg.end()
}

func TestDelayedRPC(t *testing.T) {
	servers := 5
	cfg := make_config(t, servers, false, false)
	defer cfg.cleanup()

	cfg.begin("Test: Delayed RPCs")

	for i := 0; i < servers; i++ {
		cfg.setlongreordering(true)
	}

	cfg.checkOneLeader()
	cfg.one(rand.Int(), servers, true)
	cfg.one(rand.Int(), servers, true)

	cfg.setlongreordering(false)
	cfg.one(rand.Int(), servers, true)

	cfg.end()
}
