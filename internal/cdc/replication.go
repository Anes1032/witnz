package cdc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	OutputPlugin = "pgoutput"
)

type ReplicationConfig struct {
	Host           string
	Port           int
	Database       string
	User           string
	Password       string
	SlotName       string
	PublicationName string
}

type ReplicationClient struct {
	config    *ReplicationConfig
	conn      *pgconn.PgConn
	relations map[uint32]*pglogrepl.RelationMessage
	typeMap   *pgtype.Map
	handler   EventHandler
}

func NewReplicationClient(config *ReplicationConfig, handler EventHandler) *ReplicationClient {
	return &ReplicationClient{
		config:    config,
		relations: make(map[uint32]*pglogrepl.RelationMessage),
		typeMap:   pgtype.NewMap(),
		handler:   handler,
	}
}

func (rc *ReplicationClient) Connect(ctx context.Context) error {
	connString := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s replication=database",
		rc.config.Host,
		rc.config.Port,
		rc.config.Database,
		rc.config.User,
		rc.config.Password,
	)

	conn, err := pgconn.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	rc.conn = conn
	return nil
}

func (rc *ReplicationClient) CreateSlotIfNotExists(ctx context.Context) error {
	if rc.conn == nil {
		return fmt.Errorf("not connected")
	}

	result, err := pglogrepl.CreateReplicationSlot(
		ctx,
		rc.conn,
		rc.config.SlotName,
		OutputPlugin,
		pglogrepl.CreateReplicationSlotOptions{},
	)

	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "42710" {
			return nil
		}
		return fmt.Errorf("failed to create replication slot: %w", err)
	}

	fmt.Printf("Created replication slot %s at LSN %s\n", result.SlotName, result.ConsistentPoint)
	return nil
}

func (rc *ReplicationClient) DropSlot(ctx context.Context) error {
	if rc.conn == nil {
		return fmt.Errorf("not connected")
	}

	err := pglogrepl.DropReplicationSlot(ctx, rc.conn, rc.config.SlotName, pglogrepl.DropReplicationSlotOptions{})
	if err != nil {
		return fmt.Errorf("failed to drop replication slot: %w", err)
	}

	return nil
}

