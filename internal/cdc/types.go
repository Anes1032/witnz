package cdc

import (
	"time"
)

type OperationType string

const (
	OperationInsert OperationType = "INSERT"
	OperationUpdate OperationType = "UPDATE"
	OperationDelete OperationType = "DELETE"
)

type ChangeEvent struct {
	TableName     string
	Operation     OperationType
	Timestamp     time.Time
	NewData       map[string]interface{}
	OldData       map[string]interface{}
	PrimaryKey    map[string]interface{}
	TransactionID uint32
	LSN           uint64
}

type EventHandler interface {
	HandleChange(event *ChangeEvent) error
}
