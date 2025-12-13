package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
}

type MerkleProof struct {
	LeafHash   string
	LeafIndex  int
	Siblings   []string
	Directions []bool
}

type MerkleTreeBuilder struct {
	leaves      []string
	leafDataMap map[string]string // recordID -> hash
	hashToID    map[string]string // hash -> recordID (reverse lookup)
	root        *MerkleNode
}

func NewMerkleTreeBuilder() *MerkleTreeBuilder {
	return &MerkleTreeBuilder{
		leaves:      make([]string, 0),
		leafDataMap: make(map[string]string),
		hashToID:    make(map[string]string),
	}
}

func (mtb *MerkleTreeBuilder) AddLeaf(recordID string, data map[string]interface{}) error {
	hash := CalculateDataHash(data)
	mtb.leaves = append(mtb.leaves, hash)
	mtb.leafDataMap[recordID] = hash
	mtb.hashToID[hash] = recordID
	return nil
}

func (mtb *MerkleTreeBuilder) AddLeafHash(recordID string, hash string) {
	mtb.leaves = append(mtb.leaves, hash)
	mtb.leafDataMap[recordID] = hash
	mtb.hashToID[hash] = recordID
}

// GetRecordIDByHash returns the recordID associated with a leaf hash
func (mtb *MerkleTreeBuilder) GetRecordIDByHash(hash string) (string, bool) {
	id, exists := mtb.hashToID[hash]
	return id, exists
}

// GetRecordIDByIndex returns the recordID for a leaf at the given sorted index
func (mtb *MerkleTreeBuilder) GetRecordIDByIndex(index int) (string, bool) {
	sortedLeaves := mtb.GetSortedLeaves()
	if index < 0 || index >= len(sortedLeaves) {
		return "", false
	}
	hash := sortedLeaves[index]
	return mtb.GetRecordIDByHash(hash)
}

func (mtb *MerkleTreeBuilder) Build() error {
	if len(mtb.leaves) == 0 {
		return fmt.Errorf("no leaves to build tree")
	}

	sortedLeaves := make([]string, len(mtb.leaves))
	copy(sortedLeaves, mtb.leaves)
	sort.Strings(sortedLeaves)

	nodes := make([]*MerkleNode, len(sortedLeaves))
	for i, leaf := range sortedLeaves {
		nodes[i] = &MerkleNode{Hash: leaf}
	}

	mtb.root = mtb.buildTree(nodes)
	return nil
}

func (mtb *MerkleTreeBuilder) buildTree(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	var nextLevel []*MerkleNode

	for i := 0; i < len(nodes); i += 2 {
		left := nodes[i]
		var right *MerkleNode

		if i+1 < len(nodes) {
			right = nodes[i+1]
		} else {
			right = nodes[i]
		}

		combined := left.Hash + right.Hash
		hash := sha256.Sum256([]byte(combined))
		parent := &MerkleNode{
			Hash:  hex.EncodeToString(hash[:]),
			Left:  left,
			Right: right,
		}

		nextLevel = append(nextLevel, parent)
	}

	return mtb.buildTree(nextLevel)
}

func (mtb *MerkleTreeBuilder) GetRoot() string {
	if mtb.root == nil {
		return ""
	}
	return mtb.root.Hash
}

func (mtb *MerkleTreeBuilder) GetProof(recordID string) (*MerkleProof, error) {
	leafHash, exists := mtb.leafDataMap[recordID]
	if !exists {
		return nil, fmt.Errorf("record not found in tree: %s", recordID)
	}

	if mtb.root == nil {
		return nil, fmt.Errorf("tree not built")
	}

	sortedLeaves := make([]string, len(mtb.leaves))
	copy(sortedLeaves, mtb.leaves)
	sort.Strings(sortedLeaves)

	leafIndex := -1
	for i, leaf := range sortedLeaves {
		if leaf == leafHash {
			leafIndex = i
			break
		}
	}

	if leafIndex == -1 {
		return nil, fmt.Errorf("leaf hash not found in sorted leaves")
	}

	proof := &MerkleProof{
		LeafHash:   leafHash,
		LeafIndex:  leafIndex,
		Siblings:   make([]string, 0),
		Directions: make([]bool, 0),
	}

	mtb.buildProof(mtb.root, leafHash, proof, 0, len(sortedLeaves)-1)

	return proof, nil
}

