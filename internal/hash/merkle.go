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
	leafDataMap map[string]string
	root        *MerkleNode
}

func NewMerkleTreeBuilder() *MerkleTreeBuilder {
	return &MerkleTreeBuilder{
		leaves:      make([]string, 0),
		leafDataMap: make(map[string]string),
	}
}

func (mtb *MerkleTreeBuilder) AddLeaf(recordID string, data map[string]interface{}) error {
	hash, err := calculateDataHash(data)
	if err != nil {
		return err
	}
	mtb.leaves = append(mtb.leaves, hash)
	mtb.leafDataMap[recordID] = hash
	return nil
}

func (mtb *MerkleTreeBuilder) AddLeafHash(recordID string, hash string) {
	mtb.leaves = append(mtb.leaves, hash)
	mtb.leafDataMap[recordID] = hash
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

func calculateDataHash(data map[string]interface{}) (string, error) {
	normalized := normalizeForHash(data)
	jsonData, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}

func normalizeForHash(data map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range data {
		if k == "created_at" || k == "updated_at" {
			continue
		}
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}
