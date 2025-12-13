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

	t.Run("DetectUpdateOnAppendOnlyTable", func(t *testing.T) {
		event := &cdc.ChangeEvent{
			TableName: "test_table",
			Operation: cdc.OperationUpdate,
			Timestamp: time.Now(),
		}

		// UPDATE should return TamperingError
		err := handler.HandleChange(event)
		if err == nil {
			t.Error("HandleChange should return error on UPDATE")
		}
		if !IsTamperingError(err) {
			t.Errorf("Expected TamperingError, got: %v", err)
		}
	})

	t.Run("DetectDeleteOnAppendOnlyTable", func(t *testing.T) {
		event := &cdc.ChangeEvent{
			TableName: "test_table",
			Operation: cdc.OperationDelete,
			Timestamp: time.Now(),
		}

		// DELETE should return TamperingError
		err := handler.HandleChange(event)
		if err == nil {
			t.Error("HandleChange should return error on DELETE")
		}
		if !IsTamperingError(err) {
			t.Errorf("Expected TamperingError, got: %v", err)
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

func TestAddTableValidation(t *testing.T) {
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

	tests := []struct {
		name      string
		tableName string
		wantErr   bool
	}{
		{"valid_simple", "audit_logs", false},
		{"valid_underscore", "_private_table", false},
		{"valid_mixed", "Table_123", false},
		{"invalid_sql_injection", "audit_logs; DROP TABLE users;--", true},
		{"invalid_spaces", "audit logs", true},
		{"invalid_quotes", `audit"logs`, true},
		{"invalid_semicolon", "audit;logs", true},
		{"invalid_dash", "audit-logs", true},
		{"invalid_start_number", "123table", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.AddTable(&TableConfig{Name: tt.tableName})
			if (err != nil) != tt.wantErr {
				t.Errorf("AddTable(%q) error = %v, wantErr %v", tt.tableName, err, tt.wantErr)
			}
		})
	}
}
