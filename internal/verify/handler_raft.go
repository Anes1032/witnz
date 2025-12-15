package verify

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/witnz/witnz/internal/cdc"
	"github.com/witnz/witnz/internal/consensus"
)

type RaftHashChainHandler struct {
	*HashChainHandler
	raftNode *consensus.Node
}

func NewRaftHashChainHandler(handler *HashChainHandler, raftNode *consensus.Node) *RaftHashChainHandler {
	return &RaftHashChainHandler{
		HashChainHandler: handler,
		raftNode:         raftNode,
	}
}

func (h *RaftHashChainHandler) HandleChange(event *cdc.ChangeEvent) error {
	_, ok := h.tableConfigs[event.TableName]
	if !ok {
		return nil
	}

	if event.Operation == cdc.OperationUpdate || event.Operation == cdc.OperationDelete {
		return NewTamperingError(event.TableName, string(event.Operation))
	}

	if h.raftNode == nil {
		return h.HashChainHandler.HandleChange(event)
	}

	if _, ok := h.tableConfigs[event.TableName]; !ok {
		return fmt.Errorf("table not configured: %s", event.TableName)
	}

	dataHash := calculateDataHash(event.NewData)

	latestEntry, _ := h.storage.GetLatestHashEntry(event.TableName)
	var seqNum uint64 = 1
	if latestEntry != nil {
		seqNum = latestEntry.SequenceNum + 1
	}

	logEntry := &consensus.LogEntry{
		Type:      consensus.LogEntryHashChain,
		TableName: event.TableName,
		Data: map[string]interface{}{
			"sequence_num":   float64(seqNum),
			"data_hash":      dataHash,
			"operation_type": string(event.Operation),
			"record_id":      fmt.Sprintf("%v", event.PrimaryKey),
		},
		Timestamp: time.Now(),
	}

	// Only the leader can apply logs to Raft
	// Followers will receive "not the leader" error and skip replication
	if err := h.raftNode.ApplyLog(logEntry); err != nil {
		if !h.raftNode.IsLeader() {
			// Follower: hash is calculated but not replicated (will receive via Raft)
			slog.Debug("CDC event processed on follower, waiting for Raft replication",
				"table", event.TableName,
				"seq", seqNum)
			return nil
		}
		return fmt.Errorf("failed to replicate via raft: %w", err)
	}

	slog.Info("Raft consensus: hash chain entry replicated",
		"table", event.TableName,
		"seq", seqNum)
	return nil
}
