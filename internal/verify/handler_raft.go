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
		return NewTamperingError(event.TableName, string(event.Operation), "append-only")
	}

	if h.raftNode == nil {
		return h.HashChainHandler.HandleChange(event)
	}

	if !h.raftNode.IsLeader() {
		slog.Debug("Ignoring CDC event on follower (will receive via Raft)",
			"table", event.TableName,
			"operation", event.Operation)
		return nil
	}

	chain, ok := h.hashChains[event.TableName]
	if !ok {
		return fmt.Errorf("no hash chain for table: %s", event.TableName)
	}

	previousHash := chain.GetPreviousHash()
	dataHash := calculateDataHash(event.NewData)
	newHash, err := chain.Add(event.NewData)
	if err != nil {
		return fmt.Errorf("failed to add to hash chain: %w", err)
	}

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
			"hash":           newHash,
			"previous_hash":  previousHash,
			"data_hash":      dataHash,
			"operation_type": string(event.Operation),
			"record_id":      fmt.Sprintf("%v", event.PrimaryKey),
		},
		Timestamp: time.Now(),
	}

	if err := h.raftNode.ApplyLog(logEntry); err != nil {
		return fmt.Errorf("failed to replicate via raft: %w", err)
	}

	slog.Info("Raft consensus: hash chain entry replicated",
		"table", event.TableName,
		"seq", seqNum)
	return nil
}
