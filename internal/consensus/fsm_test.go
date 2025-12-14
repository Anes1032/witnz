package consensus

import (
	"encoding/json"
	"io"
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
			"data_hash":      "test_hash",
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

	if retrieved.DataHash != "test_hash" {
		t.Errorf("Expected data hash test_hash, got %s", retrieved.DataHash)
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

func TestFSMSnapshotPersistAndRestore(t *testing.T) {
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

	fsm := NewFSM(store)

	testEntry := &LogEntry{
		Type:      LogEntryHashChain,
		TableName: "snapshot_test_table",
		Data: map[string]interface{}{
			"sequence_num":   float64(1),
			"data_hash":      "snapshot_test_hash",
			"operation_type": "INSERT",
			"record_id":      "100",
		},
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(testEntry)
	fsm.Apply(&raft.Log{Data: data})

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	var buf mockSnapshotSink
	if err := snapshot.Persist(&buf); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("Snapshot buffer should not be empty")
	}

	var snapshotData struct {
		HashEntries []storage.HashEntry `json:"hash_entries"`
	}
	if err := json.Unmarshal(buf.Bytes(), &snapshotData); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	if len(snapshotData.HashEntries) != 1 {
		t.Fatalf("Expected 1 hash entry in snapshot, got %d", len(snapshotData.HashEntries))
	}

	if snapshotData.HashEntries[0].DataHash != "snapshot_test_hash" {
		t.Errorf("Expected data hash 'snapshot_test_hash', got '%s'", snapshotData.HashEntries[0].DataHash)
	}

	store.Close()

	tmpfile2, _ := os.CreateTemp("", "witnz-consensus-restore-*.db")
	tmpfile2.Close()
	defer os.Remove(tmpfile2.Name())

	store2, _ := storage.New(tmpfile2.Name())
	defer store2.Close()

	fsm2 := NewFSM(store2)

	if err := fsm2.Restore(&mockReadCloser{data: buf.Bytes()}); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restored, err := store2.GetHashEntry("snapshot_test_table", 1)
	if err != nil {
		t.Fatalf("Failed to get restored entry: %v", err)
	}

	if restored.DataHash != "snapshot_test_hash" {
		t.Errorf("Expected restored data hash 'snapshot_test_hash', got '%s'", restored.DataHash)
	}
}

type mockSnapshotSink struct {
	buf      []byte
	canceled bool
}

func (m *mockSnapshotSink) Write(p []byte) (n int, err error) {
	m.buf = append(m.buf, p...)
	return len(p), nil
}

func (m *mockSnapshotSink) Close() error {
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "mock-snapshot"
}

func (m *mockSnapshotSink) Cancel() error {
	m.canceled = true
	return nil
}

func (m *mockSnapshotSink) Bytes() []byte {
	return m.buf
}

func (m *mockSnapshotSink) Len() int {
	return len(m.buf)
}

type mockReadCloser struct {
	data   []byte
	offset int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}
