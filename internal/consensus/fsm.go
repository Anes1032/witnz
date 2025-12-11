package consensus

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/witnz/witnz/internal/storage"
)

type FSM struct {
	mu      sync.RWMutex
	storage *storage.Storage
}

func NewFSM(store *storage.Storage) *FSM {
	return &FSM{
		storage: store,
	}
}

func (f *FSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	var entry LogEntry
	if err := json.Unmarshal(log.Data, &entry); err != nil {
		return fmt.Errorf("failed to unmarshal log entry: %w", err)
	}

	switch entry.Type {
	case LogEntryHashChain:
		return f.applyHashChain(&entry)
	case LogEntryMerkleRoot:
		return f.applyMerkleRoot(&entry)
	default:
		return fmt.Errorf("unknown log entry type: %s", entry.Type)
	}
}

func (f *FSM) applyHashChain(entry *LogEntry) interface{} {
	hashEntry := &storage.HashEntry{
		TableName:     entry.TableName,
		SequenceNum:   uint64(entry.Data["sequence_num"].(float64)),
		Hash:          entry.Data["hash"].(string),
		PreviousHash:  entry.Data["previous_hash"].(string),
		Timestamp:     entry.Timestamp,
		OperationType: entry.Data["operation_type"].(string),
		RecordID:      entry.Data["record_id"].(string),
	}

	if err := f.storage.SaveHashEntry(hashEntry); err != nil {
		return err
	}

	return nil
}

func (f *FSM) applyMerkleRoot(entry *LogEntry) interface{} {
	merkleEntry := &storage.MerkleRootEntry{
		TableName:   entry.TableName,
		Root:        entry.Data["root"].(string),
		Timestamp:   entry.Timestamp,
		RecordCount: int(entry.Data["record_count"].(float64)),
	}

	if err := f.storage.SaveMerkleRoot(merkleEntry); err != nil {
		return err
	}

	return nil
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &fsmSnapshot{
		storage: f.storage,
	}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer rc.Close()

	decoder := json.NewDecoder(rc)

	var snapshot struct {
		HashEntries  []storage.HashEntry       `json:"hash_entries"`
		MerkleRoots  []storage.MerkleRootEntry `json:"merkle_roots"`
	}

	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}

	for _, entry := range snapshot.HashEntries {
		if err := f.storage.SaveHashEntry(&entry); err != nil {
			return fmt.Errorf("failed to restore hash entry: %w", err)
		}
	}

	for _, entry := range snapshot.MerkleRoots {
		if err := f.storage.SaveMerkleRoot(&entry); err != nil {
			return fmt.Errorf("failed to restore merkle root: %w", err)
		}
	}

	return nil
}

type fsmSnapshot struct {
	storage *storage.Storage
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	defer sink.Close()

	snapshot := struct {
		HashEntries  []storage.HashEntry       `json:"hash_entries"`
		MerkleRoots  []storage.MerkleRootEntry `json:"merkle_roots"`
	}{
		HashEntries:  make([]storage.HashEntry, 0),
		MerkleRoots:  make([]storage.MerkleRootEntry, 0),
	}

	encoder := json.NewEncoder(sink)
	if err := encoder.Encode(snapshot); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	return nil
}

func (s *fsmSnapshot) Release() {
}