func (mtb *MerkleTreeBuilder) buildProof(node *MerkleNode, targetHash string, proof *MerkleProof, start, end int) bool {
	if node == nil {
		return false
	}

	if node.Left == nil && node.Right == nil {
		return node.Hash == targetHash
	}

	mid := (start + end) / 2

	if mtb.buildProof(node.Left, targetHash, proof, start, mid) {
		if node.Right != nil {
			proof.Siblings = append(proof.Siblings, node.Right.Hash)
			proof.Directions = append(proof.Directions, true)
		}
		return true
	}

	if mtb.buildProof(node.Right, targetHash, proof, mid+1, end) {
		if node.Left != nil {
			proof.Siblings = append(proof.Siblings, node.Left.Hash)
			proof.Directions = append(proof.Directions, false)
		}
		return true
	}

	return false
}

func (mp *MerkleProof) Verify(expectedRoot string) bool {
	currentHash := mp.LeafHash

	for i := 0; i < len(mp.Siblings); i++ {
		sibling := mp.Siblings[i]
		isRight := mp.Directions[i]

		var combined string
		if isRight {
			combined = currentHash + sibling
		} else {
			combined = sibling + currentHash
		}

		hash := sha256.Sum256([]byte(combined))
		currentHash = hex.EncodeToString(hash[:])
	}

	return currentHash == expectedRoot
}

func (mtb *MerkleTreeBuilder) FindTamperedLeaves(storedRoot string) ([]int, error) {
	if mtb.root == nil {
		return nil, fmt.Errorf("tree not built")
	}

	currentRoot := mtb.root.Hash
	if currentRoot == storedRoot {
		return []int{}, nil
	}

	tamperedIndices := make([]int, 0)
	mtb.findTamperedLeavesRecursive(mtb.root, 0, len(mtb.leaves)-1, &tamperedIndices)

	return tamperedIndices, nil
}

func (mtb *MerkleTreeBuilder) findTamperedLeavesRecursive(node *MerkleNode, start, end int, tamperedIndices *[]int) {
	if node == nil {
		return
	}

	if node.Left == nil && node.Right == nil {
		*tamperedIndices = append(*tamperedIndices, start)
		return
	}

	mid := (start + end) / 2

	if node.Left != nil {
		mtb.findTamperedLeavesRecursive(node.Left, start, mid, tamperedIndices)
	}

	if node.Right != nil && node.Right != node.Left {
		mtb.findTamperedLeavesRecursive(node.Right, mid+1, end, tamperedIndices)
	}
}

// GetSortedLeaves returns the leaves in sorted order (as used in the tree)
func (mtb *MerkleTreeBuilder) GetSortedLeaves() []string {
	sortedLeaves := make([]string, len(mtb.leaves))
	copy(sortedLeaves, mtb.leaves)
	sort.Strings(sortedLeaves)
	return sortedLeaves
}

// GetLeafCount returns the number of leaves in the tree
func (mtb *MerkleTreeBuilder) GetLeafCount() int {
	return len(mtb.leaves)
}

// DifferingRecord represents a record that differs between two trees
type DifferingRecord struct {
	RecordID     string
	ExpectedHash string
	ActualHash   string
	Type         string // "modified", "missing_in_actual", "missing_in_expected"
}

