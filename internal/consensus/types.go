package consensus

import (
	"time"
)

type LogEntryType string

const (
	LogEntryHashChain  LogEntryType = "hash_chain"
	LogEntryMerkleRoot LogEntryType = "merkle_root"
)

type LogEntry struct {
	Type      LogEntryType           `json:"type"`
	TableName string                 `json:"table_name"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

type SnapshotMeta struct {
	Index     uint64    `json:"index"`
	Term      uint64    `json:"term"`
	Timestamp time.Time `json:"timestamp"`
}
