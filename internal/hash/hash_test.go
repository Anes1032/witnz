package hash

import (
	"testing"
)

func TestCalculate(t *testing.T) {
	data := map[string]interface{}{
		"id":   1,
		"name": "test",
	}

	hash1, err := Calculate(data)
	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	hash2, err := Calculate(data)
	if err != nil {
		t.Fatalf("Calculate failed: %v", err)
	}

	if hash1 != hash2 {
		t.Error("Same data should produce same hash")
	}

	if len(hash1) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash1))
	}
}

func TestCalculateString(t *testing.T) {
	str := "test string"

	hash1 := CalculateString(str)
	hash2 := CalculateString(str)

	if hash1 != hash2 {
		t.Error("Same string should produce same hash")
	}

	if len(hash1) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash1))
	}
}

func TestHashChain(t *testing.T) {
	hc := NewHashChain("genesis")

	data1 := "first block"
	hash1, err := hc.Add(data1)
	if err != nil {
		t.Fatalf("Failed to add to chain: %v", err)
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	data2 := "second block"
	hash2, err := hc.Add(data2)
	if err != nil {
		t.Fatalf("Failed to add to chain: %v", err)
	}

	if hash1 == hash2 {
		t.Error("Different blocks should produce different hashes")
	}

	if hc.GetPreviousHash() != hash2 {
		t.Error("Previous hash should be updated to latest hash")
	}
}

func TestHashChainSetPreviousHash(t *testing.T) {
	hc := NewHashChain("initial")

	newHash := "new_hash_value"
	hc.SetPreviousHash(newHash)

	if hc.GetPreviousHash() != newHash {
		t.Errorf("Expected previous hash %s, got %s", newHash, hc.GetPreviousHash())
	}
}

func TestMerkleTree(t *testing.T) {
	mt := NewMerkleTree()

	if mt.LeafCount() != 0 {
		t.Error("New tree should have 0 leaves")
	}

	data1 := map[string]interface{}{"id": 1, "value": "a"}
	data2 := map[string]interface{}{"id": 2, "value": "b"}
	data3 := map[string]interface{}{"id": 3, "value": "c"}

	if err := mt.AddLeaf(data1); err != nil {
		t.Fatalf("Failed to add leaf: %v", err)
	}
	if err := mt.AddLeaf(data2); err != nil {
		t.Fatalf("Failed to add leaf: %v", err)
	}
	if err := mt.AddLeaf(data3); err != nil {
		t.Fatalf("Failed to add leaf: %v", err)
	}

	if mt.LeafCount() != 3 {
		t.Errorf("Expected 3 leaves, got %d", mt.LeafCount())
	}

	root1 := mt.GetRoot()
	if root1 == "" {
		t.Error("Root should not be empty")
	}

	mt2 := NewMerkleTree()
	mt2.AddLeaf(data1)
	mt2.AddLeaf(data2)
	mt2.AddLeaf(data3)
	root2 := mt2.GetRoot()

	if root1 != root2 {
		t.Error("Same leaves should produce same root")
	}

	mt3 := NewMerkleTree()
	mt3.AddLeaf(data1)
	mt3.AddLeaf(data3)
	mt3.AddLeaf(data2)
	root3 := mt3.GetRoot()

	if root1 != root3 {
		t.Error("Merkle tree should be order-independent (sorted internally)")
	}
}

func TestMerkleTreeAddLeafHash(t *testing.T) {
	mt := NewMerkleTree()

	hash1 := "abcd1234"
	hash2 := "efgh5678"

	mt.AddLeafHash(hash1)
	mt.AddLeafHash(hash2)

	if mt.LeafCount() != 2 {
		t.Errorf("Expected 2 leaves, got %d", mt.LeafCount())
	}

	root := mt.GetRoot()
	if root == "" {
		t.Error("Root should not be empty")
	}
}

func TestMerkleTreeReset(t *testing.T) {
	mt := NewMerkleTree()

	mt.AddLeaf("data1")
	mt.AddLeaf("data2")

	if mt.LeafCount() != 2 {
		t.Error("Expected 2 leaves before reset")
	}

	mt.Reset()

	if mt.LeafCount() != 0 {
		t.Error("Expected 0 leaves after reset")
	}

	if mt.GetRoot() != "" {
		t.Error("Root should be empty after reset")
	}
}

func TestMerkleTreeSingleLeaf(t *testing.T) {
	mt := NewMerkleTree()
	mt.AddLeaf("single")

	root := mt.GetRoot()
	if root == "" {
		t.Error("Root should not be empty for single leaf")
	}
}

func TestMerkleTreeEmpty(t *testing.T) {
	mt := NewMerkleTree()

	root := mt.GetRoot()
	if root != "" {
		t.Error("Root should be empty for empty tree")
	}
}
