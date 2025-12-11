package verify

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/witnz/witnz/internal/hash"
	"github.com/witnz/witnz/internal/storage"
)

type StateIntegrityVerifier struct {
	storage    *storage.Storage
	dbConnStr  string
	tables     map[string]*TableConfig
	stopCh     chan struct{}
	merkleTree *hash.MerkleTree
}

func NewStateIntegrityVerifier(store *storage.Storage, dbConnStr string) *StateIntegrityVerifier {
	return &StateIntegrityVerifier{
		storage:    store,
		dbConnStr:  dbConnStr,
		tables:     make(map[string]*TableConfig),
		stopCh:     make(chan struct{}),
		merkleTree: hash.NewMerkleTree(),
	}
}

func (v *StateIntegrityVerifier) AddTable(config *TableConfig) {
	v.tables[config.Name] = config
}

func (v *StateIntegrityVerifier) Start(ctx context.Context) error {
	for tableName, config := range v.tables {
		interval := 5 * time.Minute
		if config.VerifyInterval != "" {
			duration, err := time.ParseDuration(config.VerifyInterval)
			if err == nil {
				interval = duration
			}
		}

		go v.verifyLoop(ctx, tableName, interval)
	}

	return nil
}

func (v *StateIntegrityVerifier) Stop() {
	close(v.stopCh)
}

func (v *StateIntegrityVerifier) verifyLoop(ctx context.Context, tableName string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-v.stopCh:
			return
		case <-ticker.C:
			if err := v.VerifyTable(ctx, tableName); err != nil {
				fmt.Printf("State integrity verification failed for %s: %v\n", tableName, err)
			}
		}
	}
}

func (v *StateIntegrityVerifier) VerifyTable(ctx context.Context, tableName string) error {
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	merkleRoot, recordCount, err := v.calculateMerkleRoot(ctx, conn, tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate merkle root: %w", err)
	}

	entry := &storage.MerkleRootEntry{
		TableName:   tableName,
		Root:        merkleRoot,
		Timestamp:   time.Now(),
		RecordCount: recordCount,
	}

	if err := v.storage.SaveMerkleRoot(entry); err != nil {
		return fmt.Errorf("failed to save merkle root: %w", err)
	}

	fmt.Printf("State integrity check for %s: root=%s, records=%d\n",
		tableName, merkleRoot[:16], recordCount)

	return nil
}

func (v *StateIntegrityVerifier) calculateMerkleRoot(ctx context.Context, conn *pgx.Conn, tableName string) (string, int, error) {
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY 1", tableName)

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return "", 0, fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	v.merkleTree.Reset()
	recordCount := 0

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return "", 0, fmt.Errorf("failed to read row: %w", err)
		}

		rowData := make(map[string]interface{})
		fieldDescriptions := rows.FieldDescriptions()
		for i, val := range values {
			rowData[string(fieldDescriptions[i].Name)] = val
		}

		if err := v.merkleTree.AddLeaf(rowData); err != nil {
			return "", 0, fmt.Errorf("failed to add leaf: %w", err)
		}

		recordCount++
	}

	if err := rows.Err(); err != nil {
		return "", 0, fmt.Errorf("error iterating rows: %w", err)
	}

	root := v.merkleTree.GetRoot()
	return root, recordCount, nil
}

func (v *StateIntegrityVerifier) CompareWithStored(tableName string) error {
	stored, err := v.storage.GetMerkleRoot(tableName)
	if err != nil {
		return fmt.Errorf("failed to get stored merkle root: %w", err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	current, recordCount, err := v.calculateMerkleRoot(ctx, conn, tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate current merkle root: %w", err)
	}

	if stored.Root != current {
		return fmt.Errorf("TAMPERING DETECTED: merkle root mismatch for table %s (stored: %s, current: %s, record count: %d)",
			tableName, stored.Root[:16], current[:16], recordCount)
	}

	return nil
}
