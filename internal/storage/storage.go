package storage

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	HashChainBucket = []byte("hashchain")
	MetadataBucket  = []byte("metadata")
)

type Storage struct {
	db *bolt.DB
}

type HashEntry struct {
	TableName     string    `json:"table_name"`
	SequenceNum   uint64    `json:"sequence_num"`
	Hash          string    `json:"hash"`
	PreviousHash  string    `json:"previous_hash"`
	DataHash      string    `json:"data_hash"`
	Timestamp     time.Time `json:"timestamp"`
	OperationType string    `json:"operation_type"`
	RecordID      string    `json:"record_id"`
}

func New(path string) (*Storage, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{HashChainBucket, MetadataBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) SaveHashEntry(entry *HashEntry) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(HashChainBucket)

		key := fmt.Sprintf("%s:%d", entry.TableName, entry.SequenceNum)

		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal hash entry: %w", err)
		}

		return bucket.Put([]byte(key), data)
	})
}

func (s *Storage) GetHashEntry(tableName string, seqNum uint64) (*HashEntry, error) {
	var entry HashEntry

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(HashChainBucket)

		key := fmt.Sprintf("%s:%d", tableName, seqNum)
		data := bucket.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("hash entry not found")
		}

		return json.Unmarshal(data, &entry)
	})

	if err != nil {
		return nil, err
	}

	return &entry, nil
}

func (s *Storage) GetLatestHashEntry(tableName string) (*HashEntry, error) {
	var latestEntry *HashEntry

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(HashChainBucket)
		cursor := bucket.Cursor()

		prefix := []byte(tableName + ":")

		for k, v := cursor.Seek(prefix); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == string(prefix); k, v = cursor.Next() {
			var entry HashEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}
			latestEntry = &entry
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if latestEntry == nil {
		return nil, fmt.Errorf("no hash entries found for table %s", tableName)
	}

	return latestEntry, nil
}

func (s *Storage) SetMetadata(key, value string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(MetadataBucket)
		return bucket.Put([]byte(key), []byte(value))
	})
}

func (s *Storage) GetMetadata(key string) (string, error) {
	var value string

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(MetadataBucket)
		data := bucket.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("metadata key not found: %s", key)
		}
		value = string(data)
		return nil
	})

	return value, err
}
