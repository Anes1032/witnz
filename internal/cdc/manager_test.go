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

func TestManagerStartWithoutInit(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	ctx := t.Context()
	err := manager.Start(ctx)
	if err == nil {
		t.Error("Start should fail without Initialize")
	}
}

func TestManagerStopWhenNotRunning(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	ctx := t.Context()
	err := manager.Stop(ctx)
	if err != nil {
		t.Errorf("Stop should not fail when not running: %v", err)
	}
}

func TestManagerHandleChangeWithMultipleHandlers(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	handler1 := &mockHandler{events: make([]*ChangeEvent, 0)}
	handler2 := &mockHandler{events: make([]*ChangeEvent, 0)}
	handler3 := &mockHandler{events: make([]*ChangeEvent, 0)}

	manager.AddHandler(handler1)
	manager.AddHandler(handler2)
	manager.AddHandler(handler3)

	event := &ChangeEvent{
		TableName: "multi_test",
		Operation: OperationUpdate,
	}

	err := manager.HandleChange(event)
	if err != nil {
		t.Fatalf("HandleChange failed: %v", err)
	}

	if len(handler1.events) != 1 {
		t.Error("handler1 should have 1 event")
	}
	if len(handler2.events) != 1 {
		t.Error("handler2 should have 1 event")
	}
	if len(handler3.events) != 1 {
		t.Error("handler3 should have 1 event")
	}
}

func TestManagerHandleChangeWithNoHandlers(t *testing.T) {
	manager := NewManager(&ReplicationConfig{})

	event := &ChangeEvent{
		TableName: "test",
		Operation: OperationDelete,
	}

	err := manager.HandleChange(event)
	if err != nil {
		t.Errorf("HandleChange with no handlers should not error: %v", err)
	}
}
