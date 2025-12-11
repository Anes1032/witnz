package verify

import (
	"context"
	"os"
	"testing"

	"github.com/witnz/witnz/internal/storage"
)

func TestStateIntegrityVerifier(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-state-test-*.db")
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

	connStr := "host=localhost port=5432 dbname=witnzdb user=witnz password=witnz_dev_password"
	verifier := NewStateIntegrityVerifier(store, connStr)

	if verifier == nil {
		t.Fatal("NewStateIntegrityVerifier returned nil")
	}

	config := &TableConfig{
		Name:           "test_table",
		Mode:           StateIntegrityMode,
		VerifyInterval: "1m",
	}

	verifier.AddTable(config)

	if len(verifier.tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(verifier.tables))
	}
}

func TestStateIntegrityVerifierAddTable(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-state-test-*.db")
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

	verifier := NewStateIntegrityVerifier(store, "")

	config1 := &TableConfig{Name: "table1", Mode: StateIntegrityMode}
	config2 := &TableConfig{Name: "table2", Mode: StateIntegrityMode}

	verifier.AddTable(config1)
	verifier.AddTable(config2)

	if len(verifier.tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(verifier.tables))
	}

	if verifier.tables["table1"] == nil {
		t.Error("table1 not found")
	}

	if verifier.tables["table2"] == nil {
		t.Error("table2 not found")
	}
}

func TestStateIntegrityVerifierStartStop(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "witnz-state-test-*.db")
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

	verifier := NewStateIntegrityVerifier(store, "")

	config := &TableConfig{
		Name:           "test_table",
		Mode:           StateIntegrityMode,
		VerifyInterval: "10s",
	}

	verifier.AddTable(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = verifier.Start(ctx)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	verifier.Stop()
}
