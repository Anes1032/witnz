package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

// HashEntry represents a hash chain entry stored in BoltDB
type HashEntry struct {
	TableName   string    `json:"table_name"`
	RecordID    string    `json:"record_id"`
	SequenceNum uint64    `json:"sequence_num"`
	Hash        string    `json:"hash"`
	DataHash    string    `json:"data_hash"`
	PrevHash    string    `json:"prev_hash"`
	Timestamp   time.Time `json:"timestamp"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <boltdb-path> <table-name>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This tool corrupts the first hash entry in the specified table\n")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	tableName := os.Args[2]

	fmt.Printf("Opening BoltDB: %s\n", dbPath)
	fmt.Printf("Target table: %s\n", tableName)

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open BoltDB: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	bucketName := []byte("hashchain")

	var targetKey []byte
	var targetEntry HashEntry

	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket not found: %s", bucketName)
		}

		// Find first entry matching the target table
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var entry HashEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}

			if entry.TableName == tableName {
				targetKey = make([]byte, len(k))
				copy(targetKey, k)
				targetEntry = entry
				fmt.Printf("Found entry for table %s (seq=%d)\n", tableName, entry.SequenceNum)
				fmt.Printf("  Original Hash: %s\n", entry.Hash[:32]+"...")
				fmt.Printf("  Original DataHash: %s\n", entry.DataHash[:32]+"...")
				break
			}
		}

		if len(targetKey) == 0 {
			return fmt.Errorf("no entries found for table: %s", tableName)
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Corrupt the hash (change first character)
	if targetEntry.Hash[0] == 'a' {
		targetEntry.Hash = "b" + targetEntry.Hash[1:]
	} else {
		targetEntry.Hash = "a" + targetEntry.Hash[1:]
	}

	if targetEntry.DataHash[0] == 'a' {
		targetEntry.DataHash = "b" + targetEntry.DataHash[1:]
	} else {
		targetEntry.DataHash = "a" + targetEntry.DataHash[1:]
	}

	fmt.Printf("Corrupted entry (seq=%d):\n", targetEntry.SequenceNum)
	fmt.Printf("  Hash: %s\n", targetEntry.Hash[:32]+"...")
	fmt.Printf("  DataHash: %s\n", targetEntry.DataHash[:32]+"...")

	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return fmt.Errorf("bucket not found: %s", bucketName)
		}

		// Save corrupted entry
		corruptedValue, err := json.Marshal(targetEntry)
		if err != nil {
			return fmt.Errorf("failed to marshal corrupted entry: %w", err)
		}

		if err := bucket.Put(targetKey, corruptedValue); err != nil {
			return fmt.Errorf("failed to save corrupted entry: %w", err)
		}

		fmt.Println("✓ Successfully corrupted hash entry")
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("BoltDB tampering completed")
}
