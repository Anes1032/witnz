package consensus

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/witnz/witnz/internal/storage"
)

func TestThreeNodeCluster(t *testing.T) {
	node1Dir := t.TempDir()
	node2Dir := t.TempDir()
	node3Dir := t.TempDir()

	db1, err := os.CreateTemp("", "witnz-cluster-test-node1-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()
	defer os.Remove(db1.Name())

	db2, err := os.CreateTemp("", "witnz-cluster-test-node2-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
	defer os.Remove(db2.Name())

	db3, err := os.CreateTemp("", "witnz-cluster-test-node3-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db3.Close()
	defer os.Remove(db3.Name())

	store1, err := storage.New(db1.Name())
	if err != nil {
		t.Fatalf("Failed to create storage for node1: %v", err)
	}
	defer store1.Close()

	store2, err := storage.New(db2.Name())
	if err != nil {
		t.Fatalf("Failed to create storage for node2: %v", err)
	}
	defer store2.Close()

	store3, err := storage.New(db3.Name())
	if err != nil {
		t.Fatalf("Failed to create storage for node3: %v", err)
	}
	defer store3.Close()

	node1Cfg := &NodeConfig{
		NodeID:    "node1",
		BindAddr:  "127.0.0.1:17001",
		DataDir:   node1Dir,
		Bootstrap: true,
		PeerAddrs: map[string]string{
			"node2": "127.0.0.1:17002",
			"node3": "127.0.0.1:17003",
		},
	}

	node2Cfg := &NodeConfig{
		NodeID:    "node2",
		BindAddr:  "127.0.0.1:17002",
		DataDir:   node2Dir,
		Bootstrap: false,
		PeerAddrs: map[string]string{
			"node1": "127.0.0.1:17001",
			"node3": "127.0.0.1:17003",
		},
	}

	node3Cfg := &NodeConfig{
		NodeID:    "node3",
		BindAddr:  "127.0.0.1:17003",
		DataDir:   node3Dir,
		Bootstrap: false,
		PeerAddrs: map[string]string{
			"node1": "127.0.0.1:17001",
			"node2": "127.0.0.1:17002",
		},
	}

	node1, err := NewNode(node1Cfg, store1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}

	node2, err := NewNode(node2Cfg, store2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}

	node3, err := NewNode(node3Cfg, store3)
	if err != nil {
		t.Fatalf("Failed to create node3: %v", err)
	}

	ctx := context.Background()

	if err := node1.Start(ctx); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	time.Sleep(2 * time.Second)

	if err := node2.Start(ctx); err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Stop()

	if err := node3.Start(ctx); err != nil {
		t.Fatalf("Failed to start node3: %v", err)
	}
	defer node3.Stop()

	time.Sleep(5 * time.Second)

	leader1 := node1.Leader()
	leader2 := node2.Leader()
	leader3 := node3.Leader()

	if leader1 == "" {
		t.Error("Node1 has no leader")
	}

	if leader1 != leader2 {
		t.Errorf("Leader mismatch: node1=%s, node2=%s", leader1, leader2)
	}

	if leader1 != leader3 {
		t.Errorf("Leader mismatch: node1=%s, node3=%s", leader1, leader3)
	}

	var leaderNode *Node
	if node1.IsLeader() {
		leaderNode = node1
	} else if node2.IsLeader() {
		leaderNode = node2
	} else if node3.IsLeader() {
		leaderNode = node3
	}

	if leaderNode == nil {
		t.Fatal("No leader node found")
	}

	entry := &LogEntry{
		Type:      LogEntryHashChain,
		TableName: "test_table",
		Data: map[string]interface{}{
			"sequence_num":   float64(1),
			"hash":           "test_hash_123",
			"previous_hash":  "genesis",
			"operation_type": "INSERT",
			"record_id":      "1",
		},
		Timestamp: time.Now(),
	}

	if err := leaderNode.ApplyLog(entry); err != nil {
		t.Fatalf("Failed to apply log: %v", err)
	}

	time.Sleep(2 * time.Second)

	hashEntry1, err := store1.GetLatestHashEntry("test_table")
	if err != nil {
		t.Fatalf("Failed to get hash entry from node1: %v", err)
	}

	hashEntry2, err := store2.GetLatestHashEntry("test_table")
	if err != nil {
		t.Fatalf("Failed to get hash entry from node2: %v", err)
	}

	hashEntry3, err := store3.GetLatestHashEntry("test_table")
	if err != nil {
		t.Fatalf("Failed to get hash entry from node3: %v", err)
	}

	if hashEntry1.DataHash != "test_hash_123" {
		t.Errorf("Node1 data hash mismatch: got %s, want test_hash_123", hashEntry1.DataHash)
	}

	if hashEntry2.DataHash != "test_hash_123" {
		t.Errorf("Node2 data hash mismatch: got %s, want test_hash_123", hashEntry2.DataHash)
	}

	if hashEntry3.DataHash != "test_hash_123" {
		t.Errorf("Node3 data hash mismatch: got %s, want test_hash_123", hashEntry3.DataHash)
	}

	if hashEntry1.SequenceNum != 1 || hashEntry2.SequenceNum != 1 || hashEntry3.SequenceNum != 1 {
		t.Error("Sequence numbers do not match across all nodes")
	}
}

func TestClusterLeaderElection(t *testing.T) {
	node1Dir := t.TempDir()
	node2Dir := t.TempDir()
	node3Dir := t.TempDir()

	db1, err := os.CreateTemp("", "witnz-leader-test-node1-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()
	defer os.Remove(db1.Name())

	db2, err := os.CreateTemp("", "witnz-leader-test-node2-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
	defer os.Remove(db2.Name())

	db3, err := os.CreateTemp("", "witnz-leader-test-node3-*.db")
	if err != nil {
		t.Fatal(err)
	}
	db3.Close()
	defer os.Remove(db3.Name())

	store1, err := storage.New(db1.Name())
	if err != nil {
		t.Fatalf("Failed to create storage for node1: %v", err)
	}
	defer store1.Close()

	store2, err := storage.New(db2.Name())
	if err != nil {
		t.Fatalf("Failed to create storage for node2: %v", err)
	}
	defer store2.Close()

	store3, err := storage.New(db3.Name())
	if err != nil {
		t.Fatalf("Failed to create storage for node3: %v", err)
	}
	defer store3.Close()

	node1Cfg := &NodeConfig{
		NodeID:    "node1",
		BindAddr:  "127.0.0.1:18001",
		DataDir:   node1Dir,
		Bootstrap: true,
		PeerAddrs: map[string]string{
			"node2": "127.0.0.1:18002",
			"node3": "127.0.0.1:18003",
		},
	}

	node2Cfg := &NodeConfig{
		NodeID:    "node2",
		BindAddr:  "127.0.0.1:18002",
		DataDir:   node2Dir,
		Bootstrap: false,
		PeerAddrs: map[string]string{
			"node1": "127.0.0.1:18001",
			"node3": "127.0.0.1:18003",
		},
	}

	node3Cfg := &NodeConfig{
		NodeID:    "node3",
		BindAddr:  "127.0.0.1:18003",
		DataDir:   node3Dir,
		Bootstrap: false,
		PeerAddrs: map[string]string{
			"node1": "127.0.0.1:18001",
			"node2": "127.0.0.1:18002",
		},
	}

	node1, err := NewNode(node1Cfg, store1)
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}

	node2, err := NewNode(node2Cfg, store2)
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}

	node3, err := NewNode(node3Cfg, store3)
	if err != nil {
		t.Fatalf("Failed to create node3: %v", err)
	}

	ctx := context.Background()

	if err := node1.Start(ctx); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	time.Sleep(2 * time.Second)

	if err := node2.Start(ctx); err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Stop()

	if err := node3.Start(ctx); err != nil {
		t.Fatalf("Failed to start node3: %v", err)
	}
	defer node3.Stop()

	time.Sleep(5 * time.Second)

	leaderCount := 0
	if node1.IsLeader() {
		leaderCount++
	}
	if node2.IsLeader() {
		leaderCount++
	}
	if node3.IsLeader() {
		leaderCount++
	}

	if leaderCount != 1 {
		t.Errorf("Expected exactly 1 leader, got %d", leaderCount)
	}
}
