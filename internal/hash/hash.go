package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func Calculate(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}

func CalculateString(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
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
