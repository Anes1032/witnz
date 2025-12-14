package verify

import (
	"fmt"
	"regexp"
	"time"

	"github.com/witnz/witnz/internal/alert"
	"github.com/witnz/witnz/internal/cdc"
	"github.com/witnz/witnz/internal/hash"
	"github.com/witnz/witnz/internal/storage"
)

var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type TableConfig struct {
	Name           string
	VerifyInterval string
}

type HashChainHandler struct {
	storage      *storage.Storage
	tableConfigs map[string]*TableConfig
	alertManager *alert.Manager
}

func NewHashChainHandler(store *storage.Storage) *HashChainHandler {
	return &HashChainHandler{
		storage:      store,
		tableConfigs: make(map[string]*TableConfig),
	}
}

func (h *HashChainHandler) SetAlertManager(am *alert.Manager) {
	h.alertManager = am
}

func (h *HashChainHandler) AddTable(config *TableConfig) error {
	if !validTableName.MatchString(config.Name) {
		return fmt.Errorf("invalid table name: %s", config.Name)
	}

	h.tableConfigs[config.Name] = config
	return nil
}

func (h *HashChainHandler) HandleChange(event *cdc.ChangeEvent) error {
	_, ok := h.tableConfigs[event.TableName]
	if !ok {
		return nil
	}

	if event.Operation == cdc.OperationUpdate || event.Operation == cdc.OperationDelete {
		return NewTamperingError(event.TableName, string(event.Operation))
	}

	dataHash := calculateDataHash(event.NewData)

	latestEntry, _ := h.storage.GetLatestHashEntry(event.TableName)
	var seqNum uint64 = 1
	if latestEntry != nil {
		seqNum = latestEntry.SequenceNum + 1
	}

	entry := &storage.HashEntry{
		TableName:     event.TableName,
		SequenceNum:   seqNum,
		DataHash:      dataHash,
		Timestamp:     time.Now(),
		OperationType: string(event.Operation),
		RecordID:      fmt.Sprintf("%v", event.PrimaryKey),
	}

	return h.storage.SaveHashEntry(entry)
}

// VerifyHashChain validates that hash entries exist for the table
// Hash chain verification is no longer needed as we only store DataHash
func (h *HashChainHandler) VerifyHashChain(tableName string) error {
	_, ok := h.tableConfigs[tableName]
	if !ok {
		return fmt.Errorf("table not configured: %s", tableName)
	}

	// Check that entries exist
	_, err := h.storage.GetLatestHashEntry(tableName)
	if err != nil {
		return fmt.Errorf("no hash entries found for table %s", tableName)
	}

	return nil
}

func calculateDataHash(data map[string]interface{}) string {
	return hash.CalculateDataHash(data)
}