// CompareTrees compares two Merkle trees and returns records that differ.
// First checks roots (O(1)), then uses recordID-based comparison for differences.
// Returns nil if both trees are nil, empty slice if identical.
func CompareTrees(expected, actual *MerkleTreeBuilder) []int {
	if expected == nil || actual == nil {
		return nil
	}

	// Quick check: if roots match, trees are identical
	if expected.root != nil && actual.root != nil && expected.root.Hash == actual.root.Hash {
		return []int{}
	}

	// Compare by recordID to find exact differences
	return compareByRecordID(expected, actual)
}

// CompareTreesDetailed returns detailed information about differing records
func CompareTreesDetailed(expected, actual *MerkleTreeBuilder) []DifferingRecord {
	if expected == nil || actual == nil {
		return nil
	}

	// Quick check: if roots match, trees are identical
	if expected.root != nil && actual.root != nil && expected.root.Hash == actual.root.Hash {
		return []DifferingRecord{}
	}

	differing := []DifferingRecord{}

	// Check all records in expected tree
	for recordID, expectedHash := range expected.leafDataMap {
		actualHash, exists := actual.leafDataMap[recordID]
		if !exists {
			differing = append(differing, DifferingRecord{
				RecordID:     recordID,
				ExpectedHash: expectedHash,
				ActualHash:   "",
				Type:         "missing_in_actual",
			})
		} else if expectedHash != actualHash {
			differing = append(differing, DifferingRecord{
				RecordID:     recordID,
				ExpectedHash: expectedHash,
				ActualHash:   actualHash,
				Type:         "modified",
			})
		}
	}

	// Check for records only in actual tree
	for recordID, actualHash := range actual.leafDataMap {
		if _, exists := expected.leafDataMap[recordID]; !exists {
			differing = append(differing, DifferingRecord{
				RecordID:     recordID,
				ExpectedHash: "",
				ActualHash:   actualHash,
				Type:         "missing_in_expected",
			})
		}
	}

	return differing
}

// compareByRecordID compares trees by recordID mapping and returns indices of differing records
func compareByRecordID(expected, actual *MerkleTreeBuilder) []int {
	differing := []int{}
	actualLeaves := actual.GetSortedLeaves()

	// Create a map from hash to index in actual tree
	actualHashToIdx := make(map[string]int)
	for i, h := range actualLeaves {
		actualHashToIdx[h] = i
	}

	// Check each record in expected tree
	for recordID, expectedHash := range expected.leafDataMap {
		actualHash, exists := actual.leafDataMap[recordID]
		if !exists || expectedHash != actualHash {
			// Record is missing or modified - find its index in actual tree
			if actualHash != "" {
				if idx, ok := actualHashToIdx[actualHash]; ok {
					differing = append(differing, idx)
				}
			}
		}
	}

	return differing
}

// CalculateDataHash computes a SHA-256 hash of the given data map.
// It normalizes the data to ensure consistent hashing across different data sources
// (CDC events vs PostgreSQL queries).
func CalculateDataHash(data map[string]interface{}) string {
	normalized := NormalizeForHash(data)
	jsonData, _ := json.Marshal(normalized)
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// NormalizeForHash normalizes data for consistent hash calculation.
// This ensures the same hash is produced regardless of data source (CDC vs DB query).
// Excludes timestamp fields and normalizes type representations.
func NormalizeForHash(data map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range data {
		// Skip timestamp fields that may have format differences
		if k == "created_at" || k == "updated_at" {
			continue
		}
		result[k] = normalizeValue(v)
	}
	return result
}

// normalizeValue converts a value to a consistent string representation
// regardless of whether it came from CDC (text) or PostgreSQL query (native types).
func normalizeValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}

	switch val := v.(type) {
	case []byte:
		// Binary data - use hex encoding for consistency
		return hex.EncodeToString(val)
	case string:
		return val
	default:
		// For all other types, use fmt.Sprintf which handles
		// int, float, bool, etc. consistently
		return fmt.Sprintf("%v", val)
	}
}
