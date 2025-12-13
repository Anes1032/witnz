package verify

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/witnz/witnz/internal/alert"
	"github.com/witnz/witnz/internal/storage"
)

func TestFollowerVerifier_VerifyHashEntry(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	shutdownCalled := false
	shutdownFunc := func() error {
		shutdownCalled = true
		return nil
	}

	t.Run("no inconsistency when hashes match", func(t *testing.T) {
		shutdownCalled = false

		err := store.SaveHashEntry(&storage.HashEntry{
			TableName:   "test_table",
			SequenceNum: 1,
			Hash:        "abc123",
		})
		if err != nil {
			t.Fatalf("failed to save hash entry: %v", err)
		}

		verifier := NewFollowerVerifier(store, nil, shutdownFunc, true, logger)

		err = verifier.VerifyHashEntry("test_table", 1, "abc123")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if shutdownCalled {
			t.Error("shutdown should not be called when hashes match")
		}
	})

	t.Run("detects inconsistency when hashes differ", func(t *testing.T) {
		shutdownCalled = false

		err := store.SaveHashEntry(&storage.HashEntry{
			TableName:   "test_table2",
			SequenceNum: 2,
			Hash:        "local_hash_123",
		})
		if err != nil {
			t.Fatalf("failed to save hash entry: %v", err)
		}

		verifier := NewFollowerVerifier(store, nil, shutdownFunc, true, logger)

		err = verifier.VerifyHashEntry("test_table2", 2, "consensus_hash_456")
		if err == nil {
			t.Error("expected error for inconsistent hash")
		}

		if !shutdownCalled {
			t.Error("shutdown should be called when auto-shutdown is enabled")
		}
	})

	t.Run("no shutdown when auto-shutdown disabled", func(t *testing.T) {
		shutdownCalled = false

		err := store.SaveHashEntry(&storage.HashEntry{
			TableName:   "test_table3",
			SequenceNum: 3,
			Hash:        "local_hash_abc",
		})
		if err != nil {
			t.Fatalf("failed to save hash entry: %v", err)
		}

		verifier := NewFollowerVerifier(store, nil, shutdownFunc, false, logger)

		err = verifier.VerifyHashEntry("test_table3", 3, "consensus_hash_xyz")
		if err == nil {
			t.Error("expected error for inconsistent hash")
		}

		if shutdownCalled {
			t.Error("shutdown should not be called when auto-shutdown is disabled")
		}
	})

	t.Run("sets termination flag on shutdown", func(t *testing.T) {
		shutdownCalled = false

		err := store.SaveHashEntry(&storage.HashEntry{
			TableName:   "test_table4",
			SequenceNum: 4,
			Hash:        "original",
		})
		if err != nil {
			t.Fatalf("failed to save hash entry: %v", err)
		}

		verifier := NewFollowerVerifier(store, nil, shutdownFunc, true, logger)

		_ = verifier.VerifyHashEntry("test_table4", 4, "tampered")

		terminated, err := verifier.CheckTerminationFlag()
		if err != nil {
			t.Errorf("failed to check termination flag: %v", err)
		}

		if !terminated {
			t.Error("termination flag should be set after inconsistency shutdown")
		}
	})
}

func TestFollowerVerifier_WithAlertManager(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	alertMgr := alert.NewManager(false, "")

	shutdownFunc := func() error {
		return nil
	}

	err = store.SaveHashEntry(&storage.HashEntry{
		TableName:   "alert_test",
		SequenceNum: 1,
		Hash:        "hash1",
	})
	if err != nil {
		t.Fatalf("failed to save hash entry: %v", err)
	}

	verifier := NewFollowerVerifier(store, alertMgr, shutdownFunc, true, logger)

	err = verifier.VerifyHashEntry("alert_test", 1, "hash2")
	if err == nil {
		t.Error("expected error for inconsistent hash")
	}
}
