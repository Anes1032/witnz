package consensus

import (
	"os"
	"testing"

	"github.com/witnz/witnz/internal/storage"
)

func TestNewNode(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7000",
		DataDir:  t.TempDir(),
	}

	node, err := NewNode(cfg, store)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	if node.config.NodeID != "test-node" {
		t.Errorf("Expected NodeID test-node, got %s", node.config.NodeID)
	}
}

func TestNodeStatsBeforeStart(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7001",
		DataDir:  t.TempDir(),
	}

	node, _ := NewNode(cfg, store)

	stats := node.Stats()
	if stats["state"] != "not initialized" {
		t.Errorf("Expected state 'not initialized', got %s", stats["state"])
	}
}

func TestNodeIsLeaderBeforeStart(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7002",
		DataDir:  t.TempDir(),
	}

	node, _ := NewNode(cfg, store)

	if node.IsLeader() {
		t.Error("Node should not be leader before start")
	}
}

func TestNodeLeaderBeforeStart(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7003",
		DataDir:  t.TempDir(),
	}

	node, _ := NewNode(cfg, store)

	leader := node.Leader()
	if leader != "" {
		t.Errorf("Expected empty leader, got %s", leader)
	}
}

func TestNodeAddPeerBeforeStart(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7004",
		DataDir:  t.TempDir(),
	}

	node, _ := NewNode(cfg, store)

	err = node.AddPeer("peer1", "127.0.0.1:7005")
	if err == nil {
		t.Error("AddPeer should fail before raft is initialized")
	}
}

func TestNodeRemovePeerBeforeStart(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7006",
		DataDir:  t.TempDir(),
	}

	node, _ := NewNode(cfg, store)

	err = node.RemovePeer("peer1")
	if err == nil {
		t.Error("RemovePeer should fail before raft is initialized")
	}
}

func TestNodeStopBeforeStart(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-consensus-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	cfg := &NodeConfig{
		NodeID:   "test-node",
		BindAddr: "127.0.0.1:7007",
		DataDir:  t.TempDir(),
	}

	node, _ := NewNode(cfg, store)

	err = node.Stop()
	if err != nil {
		t.Errorf("Stop should not fail: %v", err)
	}
}
