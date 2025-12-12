package verify

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/witnz/witnz/internal/storage"
)

type HashChainVerifier struct {
	storage   *storage.Storage
	dbConnStr string
	tables    []*TableConfig
	mu        sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewHashChainVerifier(store *storage.Storage, dbConnStr string) *HashChainVerifier {
	return &HashChainVerifier{
		storage:   store,
		dbConnStr: dbConnStr,
		tables:    make([]*TableConfig, 0),
		stopCh:    make(chan struct{}),
	}
}

func (v *HashChainVerifier) AddTable(config *TableConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.tables = append(v.tables, config)
}

func (v *HashChainVerifier) Start(ctx context.Context) error {
	fmt.Println("Running startup hash chain verification...")
	for _, table := range v.tables {
		if err := v.VerifyTable(ctx, table.Name); err != nil {
			fmt.Printf("⚠️  VERIFICATION WARNING for %s: %v\n", table.Name, err)
		} else {
			fmt.Printf("✅ Hash chain verified for %s\n", table.Name)
		}
	}

	for _, table := range v.tables {
		if table.VerifyInterval != "" {
			interval, err := time.ParseDuration(table.VerifyInterval)
			if err != nil {
				return fmt.Errorf("invalid verify_interval for %s: %w", table.Name, err)
			}

			v.wg.Add(1)
			go v.runPeriodicVerification(ctx, table.Name, interval)
		}
	}

	return nil
}

func (v *HashChainVerifier) Stop() {
	close(v.stopCh)
	v.wg.Wait()
}

func (v *HashChainVerifier) runPeriodicVerification(ctx context.Context, tableName string, interval time.Duration) {
	defer v.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-v.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := v.VerifyTable(ctx, tableName); err != nil {
				fmt.Printf("🚨 TAMPERING DETECTED during verification of %s: %v\n", tableName, err)
			}
		}
	}
}

func (v *HashChainVerifier) VerifyTable(ctx context.Context, tableName string) error {
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	dbRecordIDs := make(map[string]bool)
	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT id FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("failed to query IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id interface{}
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan ID: %w", err)
		}
		dbRecordIDs[fmt.Sprintf("%v", id)] = true
	}

	seqNum := uint64(1)
	tamperedRecords := []string{}

	for {
		entry, err := v.storage.GetHashEntry(tableName, seqNum)
		if err != nil {
			break
		}

		id := extractIDFromRecordID(entry.RecordID)

		if !dbRecordIDs[id] {
			tamperedRecords = append(tamperedRecords, fmt.Sprintf("seq=%d (record deleted: id=%s)", seqNum, id))
			seqNum++
			continue
		}

		delete(dbRecordIDs, id)

		recordData, err := v.getRecordFromDB(ctx, conn, tableName, entry.RecordID)
		if err != nil {
			tamperedRecords = append(tamperedRecords, fmt.Sprintf("seq=%d (record unavailable: id=%s)", seqNum, id))
			continue
		}

		currentDataHash := calculateRecordHash(recordData)

		if entry.DataHash != "" && entry.DataHash != currentDataHash {
			tamperedRecords = append(tamperedRecords, fmt.Sprintf("seq=%d id=%s", seqNum, id))
			fmt.Printf("  TAMPERING: Record id=%s modified (stored hash: %s..., current: %s...)\n",
				id, entry.DataHash[:16], currentDataHash[:16])
		}

		seqNum++
	}

	if len(dbRecordIDs) > 0 {
		for id := range dbRecordIDs {
			msg := fmt.Sprintf("found extra record in DB not in hash chain (Phantom Insert): id=%s", id)
			tamperedRecords = append(tamperedRecords, msg)
			fmt.Printf("  TAMPERING: %s\n", msg)
		}
	}

	if len(tamperedRecords) > 0 {
		return fmt.Errorf("found %d tampered records: %v", len(tamperedRecords), tamperedRecords)
	}

	return nil
}

func (v *HashChainVerifier) getRecordFromDB(ctx context.Context, conn *pgx.Conn, tableName string, recordID string) (map[string]interface{}, error) {
	id := extractIDFromRecordID(recordID)

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", tableName)

	rows, err := conn.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("record not found")
	}

	fieldDescs := rows.FieldDescriptions()
	values, err := rows.Values()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for i, fd := range fieldDescs {
		result[string(fd.Name)] = values[i]
	}

	return result, nil
}

func extractIDFromRecordID(recordID string) string {
	if len(recordID) > 7 && recordID[:4] == "map[" {
		idStart := -1
		for i := 0; i < len(recordID)-3; i++ {
			if recordID[i:i+3] == "id:" {
				idStart = i + 3
				break
			}
		}
		if idStart > 0 {
			idEnd := idStart
			for idEnd < len(recordID) && recordID[idEnd] != ']' && recordID[idEnd] != ' ' {
				idEnd++
			}
			return recordID[idStart:idEnd]
		}
	}
	return recordID
}

func calculateRecordHash(data map[string]interface{}) string {
	normalized := make(map[string]string)
	for k, v := range data {
		if k == "created_at" || k == "updated_at" {
			continue
		}
		normalized[k] = fmt.Sprintf("%v", v)
	}
	jsonData, _ := json.Marshal(normalized)
	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash)
}
