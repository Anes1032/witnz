package hash

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/zeebo/blake3"
	"golang.org/x/crypto/blake2b"
)

var (
	globalHasher Hasher
	hasherMu     sync.RWMutex
)

type Hasher interface {
	Hash(data []byte) string
	Name() string
}

func Initialize(algorithm string) error {
	var h Hasher
	switch algorithm {
	case "xxhash64":
		h = &xxHash64Hasher{}
	case "xxhash128":
		h = &xxHash128Hasher{}
	case "sha256":
		h = &sha256Hasher{}
	case "blake2b_256":
		h = &blake2b256Hasher{}
	case "blake3":
		h = &blake3Hasher{}
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}

	hasherMu.Lock()
	globalHasher = h
	hasherMu.Unlock()
	return nil
}

func GetHasher() Hasher {
	hasherMu.RLock()
	defer hasherMu.RUnlock()
	if globalHasher == nil {
		return &sha256Hasher{}
	}
	return globalHasher
}

type xxHash64Hasher struct{}

func (h *xxHash64Hasher) Hash(data []byte) string {
	hash := xxhash.Sum64(data)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, hash)
	return hex.EncodeToString(buf)
}

func (h *xxHash64Hasher) Name() string {
	return "xxhash64"
}

type xxHash128Hasher struct{}

func (h *xxHash128Hasher) Hash(data []byte) string {
	digest := xxhash.New()
	digest.Write(data)
	hash := digest.Sum(nil)
	digest2 := xxhash.New()
	digest2.Write(append([]byte{0x01}, data...))
	hash2 := digest2.Sum(nil)
	return hex.EncodeToString(hash) + hex.EncodeToString(hash2)
}

func (h *xxHash128Hasher) Name() string {
	return "xxhash128"
}

type sha256Hasher struct{}

func (h *sha256Hasher) Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (h *sha256Hasher) Name() string {
	return "sha256"
}

type blake2b256Hasher struct{}

func (h *blake2b256Hasher) Hash(data []byte) string {
	hash := blake2b.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (h *blake2b256Hasher) Name() string {
	return "blake2b_256"
}

type blake3Hasher struct{}

func (h *blake3Hasher) Hash(data []byte) string {
	hash := blake3.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (h *blake3Hasher) Name() string {
	return "blake3"
}

func Calculate(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	hasher := GetHasher()
	return hasher.Hash(jsonData), nil
}

func CalculateString(data string) string {
	hasher := GetHasher()
	return hasher.Hash([]byte(data))
}

type HashChain struct {
	previousHash string
}

func NewHashChain(initialHash string) *HashChain {
	return &HashChain{
		previousHash: initialHash,
	}
}

func (hc *HashChain) Add(data interface{}) (string, error) {
	dataHash, err := Calculate(data)
	if err != nil {
		return "", err
	}

	combined := hc.previousHash + dataHash
	newHash := CalculateString(combined)

	hc.previousHash = newHash

	return newHash, nil
}

func (hc *HashChain) GetPreviousHash() string {
	return hc.previousHash
}

func (hc *HashChain) SetPreviousHash(hash string) {
	hc.previousHash = hash
}

type MerkleTree struct {
	leaves []string
}

func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		leaves: make([]string, 0),
	}
}

func (mt *MerkleTree) AddLeaf(data interface{}) error {
	hash, err := Calculate(data)
	if err != nil {
		return err
	}
	mt.leaves = append(mt.leaves, hash)
	return nil
}

func (mt *MerkleTree) AddLeafHash(hash string) {
	mt.leaves = append(mt.leaves, hash)
}

func (mt *MerkleTree) GetRoot() string {
	if len(mt.leaves) == 0 {
		return ""
	}

	sortedLeaves := make([]string, len(mt.leaves))
	copy(sortedLeaves, mt.leaves)
	sort.Strings(sortedLeaves)

	return mt.calculateRoot(sortedLeaves)
}

func (mt *MerkleTree) calculateRoot(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}

	if len(hashes) == 1 {
		return hashes[0]
	}

	var nextLevel []string

	for i := 0; i < len(hashes); i += 2 {
		var combined string
		if i+1 < len(hashes) {
			combined = hashes[i] + hashes[i+1]
		} else {
			combined = hashes[i] + hashes[i]
		}
		nextLevel = append(nextLevel, CalculateString(combined))
	}

	return mt.calculateRoot(nextLevel)
}

func (mt *MerkleTree) Reset() {
	mt.leaves = make([]string, 0)
}

func (mt *MerkleTree) LeafCount() int {
	return len(mt.leaves)
}
