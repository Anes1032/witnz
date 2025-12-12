package verify

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/witnz/witnz/internal/alert"
	"github.com/witnz/witnz/internal/cdc"
	"github.com/witnz/witnz/internal/hash"
	"github.com/witnz/witnz/internal/storage"
)

type TableConfig struct {
	Name           string
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
	_, ok := h.tableConfigs[event.TableName]
	if !ok {
		return nil
	}

	// Reject UPDATE/DELETE on protected tables
	if event.Operation == cdc.OperationUpdate || event.Operation == cdc.OperationDelete {
		return NewTamperingError(event.TableName, string(event.Operation), "append-only")
	}

	// Process INSERT
	chain, ok := h.hashChains[event.TableName]
	if !ok {
		return fmt.Errorf("no hash chain for table: %s", event.TableName)
	}

	previousHash := chain.GetPreviousHash()

	// Calculate data hash of the record content
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

	entry := &storage.HashEntry{
		TableName:     event.TableName,
		SequenceNum:   seqNum,
		Hash:          newHash,
		PreviousHash:  previousHash,
		DataHash:      dataHash,
		Timestamp:     time.Now(),
		OperationType: string(event.Operation),
		RecordID:      fmt.Sprintf("%v", event.PrimaryKey),
	}

	return h.storage.SaveHashEntry(entry)
}

func (h *HashChainHandler) VerifyHashChain(tableName string) error {
	_, ok := h.tableConfigs[tableName]
	if !ok {
		return fmt.Errorf("table not configured: %s", tableName)
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

func calculateDataHash(data map[string]interface{}) string {
	// Normalize data for consistent hashing
	normalized := normalizeForHash(data)
	jsonData, _ := json.Marshal(normalized)
	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash)
}

// normalizeForHash normalizes data for consistent hash calculation
// This ensures the same hash is produced regardless of data source (CDC vs DB query)
// Excludes timestamp fields as they have different formats between CDC and DB
func normalizeForHash(data map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range data {
		// Skip timestamp fields that may have format differences
		if k == "created_at" || k == "updated_at" {
			continue
		}
		// Convert all values to string representation
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}
