package storage

import (
	"os"
	"testing"
	"time"
)

func TestStorage(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	storage, err := New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	t.Run("SaveAndGetHashEntry", func(t *testing.T) {
		entry := &HashEntry{
			TableName:     "test_table",
			SequenceNum:   1,
			Hash:          "abcd1234",
			PreviousHash:  "0000",
			Timestamp:     time.Now(),
			OperationType: "INSERT",
			RecordID:      "123",
		}

		if err := storage.SaveHashEntry(entry); err != nil {
			t.Fatalf("SaveHashEntry failed: %v", err)
		}

		retrieved, err := storage.GetHashEntry("test_table", 1)
		if err != nil {
			t.Fatalf("GetHashEntry failed: %v", err)
		}

		if retrieved.Hash != entry.Hash {
			t.Errorf("Expected hash %s, got %s", entry.Hash, retrieved.Hash)
		}
	})

	t.Run("GetLatestHashEntry", func(t *testing.T) {
		entries := []*HashEntry{
			{TableName: "test_table", SequenceNum: 2, Hash: "hash2", Timestamp: time.Now()},
			{TableName: "test_table", SequenceNum: 3, Hash: "hash3", Timestamp: time.Now()},
		}

		for _, e := range entries {
			if err := storage.SaveHashEntry(e); err != nil {
				t.Fatalf("SaveHashEntry failed: %v", err)
			}
		}

		latest, err := storage.GetLatestHashEntry("test_table")
		if err != nil {
			t.Fatalf("GetLatestHashEntry failed: %v", err)
		}

		if latest.SequenceNum != 3 {
			t.Errorf("Expected sequence 3, got %d", latest.SequenceNum)
		}
	})

	t.Run("SetAndGetMetadata", func(t *testing.T) {
		key := "test_key"
		value := "test_value"

		if err := storage.SetMetadata(key, value); err != nil {
			t.Fatalf("SetMetadata failed: %v", err)
		}

		retrieved, err := storage.GetMetadata(key)
		if err != nil {
			t.Fatalf("GetMetadata failed: %v", err)
		}

		if retrieved != value {
			t.Errorf("Expected value %s, got %s", value, retrieved)
		}
	})
}