func (rc *ReplicationClient) StartReplication(ctx context.Context, startLSN pglogrepl.LSN) error {
	if rc.conn == nil {
		return fmt.Errorf("not connected")
	}

	pluginArguments := []string{
		"proto_version '1'",
		fmt.Sprintf("publication_names '%s'", rc.config.PublicationName),
	}

	err := pglogrepl.StartReplication(
		ctx,
		rc.conn,
		rc.config.SlotName,
		startLSN,
		pglogrepl.StartReplicationOptions{
			PluginArgs: pluginArguments,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	return nil
}

func (rc *ReplicationClient) ReceiveMessage(ctx context.Context) error {
	if rc.conn == nil {
		return fmt.Errorf("not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	msg, err := rc.conn.ReceiveMessage(ctx)
	if err != nil {
		if pgconn.Timeout(err) {
			return nil
		}
		return fmt.Errorf("receive message failed: %w", err)
	}

	switch msg := msg.(type) {
	case *pgproto3.CopyData:
		return rc.handleCopyData(msg.Data)
	default:
		return nil
	}
}

func (rc *ReplicationClient) handleCopyData(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	switch data[0] {
	case pglogrepl.PrimaryKeepaliveMessageByteID:
		return rc.handleKeepalive(data[1:])
	case pglogrepl.XLogDataByteID:
		return rc.handleXLogData(data[1:])
	}

	return nil
}

func (rc *ReplicationClient) handleKeepalive(data []byte) error {
	pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(data)
	if err != nil {
		return fmt.Errorf("failed to parse keepalive: %w", err)
	}

	if pkm.ReplyRequested {
		return rc.SendStandbyStatusUpdate(context.Background(), pkm.ServerWALEnd)
	}

	return nil
}

func (rc *ReplicationClient) handleXLogData(data []byte) error {
	xld, err := pglogrepl.ParseXLogData(data)
	if err != nil {
		return fmt.Errorf("failed to parse xlog data: %w", err)
	}

	return rc.processWALData(xld.WALData)
}

func (rc *ReplicationClient) processWALData(walData []byte) error {
	logicalMsg, err := pglogrepl.Parse(walData)
	if err != nil {
		return fmt.Errorf("failed to parse logical replication message: %w", err)
	}

	switch msg := logicalMsg.(type) {
	case *pglogrepl.RelationMessage:
		rc.relations[msg.RelationID] = msg

	case *pglogrepl.InsertMessage:
		return rc.handleInsert(msg)

	case *pglogrepl.UpdateMessage:
		return rc.handleUpdate(msg)

	case *pglogrepl.DeleteMessage:
		return rc.handleDelete(msg)
	}

	return nil
}

func (rc *ReplicationClient) SendStandbyStatusUpdate(ctx context.Context, lsn pglogrepl.LSN) error {
	if rc.conn == nil {
		return fmt.Errorf("not connected")
	}

	status := pglogrepl.StandbyStatusUpdate{
		WALWritePosition: lsn,
	}

	return pglogrepl.SendStandbyStatusUpdate(ctx, rc.conn, status)
}

func (rc *ReplicationClient) Close(ctx context.Context) error {
	if rc.conn != nil {
		return rc.conn.Close(ctx)
	}
	return nil
}

func (rc *ReplicationClient) handleInsert(msg *pglogrepl.InsertMessage) error {
	rel, ok := rc.relations[msg.RelationID]
	if !ok {
		return fmt.Errorf("unknown relation ID: %d", msg.RelationID)
	}

	values := rc.tupleToMap(rel, msg.Tuple)

	event := &ChangeEvent{
		TableName:  rel.RelationName,
		Operation:  OperationInsert,
		Timestamp:  time.Now(),
		NewData:    values,
		PrimaryKey: rc.extractPrimaryKey(rel, values),
	}

	if rc.handler != nil {
		return rc.handler.HandleChange(event)
	}

	return nil
}

func (rc *ReplicationClient) handleUpdate(msg *pglogrepl.UpdateMessage) error {
	rel, ok := rc.relations[msg.RelationID]
	if !ok {
		return fmt.Errorf("unknown relation ID: %d", msg.RelationID)
	}

	newValues := rc.tupleToMap(rel, msg.NewTuple)
	var oldValues map[string]interface{}
	if msg.OldTuple != nil {
		oldValues = rc.tupleToMap(rel, msg.OldTuple)
	}

	event := &ChangeEvent{
		TableName:  rel.RelationName,
		Operation:  OperationUpdate,
		Timestamp:  time.Now(),
		NewData:    newValues,
		OldData:    oldValues,
		PrimaryKey: rc.extractPrimaryKey(rel, newValues),
	}

	if rc.handler != nil {
		return rc.handler.HandleChange(event)
	}

	return nil
}

func (rc *ReplicationClient) handleDelete(msg *pglogrepl.DeleteMessage) error {
	rel, ok := rc.relations[msg.RelationID]
	if !ok {
		return fmt.Errorf("unknown relation ID: %d", msg.RelationID)
	}

	var values map[string]interface{}
	if msg.OldTuple != nil {
		values = rc.tupleToMap(rel, msg.OldTuple)
	}

	event := &ChangeEvent{
		TableName:  rel.RelationName,
		Operation:  OperationDelete,
		Timestamp:  time.Now(),
		OldData:    values,
		PrimaryKey: rc.extractPrimaryKey(rel, values),
	}

	if rc.handler != nil {
		return rc.handler.HandleChange(event)
	}

	return nil
}

func (rc *ReplicationClient) tupleToMap(rel *pglogrepl.RelationMessage, tuple *pglogrepl.TupleData) map[string]interface{} {
	values := make(map[string]interface{})

	for i, col := range tuple.Columns {
		colName := rel.Columns[i].Name

		switch col.DataType {
		case 'n':
			values[colName] = nil
		case 't':
			values[colName] = string(col.Data)
		}
	}

	return values
}

func (rc *ReplicationClient) extractPrimaryKey(rel *pglogrepl.RelationMessage, values map[string]interface{}) map[string]interface{} {
	pk := make(map[string]interface{})

	for _, col := range rel.Columns {
		if col.Flags == 1 {
			if val, ok := values[col.Name]; ok {
				pk[col.Name] = val
			}
		}
	}

	return pk
}
