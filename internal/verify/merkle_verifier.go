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
	actualMerkleRoot, pgRecordCount, pgTree, err := v.calculateCurrentMerkleRootFromPG(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate actual Merkle Root from PostgreSQL: %w", err)
	}

	expectedMerkleRoot, boltdbRecordCount, err := v.calculateMerkleRootFromBoltDB(tableName)
	if err != nil {
		return fmt.Errorf("failed to calculate expected Merkle Root from BoltDB: %w", err)
	}

	if boltdbRecordCount == 0 {
		fmt.Printf("No hash entries in BoltDB for %s, creating initial checkpoint...\n", tableName)
		return v.createCheckpointWithTree(tableName, actualMerkleRoot, pgRecordCount, pgTree)
	}

	if actualMerkleRoot == expectedMerkleRoot && pgRecordCount == boltdbRecordCount {
		fmt.Printf("✅ Merkle Root match for %s (PostgreSQL matches BoltDB)\n", tableName)
		return v.createCheckpointWithTree(tableName, actualMerkleRoot, pgRecordCount, pgTree)
	}

	fmt.Printf("⚠️  Merkle Root mismatch for %s, performing detailed verification...\n", tableName)
	expectedShort := expectedMerkleRoot
	if len(expectedShort) > 16 {
		expectedShort = expectedShort[:16] + "..."
	}
	actualShort := actualMerkleRoot
	if len(actualShort) > 16 {
		actualShort = actualShort[:16] + "..."
	}
	fmt.Printf("   Expected (BoltDB): %s (%d records)\n", expectedShort, boltdbRecordCount)
	fmt.Printf("   Actual (PostgreSQL): %s (%d records)\n", actualShort, pgRecordCount)
	return v.performDetailedVerification(ctx, tableName)
}

