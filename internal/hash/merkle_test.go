package hash

import (
	"testing"
)

func TestMerkleTreeBuilder_Basic(t *testing.T) {
	builder := NewMerkleTreeBuilder()

	record1 := map[string]interface{}{
		"id":   1,
		"name": "Alice",
		"age":  30,
	}
	record2 := map[string]interface{}{
		"id":   2,
		"name": "Bob",
		"age":  25,
	}

	if err := builder.AddLeaf("1", record1); err != nil {
		t.Fatalf("Failed to add leaf 1: %v", err)
	}

	if err := builder.AddLeaf("2", record2); err != nil {
		t.Fatalf("Failed to add leaf 2: %v", err)
	}

	if err := builder.Build(); err != nil {
		t.Fatalf("Failed to build tree: %v", err)
	}

	root := builder.GetRoot()
	if root == "" {
		t.Fatal("Root hash is empty")
	}

	t.Logf("Merkle Root: %s", root)
}

func TestMerkleTreeBuilder_SameDataSameRoot(t *testing.T) {
	record1 := map[string]interface{}{
		"id":   1,
		"name": "Alice",
	}
	record2 := map[string]interface{}{
		"id":   2,
		"name": "Bob",
	}

	builder1 := NewMerkleTreeBuilder()
	builder1.AddLeaf("1", record1)
	builder1.AddLeaf("2", record2)
	builder1.Build()
	root1 := builder1.GetRoot()

	builder2 := NewMerkleTreeBuilder()
	builder2.AddLeaf("1", record1)
	builder2.AddLeaf("2", record2)
	builder2.Build()
	root2 := builder2.GetRoot()

	if root1 != root2 {
		t.Errorf("Same data should produce same root. Got %s and %s", root1, root2)
	}
}

func TestMerkleTreeBuilder_DifferentDataDifferentRoot(t *testing.T) {
	record1 := map[string]interface{}{
		"id":   1,
		"name": "Alice",
	}
	record2 := map[string]interface{}{
		"id":   2,
		"name": "Bob",
	}
	record3 := map[string]interface{}{
		"id":   2,
		"name": "Charlie",
	}

	builder1 := NewMerkleTreeBuilder()
	builder1.AddLeaf("1", record1)
	builder1.AddLeaf("2", record2)
	builder1.Build()
	root1 := builder1.GetRoot()

	builder2 := NewMerkleTreeBuilder()
	builder2.AddLeaf("1", record1)
	builder2.AddLeaf("2", record3)
	builder2.Build()
	root2 := builder2.GetRoot()

	if root1 == root2 {
		t.Error("Different data should produce different roots")
	}
}

func TestMerkleProof_Verify(t *testing.T) {
	builder := NewMerkleTreeBuilder()

	for i := 1; i <= 4; i++ {
		record := map[string]interface{}{
			"id":    i,
			"value": i * 10,
		}
		builder.AddLeaf(string(rune('0'+i)), record)
	}

	if err := builder.Build(); err != nil {
		t.Fatalf("Failed to build tree: %v", err)
	}

	root := builder.GetRoot()

	proof, err := builder.GetProof("1")
	if err != nil {
		t.Fatalf("Failed to get proof: %v", err)
	}

	if !proof.Verify(root) {
		t.Error("Valid proof failed verification")
	}

	wrongRoot := "0000000000000000000000000000000000000000000000000000000000000000"
	if proof.Verify(wrongRoot) {
		t.Error("Proof should not verify against wrong root")
	}
}

func TestNormalizeForHash_IgnoresTimestamps(t *testing.T) {
	data1 := map[string]interface{}{
		"id":         1,
		"name":       "Alice",
		"created_at": "2024-01-01",
		"updated_at": "2024-01-02",
	}

	data2 := map[string]interface{}{
		"id":         1,
		"name":       "Alice",
		"created_at": "2024-12-12",
		"updated_at": "2024-12-13",
	}

	hash1, err := calculateDataHash(data1)
	if err != nil {
		t.Fatalf("Failed to calculate hash1: %v", err)
	}

	hash2, err := calculateDataHash(data2)
	if err != nil {
		t.Fatalf("Failed to calculate hash2: %v", err)
	}

	if hash1 != hash2 {
		t.Error("Hashes should be same when only timestamps differ")
	}
}
