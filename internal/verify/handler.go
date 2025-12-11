package verify

import (
	"fmt"
	"time"

	"github.com/witnz/witnz/internal/alert"
	"github.com/witnz/witnz/internal/cdc"
	"github.com/witnz/witnz/internal/hash"
	"github.com/witnz/witnz/internal/storage"
)

type ProtectionMode string

const (
	AppendOnlyMode     ProtectionMode = "append_only"
	StateIntegrityMode ProtectionMode = "state_integrity"
)

type TableConfig struct {
	Name           string
	Mode           ProtectionMode
	VerifyInterval string
}

type HashChainHandler struct {
	storage      *storage.Storage
	hashChains   map[string]*hash.HashChain
	tableConfigs map[string]*TableConfig
	alertManager *alert.Manager
}

func NewHashChainHandler(store *storage.Storage) *HashChainHandler {
	return &HashChainHandler{
		storage:      store,
		hashChains:   make(map[string]*hash.HashChain),
		tableConfigs: make(map[string]*TableConfig),
	}
}

func (h *HashChainHandler) SetAlertManager(am *alert.Manager) {
	h.alertManager = am
}

func (h *HashChainHandler) AddTable(config *TableConfig) error {
	h.tableConfigs[config.Name] = config

	latestEntry, err := h.storage.GetLatestHashEntry(config.Name)
	if err != nil {
		h.hashChains[config.Name] = hash.NewHashChain("genesis")
	} else {
		h.hashChains[config.Name] = hash.NewHashChain(latestEntry.Hash)
	}

	return nil
}

func (h *HashChainHandler) HandleChange(event *cdc.ChangeEvent) error {
	config, ok := h.tableConfigs[event.TableName]
	if !ok {
		return nil
	}

	switch config.Mode {
	case AppendOnlyMode:
		return h.handleAppendOnly(event)
	case StateIntegrityMode:
		return h.handleStateIntegrity(event)
	default:
		return fmt.Errorf("unknown protection mode: %s", config.Mode)
	}
}

func (h *HashChainHandler) handleAppendOnly(event *cdc.ChangeEvent) error {
	if event.Operation == cdc.OperationUpdate || event.Operation == cdc.OperationDelete {
		if h.alertManager != nil {
			recordID := fmt.Sprintf("%v", event.PrimaryKey)
			details := fmt.Sprintf("Unauthorized %s operation detected on protected table", event.Operation)
			_ = h.alertManager.SendTamperAlert(event.TableName, string(event.Operation), recordID, details)
		}
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

	entry := &storage.HashEntry{
		TableName:     event.TableName,
		SequenceNum:   seqNum,
		Hash:          newHash,
		PreviousHash:  previousHash,
		Timestamp:     time.Now(),
		OperationType: string(event.Operation),
		RecordID:      fmt.Sprintf("%v", event.PrimaryKey),
	}

	return h.storage.SaveHashEntry(entry)
}

func (h *HashChainHandler) handleStateIntegrity(event *cdc.ChangeEvent) error {
	return nil
}

func (h *HashChainHandler) VerifyHashChain(tableName string) error {
	config, ok := h.tableConfigs[tableName]
	if !ok {
		return fmt.Errorf("table not configured: %s", tableName)
	}

	if config.Mode != AppendOnlyMode {
		return fmt.Errorf("hash chain verification only supported for append_only mode")
	}

	chain := hash.NewHashChain("genesis")
	seqNum := uint64(1)

	for {
		entry, err := h.storage.GetHashEntry(tableName, seqNum)
		if err != nil {
			break
		}

		if entry.PreviousHash != chain.GetPreviousHash() {
			if h.alertManager != nil {
				_ = h.alertManager.SendHashChainBrokenAlert(tableName, seqNum, chain.GetPreviousHash(), entry.PreviousHash)
			}
			return fmt.Errorf("hash chain broken at sequence %d: expected previous %s, got %s",
				seqNum, chain.GetPreviousHash(), entry.PreviousHash)
		}

		chain.SetPreviousHash(entry.Hash)
		seqNum++
	}

	return nil
}