func (v *MerkleVerifier) performDetailedVerification(ctx context.Context, tableName string) error {
	expectedLeafMap, boltdbIDs, err := v.buildLeafMapFromBoltDB(tableName)
	if err != nil {
		return fmt.Errorf("failed to build expected leaf map: %w", err)
	}

	actualLeafMap, pgIDs, newMerkleRoot, err := v.buildLeafMapFromPostgreSQL(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to build actual leaf map: %w", err)
	}

	tamperedRecords := []string{}
	phantomInserts := []string{}
	deletedRecords := []string{}

	// Identify Phantom Inserts (in PostgreSQL but not in BoltDB)
	for id := range pgIDs {
		if !boltdbIDs[id] {
			phantomInserts = append(phantomInserts, id)
			msg := fmt.Sprintf("found extra record in DB not in hash chain (Phantom Insert): id=%s", id)
			tamperedRecords = append(tamperedRecords, msg)
			fmt.Printf("  TAMPERING: %s\n", msg)
		}
	}

	// Identify Deleted Records (in BoltDB but not in PostgreSQL)
	for id := range boltdbIDs {
		if !pgIDs[id] {
			deletedRecords = append(deletedRecords, id)
			msg := fmt.Sprintf("record deleted: id=%s", id)
			tamperedRecords = append(tamperedRecords, msg)
			fmt.Printf("  TAMPERING: %s\n", msg)
		}
	}

	// Create adjusted maps to detect data modifications
	// Remove phantom inserts from actual map for comparison
	adjustedActualMap := make(map[string]string)
	for id, hash := range actualLeafMap {
		isPhantom := false
		for _, phantomID := range phantomInserts {
			if id == phantomID {
				isPhantom = true
				break
			}
		}
		if !isPhantom {
			adjustedActualMap[id] = hash
		}
	}

	// Remove deleted records from expected map for comparison
	adjustedExpectedMap := make(map[string]string)
	for id, hash := range expectedLeafMap {
		isDeleted := false
		for _, deletedID := range deletedRecords {
			if id == deletedID {
				isDeleted = true
				break
			}
		}
		if !isDeleted {
			adjustedExpectedMap[id] = hash
		}
	}

	// Compare adjusted maps to find data modifications
	differingRecords := hash.CompareLeafMaps(adjustedExpectedMap, adjustedActualMap)

	for _, diff := range differingRecords {
		id := diff.RecordID

		// Skip records that are phantom inserts or deleted
		isPhantomOrDeleted := false
		for _, phantomID := range phantomInserts {
			if id == phantomID {
				isPhantomOrDeleted = true
				break
			}
		}
		for _, deletedID := range deletedRecords {
			if id == deletedID {
				isPhantomOrDeleted = true
				break
			}
		}

		if isPhantomOrDeleted {
			continue
		}

		if diff.Type == "modified" {
			hashLen := min(16, len(diff.ExpectedHash), len(diff.ActualHash))
			msg := fmt.Sprintf("data modified: id=%s, expected hash=%s..., actual hash=%s...",
				id, diff.ExpectedHash[:hashLen], diff.ActualHash[:hashLen])
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

	return v.createCheckpoint(tableName, newMerkleRoot, len(actualLeafMap))
}

func (v *MerkleVerifier) buildLeafMapFromBoltDB(tableName string) (map[string]string, map[string]bool, error) {
	entries, err := v.storage.GetAllHashEntries(tableName)
	if err != nil {
		return nil, nil, err
	}

	leafMap := make(map[string]string)
	ids := make(map[string]bool)

	for _, entry := range entries {
		id := extractIDFromRecordID(entry.RecordID)
		leafMap[id] = entry.DataHash
		ids[id] = true
	}

	return leafMap, ids, nil
}

func (v *MerkleVerifier) buildLeafMapFromPostgreSQL(ctx context.Context, tableName string) (map[string]string, map[string]bool, string, error) {
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT * FROM %s ORDER BY id", quoteIdentifier(tableName)))
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	leafMap := make(map[string]string)
	ids := make(map[string]bool)
	builder := hash.NewMerkleTreeBuilder()

	for rows.Next() {
		fieldDescs := rows.FieldDescriptions()
		values, err := rows.Values()
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to scan row: %w", err)
		}

		recordData := make(map[string]interface{})
		var idValue string
		for i, fd := range fieldDescs {
			fieldName := string(fd.Name)
			recordData[fieldName] = values[i]
			if fieldName == "id" {
				idValue = fmt.Sprintf("%v", values[i])
			}
		}

		dataHash := hash.CalculateDataHash(recordData)
		leafMap[idValue] = dataHash
		ids[idValue] = true

		// For Merkle tree construction, use full record representation
		recordID := fmt.Sprintf("%v", recordData)
		builder.AddLeafHash(recordID, dataHash)
	}

	var merkleRoot string
	if len(leafMap) > 0 {
		if err := builder.Build(); err != nil {
			return nil, nil, "", fmt.Errorf("failed to build tree: %w", err)
		}
		merkleRoot = builder.GetRoot()
	}

	return leafMap, ids, merkleRoot, nil
}

func (v *MerkleVerifier) calculateCurrentMerkleRootFromPG(ctx context.Context, tableName string) (string, int, *hash.MerkleTreeBuilder, error) {
	conn, err := pgx.Connect(ctx, v.dbConnStr)
	if err != nil {
		return "", 0, nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, fmt.Sprintf("SELECT * FROM %s ORDER BY id", quoteIdentifier(tableName)))
	if err != nil {
		return "", 0, nil, fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	builder := hash.NewMerkleTreeBuilder()
	recordCount := 0

	for rows.Next() {
		fieldDescs := rows.FieldDescriptions()
		values, err := rows.Values()
		if err != nil {
			return "", 0, nil, fmt.Errorf("failed to scan row: %w", err)
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
			return "", 0, nil, fmt.Errorf("failed to add leaf: %w", err)
		}
		recordCount++
	}

	if recordCount == 0 {
		return "", 0, nil, nil
	}

	if err := builder.Build(); err != nil {
		return "", 0, nil, fmt.Errorf("failed to build tree: %w", err)
	}

	return builder.GetRoot(), recordCount, builder, nil
}

func (v *MerkleVerifier) calculateMerkleRootFromBoltDB(tableName string) (string, int, error) {
	// Try to use checkpoint optimization for O(k log n) complexity
	checkpoint, err := v.storage.GetLatestMerkleCheckpoint(tableName)
	if err == nil && checkpoint != nil && len(checkpoint.LeafMap) > 0 {
		// Use checkpoint-based reconstruction (O(k log n) where k = new entries)
		return v.calculateMerkleRootFromCheckpoint(tableName, checkpoint)
	}

	// Fallback to full scan if no checkpoint exists (O(n))
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

// calculateMerkleRootFromCheckpoint uses checkpoint data to optimize tree construction
// Only processes entries added after the checkpoint
func (v *MerkleVerifier) calculateMerkleRootFromCheckpoint(tableName string, checkpoint *storage.MerkleCheckpoint) (string, int, error) {
	// Start with checkpoint's leaf map
	leafMap := make(map[string]string, len(checkpoint.LeafMap))
	for k, v := range checkpoint.LeafMap {
		leafMap[k] = v
	}

	// Get entries added after checkpoint
	entries, err := v.storage.GetAllHashEntries(tableName)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get hash entries: %w", err)
	}

	// Update leaf map with new entries (those with SequenceNum > checkpoint.SequenceNum)
	newEntryCount := 0
	for _, entry := range entries {
		if entry.SequenceNum > checkpoint.SequenceNum {
			leafMap[entry.RecordID] = entry.DataHash
			newEntryCount++
		}
	}

	// Build Merkle tree from updated leaf map
	builder := hash.NewMerkleTreeBuilder()
	for recordID, dataHash := range leafMap {
		builder.AddLeafHash(recordID, dataHash)
	}

	if err := builder.Build(); err != nil {
		return "", 0, fmt.Errorf("failed to build Merkle Tree: %w", err)
	}

	return builder.GetRoot(), len(leafMap), nil
}

func (v *MerkleVerifier) createCheckpoint(tableName, merkleRoot string, recordCount int) error {
	return v.createCheckpointWithTree(tableName, merkleRoot, recordCount, nil)
}

// createCheckpointWithTree creates a checkpoint with optional Merkle tree data
func (v *MerkleVerifier) createCheckpointWithTree(tableName, merkleRoot string, recordCount int, tree *hash.MerkleTreeBuilder) error {
	latestEntry, err := v.storage.GetLatestHashEntry(tableName)
	var seqNum uint64 = 0
	if err == nil {
		seqNum = latestEntry.SequenceNum
	}

	checkpoint := &storage.MerkleCheckpoint{
		TableName:     tableName,
		SequenceNum:   seqNum,
		MerkleRoot:    merkleRoot,
		Timestamp:     time.Now(),
		RecordCount:   recordCount,
		HashAlgorithm: hash.GetHasher().Name(),
	}

	// Store tree data if provided
	if tree != nil {
		checkpoint.LeafMap = tree.GetLeafMap()
		checkpoint.InternalNodes = tree.GetInternalNodes()
	}

	if err := v.storage.SaveMerkleCheckpoint(checkpoint); err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	if len(merkleRoot) >= 16 {
		fmt.Printf("📍 Created Merkle checkpoint for %s (root: %s..., records: %d, algorithm: %s)\n",
			tableName, merkleRoot[:16], recordCount, checkpoint.HashAlgorithm)
	} else {
		fmt.Printf("📍 Created Merkle checkpoint for %s (root: %s, records: %d, algorithm: %s)\n",
			tableName, merkleRoot, recordCount, checkpoint.HashAlgorithm)
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
