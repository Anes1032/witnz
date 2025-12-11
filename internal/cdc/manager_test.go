package cdc

import (
	"testing"

	"github.com/jackc/pglogrepl"
)

func TestNewManager(t *testing.T) {
	config := &ReplicationConfig{
		Host:            "localhost",
		Port:            5432,
		Database:        "testdb",
		User:            "testuser",
		Password:        "testpass",
		SlotName:        "test_slot",
		PublicationName: "test_pub",
	}

	manager := NewManager(config)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.config != config {
		t.Error("Config not set correctly")
	}

	if len(manager.handlers) != 0 {
		t.Error("Handlers should be empty initially")
	}
}

func TestManagerAddHandler(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	handler1 := &mockHandler{events: make([]*ChangeEvent, 0)}
	handler2 := &mockHandler{events: make([]*ChangeEvent, 0)}

	manager.AddHandler(handler1)
	manager.AddHandler(handler2)

	if len(manager.handlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(manager.handlers))
	}
}

func TestManagerHandleChange(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	handler := &mockHandler{events: make([]*ChangeEvent, 0)}
	manager.AddHandler(handler)

	event := &ChangeEvent{
		TableName: "test_table",
		Operation: OperationInsert,
	}

	err := manager.HandleChange(event)
	if err != nil {
		t.Fatalf("HandleChange failed: %v", err)
	}

	if len(handler.events) != 1 {
		t.Errorf("Expected 1 event in handler, got %d", len(handler.events))
	}

	if handler.events[0].TableName != "test_table" {
		t.Error("Event not forwarded correctly")
	}
}

func TestManagerLSN(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	lsn := pglogrepl.LSN(12345)
	manager.SetLSN(lsn)

	got := manager.GetLSN()
	if got != lsn {
		t.Errorf("Expected LSN %d, got %d", lsn, got)
	}
}
