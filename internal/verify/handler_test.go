package verify

import (
	"os"
	"testing"
	"time"

	"github.com/witnz/witnz/internal/cdc"
	"github.com/witnz/witnz/internal/storage"
)

func TestHashChainHandler(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-verify-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	handler := NewHashChainHandler(store)

	config := &TableConfig{
		Name: "test_table",
		Mode: AppendOnlyMode,
	}

	err = handler.AddTable(config)
	if err != nil {
		t.Fatalf("Failed to add table: %v", err)
	}

	t.Run("HandleInsertOnAppendOnlyTable", func(t *testing.T) {
		event := &cdc.ChangeEvent{
			TableName: "test_table",
			Operation: cdc.OperationInsert,
			Timestamp: time.Now(),
			NewData: map[string]interface{}{
				"id":   "1",
				"name": "test",
			},
			PrimaryKey: map[string]interface{}{
				"id": "1",
			},
		}

		err := handler.HandleChange(event)
		if err != nil {
			t.Fatalf("HandleChange failed: %v", err)
		}

		entry, err := store.GetHashEntry("test_table", 1)
		if err != nil {
			t.Fatalf("GetHashEntry failed: %v", err)
		}

		if entry.TableName != "test_table" {
			t.Errorf("Expected table name test_table, got %s", entry.TableName)
		}

		if entry.OperationType != "INSERT" {
			t.Errorf("Expected operation INSERT, got %s", entry.OperationType)
		}
	})

	t.Run("RejectUpdateOnAppendOnlyTable", func(t *testing.T) {
		event := &cdc.ChangeEvent{
			TableName: "test_table",
			Operation: cdc.OperationUpdate,
			Timestamp: time.Now(),
		}

		err := handler.HandleChange(event)
		if err == nil {
			t.Error("Expected error for UPDATE on append-only table")
		}
	})

	t.Run("RejectDeleteOnAppendOnlyTable", func(t *testing.T) {
		event := &cdc.ChangeEvent{
			TableName: "test_table",
			Operation: cdc.OperationDelete,
			Timestamp: time.Now(),
		}

		err := handler.HandleChange(event)
		if err == nil {
			t.Error("Expected error for DELETE on append-only table")
		}
	})

	t.Run("IgnoreUnconfiguredTable", func(t *testing.T) {
		event := &cdc.ChangeEvent{
			TableName: "unconfigured_table",
			Operation: cdc.OperationInsert,
			Timestamp: time.Now(),
		}

		err := handler.HandleChange(event)
		if err != nil {
			t.Errorf("Should ignore unconfigured table, got error: %v", err)
		}
	})
}

func TestVerifyHashChain(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-verify-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	store, err := storage.New(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	handler := NewHashChainHandler(store)

	config := &TableConfig{
		Name: "test_table",
		Mode: AppendOnlyMode,
	}

	handler.AddTable(config)

	for i := 1; i <= 3; i++ {
		event := &cdc.ChangeEvent{
			TableName: "test_table",
			Operation: cdc.OperationInsert,
			Timestamp: time.Now(),
			NewData: map[string]interface{}{
				"id":   i,
				"data": "test",
			},
			PrimaryKey: map[string]interface{}{
				"id": i,
			},
		}

		err := handler.HandleChange(event)
		if err != nil {
			t.Fatalf("HandleChange failed: %v", err)
		}
	}

	err = handler.VerifyHashChain("test_table")
	if err != nil {
		t.Errorf("VerifyHashChain failed: %v", err)
	}
}
