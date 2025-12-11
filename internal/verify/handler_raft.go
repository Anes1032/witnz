package verify

import (
	"fmt"
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
	config, ok := h.tableConfigs[event.TableName]
	if !ok {
		return nil
	}

	if config.Mode != AppendOnlyMode {
		return h.HashChainHandler.HandleChange(event)
	}

	if event.Operation == cdc.OperationUpdate || event.Operation == cdc.OperationDelete {
		return fmt.Errorf("TAMPERING DETECTED: %s operation on append-only table %s",
			event.Operation, event.TableName)
	}

	chain, ok := h.hashChains[event.TableName]
	if !ok {
		return fmt.Errorf("no hash chain for table: %s", event.TableName)
	}

	previousHash := chain.GetPreviousHash()
	newHash, err := chain.Add(event.NewData)
	if err != nil {
		return fmt.Errorf("failed to add to hash chain: %w", err)
	}

	latestEntry, _ := h.storage.GetLatestHashEntry(event.TableName)
	var seqNum uint64 = 1
	if latestEntry != nil {
		seqNum = latestEntry.SequenceNum + 1
	}

	if h.raftNode != nil && h.raftNode.IsLeader() {
		logEntry := &consensus.LogEntry{
			Type:      consensus.LogEntryHashChain,
			TableName: event.TableName,
			Data: map[string]interface{}{
				"sequence_num":   float64(seqNum),
				"hash":           newHash,
				"previous_hash":  previousHash,
				"operation_type": string(event.Operation),
				"record_id":      fmt.Sprintf("%v", event.PrimaryKey),
			},
			Timestamp: time.Now(),
		}

		if err := h.raftNode.ApplyLog(logEntry); err != nil {
			return fmt.Errorf("failed to replicate via raft: %w", err)
		}

		return nil
	}

	return h.HashChainHandler.handleAppendOnly(event)
}
