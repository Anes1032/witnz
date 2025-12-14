package consensus

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	case LogEntryCheckpoint:
		return f.applyCheckpoint(&entry)
	default:
		return fmt.Errorf("unknown log entry type: %s", entry.Type)
	}
}

func (f *FSM) applyHashChain(entry *LogEntry) interface{} {
	dataHash := ""
	if dh, ok := entry.Data["data_hash"].(string); ok {
		dataHash = dh
	}

	if dataHash == "" {
		slog.Warn("Received hash entry with empty data_hash",
			"table", entry.TableName,
			"sequence_num", entry.Data["sequence_num"])
	}

	hashEntry := &storage.HashEntry{
		TableName:     entry.TableName,
		SequenceNum:   uint64(entry.Data["sequence_num"].(float64)),
		DataHash:      dataHash,
		Timestamp:     entry.Timestamp,
		OperationType: entry.Data["operation_type"].(string),
		RecordID:      entry.Data["record_id"].(string),
	}

	if err := f.storage.SaveHashEntry(hashEntry); err != nil {
		return err
	}

	return nil
}

func (f *FSM) applyCheckpoint(entry *LogEntry) interface{} {
	checkpoint := &storage.MerkleCheckpoint{
		TableName:     entry.TableName,
		SequenceNum:   uint64(entry.Data["sequence_num"].(float64)),
		MerkleRoot:    entry.Data["merkle_root"].(string),
		Timestamp:     entry.Timestamp,
		RecordCount:   int(entry.Data["record_count"].(float64)),
		HashAlgorithm: entry.Data["hash_algorithm"].(string),
	}

	// Decode leaf_map and internal_nodes if present
	if leafMapData, ok := entry.Data["leaf_map"].(map[string]interface{}); ok {
		checkpoint.LeafMap = make(map[string]string)
		for k, v := range leafMapData {
			if strVal, ok := v.(string); ok {
				checkpoint.LeafMap[k] = strVal
			}
		}
	}

	if internalNodesData, ok := entry.Data["internal_nodes"].(map[string]interface{}); ok {
		checkpoint.InternalNodes = make(map[string]string)
		for k, v := range internalNodesData {
			if strVal, ok := v.(string); ok {
				checkpoint.InternalNodes[k] = strVal
			}
		}
	}

	if err := f.storage.SaveMerkleCheckpoint(checkpoint); err != nil {
		slog.Error("Failed to save checkpoint via Raft",
			"table", entry.TableName,
			"error", err)
		return err
	}

	slog.Info("Applied checkpoint from Raft",
		"table", entry.TableName,
		"sequence_num", checkpoint.SequenceNum,
		"record_count", checkpoint.RecordCount)

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
		HashEntries []storage.HashEntry `json:"hash_entries"`
	}

	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}

	for _, entry := range snapshot.HashEntries {
		if err := f.storage.SaveHashEntry(&entry); err != nil {
			return fmt.Errorf("failed to restore hash entry: %w", err)
		}
	}

	return nil
}

type fsmSnapshot struct {
	storage *storage.Storage
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	entries, err := s.storage.GetAllHashEntriesAllTables()
	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to get hash entries for snapshot: %w", err)
	}

	snapshot := struct {
		HashEntries []storage.HashEntry `json:"hash_entries"`
	}{
		HashEntries: entries,
	}

	encoder := json.NewEncoder(sink)
	if err := encoder.Encode(snapshot); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := sink.Close(); err != nil {
		return fmt.Errorf("failed to close snapshot sink: %w", err)
	}

	return nil
}

func (s *fsmSnapshot) Release() {
}
