package verify

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/witnz/witnz/internal/hash"
	"github.com/witnz/witnz/internal/storage"
)

var validTableNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type MerkleVerifier struct {
	storage   *storage.Storage
	dbConnStr string
	tables    []*TableConfig
	mu        sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewMerkleVerifier(store *storage.Storage, dbConnStr string) *MerkleVerifier {
	return &MerkleVerifier{
		storage:   store,
		dbConnStr: dbConnStr,
		tables:    make([]*TableConfig, 0),
		stopCh:    make(chan struct{}),
	}
}

func (v *MerkleVerifier) AddTable(config *TableConfig) error {
	if !validTableNameRegex.MatchString(config.Name) {
		return fmt.Errorf("invalid table name: %s", config.Name)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.tables = append(v.tables, config)
	return nil
}

func quoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (v *MerkleVerifier) Start(ctx context.Context) error {
	fmt.Println("Running startup Merkle Root verification...")
	for _, table := range v.tables {
		if err := v.VerifyTable(ctx, table.Name); err != nil {
			fmt.Printf("⚠️  VERIFICATION WARNING for %s: %v\n", table.Name, err)
		} else {
			fmt.Printf("✅ Merkle Root verified for %s\n", table.Name)
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

func (v *MerkleVerifier) Stop() {
	close(v.stopCh)
	v.wg.Wait()
}

func (v *MerkleVerifier) runPeriodicVerification(ctx context.Context, tableName string, interval time.Duration) {
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

func (v *MerkleVerifier) VerifyTable(ctx context.Context, tableName string) error {
	latestCheckpoint, err := v.storage.GetLatestMerkleCheckpoint(tableName)
	if err != nil {
		return v.performFullVerification(ctx, tableName)
	}

	expectedMerkleRoot, boltdbRecordCount, err := v.calculateMerkleRootFromBoltDB(tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate expected Merkle Root from BoltDB: %w", err)
	}

	actualMerkleRoot, pgRecordCount, err := v.calculateCurrentMerkleRoot(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate actual Merkle Root from PostgreSQL: %w", err)
	}

	if actualMerkleRoot == expectedMerkleRoot && pgRecordCount == boltdbRecordCount {
		fmt.Printf("✅ Merkle Root match for %s (PostgreSQL matches BoltDB)\n", tableName)
		return v.createCheckpoint(ctx, tableName, actualMerkleRoot, pgRecordCount)
	}

	fmt.Printf("⚠️  Merkle Root mismatch for %s, performing detailed verification...\n", tableName)
	fmt.Printf("   Expected (BoltDB): %s (%d records)\n", expectedMerkleRoot[:16]+"...", boltdbRecordCount)
	fmt.Printf("   Actual (PostgreSQL): %s (%d records)\n", actualMerkleRoot[:16]+"...", pgRecordCount)
	return v.performDetailedVerification(ctx, tableName, latestCheckpoint.MerkleRoot)
}

func (v *MerkleVerifier) performFullVerification(ctx context.Context, tableName string) error {
	currentMerkleRoot, recordCount, err := v.calculateCurrentMerkleRoot(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate Merkle Root: %w", err)
	}

	entries, err := v.storage.GetAllHashEntries(tableName)
	if err != nil {
		return fmt.Errorf("failed to get hash entries: %w", err)
	}

	if len(entries) == 0 {
		return v.createCheckpoint(ctx, tableName, currentMerkleRoot, recordCount)
	}

	builder := hash.NewMerkleTreeBuilder()
	for _, entry := range entries {
		builder.AddLeafHash(entry.RecordID, entry.DataHash)
	}

	if err := builder.Build(); err != nil {
		return fmt.Errorf("failed to build Merkle Tree: %w", err)
	}

	storedMerkleRoot := builder.GetRoot()

	if currentMerkleRoot == storedMerkleRoot {
		fmt.Printf("✅ Full verification passed for %s\n", tableName)
		return v.createCheckpoint(ctx, tableName, currentMerkleRoot, recordCount)
	}

	return v.performDetailedVerification(ctx, tableName, storedMerkleRoot)
}

func (v *MerkleVerifier) performDetailedVerification(ctx context.Context, tableName, storedMerkleRoot string) error {
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	entries, err := v.storage.GetAllHashEntries(tableName)
	if err != nil {
		return fmt.Errorf("failed to get hash entries: %w", err)
	}

	dbRecordIDs := make(map[string]bool)
	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT id FROM %s", quoteIdentifier(tableName)))
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

	tamperedRecords := []string{}

	for _, entry := range entries {
		id := extractIDFromRecordID(entry.RecordID)

		if !dbRecordIDs[id] {
			msg := fmt.Sprintf("seq=%d (record deleted: id=%s)", entry.SequenceNum, id)
			tamperedRecords = append(tamperedRecords, msg)
			fmt.Printf("  TAMPERING: %s\n", msg)
			continue
		}

		delete(dbRecordIDs, id)

		recordData, err := v.getRecordFromDB(ctx, conn, tableName, entry.RecordID)
		if err != nil {
			tamperedRecords = append(tamperedRecords, fmt.Sprintf("seq=%d (record unavailable: id=%s)", entry.SequenceNum, id))
			continue
		}

		builder := hash.NewMerkleTreeBuilder()
		if err := builder.AddLeaf(entry.RecordID, recordData); err != nil {
			tamperedRecords = append(tamperedRecords, fmt.Sprintf("seq=%d (hash calculation failed: id=%s)", entry.SequenceNum, id))
			continue
		}

		if err := builder.Build(); err != nil {
			continue
		}

		currentDataHash := builder.GetRoot()
		if currentDataHash != entry.DataHash {
			msg := fmt.Sprintf("seq=%d (data modified: id=%s, expected hash=%s, actual hash=%s)",
				entry.SequenceNum, id, entry.DataHash[:16]+"...", currentDataHash[:16]+"...")
			tamperedRecords = append(tamperedRecords, msg)
			fmt.Printf("  TAMPERING: %s\n", msg)
		}
	}

	if len(dbRecordIDs) > 0 {
		for id := range dbRecordIDs {
			msg := fmt.Sprintf("found extra record in DB not in hash chain (Phantom Insert): id=%s", id)
			tamperedRecords = append(tamperedRecords, msg)
			fmt.Printf("  TAMPERING: %s\n", msg)
		}
	}

	if len(tamperedRecords) > 0 {
		errMsg := fmt.Sprintf("🚨 CRITICAL: PostgreSQL tampering detected! Found %d tampered records", len(tamperedRecords))
		fmt.Println(errMsg)
		fmt.Println("Tampered records:")
		for _, record := range tamperedRecords {
			fmt.Printf("  - %s\n", record)
		}

		return fmt.Errorf("%s", errMsg)
	}

	// No tampering detected - update checkpoint with current state
	currentMerkleRoot, recordCount, err := v.calculateCurrentMerkleRoot(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate current Merkle Root: %w", err)
	}
	return v.createCheckpoint(ctx, tableName, currentMerkleRoot, recordCount)
}

func (v *MerkleVerifier) calculateCurrentMerkleRoot(ctx context.Context, tableName string) (string, int, error) {
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT * FROM %s ORDER BY id", quoteIdentifier(tableName)))
	if err != nil {
		return "", 0, fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	builder := hash.NewMerkleTreeBuilder()
	recordCount := 0

	for rows.Next() {
		fieldDescs := rows.FieldDescriptions()
		values, err := rows.Values()
		if err != nil {
			return "", 0, fmt.Errorf("failed to scan row: %w", err)
		}

		recordData := make(map[string]interface{})
		var recordID string
		for i, fd := range fieldDescs {
			fieldName := string(fd.Name)
			recordData[fieldName] = values[i]
			if fieldName == "id" {
				recordID = fmt.Sprintf("%v", values[i])
			}
		}

		if err := builder.AddLeaf(recordID, recordData); err != nil {
			return "", 0, fmt.Errorf("failed to add leaf: %w", err)
		}
		recordCount++
	}

	if recordCount == 0 {
		return "", 0, nil
	}

	if err := builder.Build(); err != nil {
		return "", 0, fmt.Errorf("failed to build tree: %w", err)
	}

	return builder.GetRoot(), recordCount, nil
}

func (v *MerkleVerifier) calculateMerkleRootFromBoltDB(tableName string) (string, int, error) {
	entries, err := v.storage.GetAllHashEntries(tableName)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get hash entries from BoltDB: %w", err)
	}

	if len(entries) == 0 {
		return "", 0, nil
	}

	builder := hash.NewMerkleTreeBuilder()

	for _, entry := range entries {
		builder.AddLeafHash(entry.RecordID, entry.DataHash)
	}

	if err := builder.Build(); err != nil {
		return "", 0, fmt.Errorf("failed to build Merkle Tree from BoltDB: %w", err)
	}

	return builder.GetRoot(), len(entries), nil
}

func (v *MerkleVerifier) createCheckpoint(ctx context.Context, tableName, merkleRoot string, recordCount int) error {
	latestEntry, err := v.storage.GetLatestHashEntry(tableName)
	var seqNum uint64 = 0
	if err == nil {
		seqNum = latestEntry.SequenceNum
	}

	checkpoint := &storage.MerkleCheckpoint{
		TableName:   tableName,
		SequenceNum: seqNum,
		MerkleRoot:  merkleRoot,
		Timestamp:   time.Now(),
		RecordCount: recordCount,
	}

	if err := v.storage.SaveMerkleCheckpoint(checkpoint); err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	if len(merkleRoot) >= 16 {
		fmt.Printf("📍 Created Merkle checkpoint for %s (root: %s..., records: %d)\n",
			tableName, merkleRoot[:16], recordCount)
	} else {
		fmt.Printf("📍 Created Merkle checkpoint for %s (root: %s, records: %d)\n",
			tableName, merkleRoot, recordCount)
	}
	return nil
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

func (v *MerkleVerifier) getRecordFromDB(ctx context.Context, conn *pgx.Conn, tableName string, recordID string) (map[string]interface{}, error) {
	id := extractIDFromRecordID(recordID)

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1", quoteIdentifier(tableName))

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
