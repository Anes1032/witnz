package cdc

import (
	"testing"
	"time"
)

type mockHandler struct {
	events []*ChangeEvent
}

func (m *mockHandler) HandleChange(event *ChangeEvent) error {
	m.events = append(m.events, event)
	return nil
}

func TestChangeEvent(t *testing.T) {
	event := &ChangeEvent{
		TableName: "test_table",
		Operation: OperationInsert,
		Timestamp: time.Now(),
		NewData: map[string]interface{}{
			"id":   "1",
			"name": "test",
		},
		PrimaryKey: map[string]interface{}{
			"id": "1",
		},
	}

	if event.TableName != "test_table" {
		t.Errorf("Expected table name test_table, got %s", event.TableName)
	}

	if event.Operation != OperationInsert {
		t.Errorf("Expected operation INSERT, got %s", event.Operation)
	}

	if event.NewData["id"] != "1" {
		t.Error("NewData not set correctly")
	}
}

func TestMockHandler(t *testing.T) {
	handler := &mockHandler{
		events: make([]*ChangeEvent, 0),
	}

	event := &ChangeEvent{
		TableName: "test",
		Operation: OperationInsert,
		Timestamp: time.Now(),
	}

	err := handler.HandleChange(event)
	if err != nil {
		t.Fatalf("HandleChange failed: %v", err)
	}

	if len(handler.events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(handler.events))
	}

	if handler.events[0].TableName != "test" {
		t.Error("Event not stored correctly")
	}
}

func TestOperationType(t *testing.T) {
	tests := []struct {
		name string
		op   OperationType
		want string
	}{
		{"insert", OperationInsert, "INSERT"},
		{"update", OperationUpdate, "UPDATE"},
		{"delete", OperationDelete, "DELETE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.op) != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, string(tt.op))
			}
		})
	}
}
