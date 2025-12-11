package consensus

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/witnz/witnz/internal/storage"
)

func TestFSMApplyHashChain(t *testing.T) {
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

	fsm := NewFSM(store)

	entry := &LogEntry{
		Type:      LogEntryHashChain,
		TableName: "test_table",
		Data: map[string]interface{}{
			"sequence_num":   float64(1),
			"hash":           "test_hash",
			"previous_hash":  "genesis",
			"operation_type": "INSERT",
			"record_id":      "1",
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	log := &raft.Log{
		Data: data,
	}

	result := fsm.Apply(log)
	if result != nil {
		t.Errorf("Apply failed: %v", result)
	}

	retrieved, err := store.GetHashEntry("test_table", 1)
	if err != nil {
		t.Fatalf("GetHashEntry failed: %v", err)
	}

	if retrieved.Hash != "test_hash" {
		t.Errorf("Expected hash test_hash, got %s", retrieved.Hash)
	}
}

func TestFSMApplyMerkleRoot(t *testing.T) {
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

	fsm := NewFSM(store)

	entry := &LogEntry{
		Type:      LogEntryMerkleRoot,
		TableName: "test_table",
		Data: map[string]interface{}{
			"root":         "merkle_root_hash",
			"record_count": float64(100),
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	log := &raft.Log{
		Data: data,
	}

	result := fsm.Apply(log)
	if result != nil {
		t.Errorf("Apply failed: %v", result)
	}

	retrieved, err := store.GetMerkleRoot("test_table")
	if err != nil {
		t.Fatalf("GetMerkleRoot failed: %v", err)
	}

	if retrieved.Root != "merkle_root_hash" {
		t.Errorf("Expected root merkle_root_hash, got %s", retrieved.Root)
	}
}

func TestFSMSnapshot(t *testing.T) {
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

	fsm := NewFSM(store)

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Error("Snapshot should not be nil")
	}
}
