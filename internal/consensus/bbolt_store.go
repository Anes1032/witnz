package consensus

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/hashicorp/raft"
	bolt "go.etcd.io/bbolt"
)

const (
	logsBucket   = "logs"
	stableBucket = "stable"
)

// BoltStore implements both LogStore and StableStore interfaces using bbolt
type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore creates a new BoltStore
func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	store := &BoltStore{db: db}

	// Initialize buckets
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return store, nil
}

func (b *BoltStore) initialize() error {
	return b.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(logsBucket)); err != nil {
			return fmt.Errorf("failed to create logs bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(stableBucket)); err != nil {
			return fmt.Errorf("failed to create stable bucket: %w", err)
		}
		return nil
	})
}

// Close closes the database
func (b *BoltStore) Close() error {
	return b.db.Close()
}

// LogStore implementation

// FirstIndex returns the first index written. 0 for no entries.
func (b *BoltStore) FirstIndex() (uint64, error) {
	var firstIndex uint64
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(logsBucket))
		cursor := bucket.Cursor()
		k, _ := cursor.First()
		if k == nil {
			return nil
		}
		firstIndex = binary.BigEndian.Uint64(k)
		return nil
	})
	return firstIndex, err
}

// LastIndex returns the last index written. 0 for no entries.
func (b *BoltStore) LastIndex() (uint64, error) {
	var lastIndex uint64
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(logsBucket))
		cursor := bucket.Cursor()
		k, _ := cursor.Last()
		if k == nil {
			return nil
		}
		lastIndex = binary.BigEndian.Uint64(k)
		return nil
	})
	return lastIndex, err
}

// GetLog gets a log entry at a given index.
func (b *BoltStore) GetLog(index uint64, log *raft.Log) error {
	return b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(logsBucket))
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, index)
		val := bucket.Get(key)
		if val == nil {
			return raft.ErrLogNotFound
		}
		return decodeLog(val, log)
	})
}

// StoreLog stores a log entry.
func (b *BoltStore) StoreLog(log *raft.Log) error {
	return b.StoreLogs([]*raft.Log{log})
}

// StoreLogs stores multiple log entries.
func (b *BoltStore) StoreLogs(logs []*raft.Log) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(logsBucket))
		for _, log := range logs {
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, log.Index)
			val, err := encodeLog(log)
			if err != nil {
				return fmt.Errorf("failed to encode log: %w", err)
			}
			if err := bucket.Put(key, val); err != nil {
				return fmt.Errorf("failed to store log: %w", err)
			}
		}
		return nil
	})
}

// DeleteRange deletes a range of log entries. The range is inclusive.
func (b *BoltStore) DeleteRange(min, max uint64) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(logsBucket))
		cursor := bucket.Cursor()
		minKey := make([]byte, 8)
		binary.BigEndian.PutUint64(minKey, min)
		maxKey := make([]byte, 8)
		binary.BigEndian.PutUint64(maxKey, max)

		for k, _ := cursor.Seek(minKey); k != nil && binary.BigEndian.Uint64(k) <= max; k, _ = cursor.Next() {
			if err := bucket.Delete(k); err != nil {
				return fmt.Errorf("failed to delete log entry: %w", err)
			}
		}
		return nil
	})
}

// StableStore implementation

// Set stores a key-value pair.
func (b *BoltStore) Set(key []byte, val []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(stableBucket))
		return bucket.Put(key, val)
	})
}

// Get returns the value for key, or an empty byte slice if key was not found.
func (b *BoltStore) Get(key []byte) ([]byte, error) {
	var val []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(stableBucket))
		v := bucket.Get(key)
		if v != nil {
			val = make([]byte, len(v))
			copy(val, v)
		}
		return nil
	})
	return val, err
}

// SetUint64 stores a uint64 value.
func (b *BoltStore) SetUint64(key []byte, val uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	return b.Set(key, buf)
}

// GetUint64 returns the uint64 value for key, or 0 if key was not found.
func (b *BoltStore) GetUint64(key []byte) (uint64, error) {
	val, err := b.Get(key)
	if err != nil {
		return 0, err
	}
	if len(val) == 0 {
		return 0, nil
	}
	if len(val) != 8 {
		return 0, fmt.Errorf("invalid uint64 value length: %d", len(val))
	}
	return binary.BigEndian.Uint64(val), nil
}

// encodeLog encodes a raft.Log into bytes
func encodeLog(log *raft.Log) ([]byte, error) {
	// Simple encoding: index (8) + term (8) + type (1) + data length (4) + data
	buf := make([]byte, 8+8+1+4+len(log.Data))
	offset := 0

	binary.BigEndian.PutUint64(buf[offset:], log.Index)
	offset += 8

	binary.BigEndian.PutUint64(buf[offset:], log.Term)
	offset += 8

	buf[offset] = byte(log.Type)
	offset += 1

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(log.Data)))
	offset += 4

	copy(buf[offset:], log.Data)

	return buf, nil
}

// decodeLog decodes bytes into a raft.Log
func decodeLog(data []byte, log *raft.Log) error {
	if len(data) < 21 {
		return fmt.Errorf("log data too short: %d bytes", len(data))
	}

	offset := 0
	log.Index = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	log.Term = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	log.Type = raft.LogType(data[offset])
	offset += 1

	dataLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	if len(data) < offset+int(dataLen) {
		return fmt.Errorf("log data incomplete: expected %d bytes, got %d", offset+int(dataLen), len(data))
	}

	log.Data = make([]byte, dataLen)
	copy(log.Data, data[offset:offset+int(dataLen)])

	return nil
}
