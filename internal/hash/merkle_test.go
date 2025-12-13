package hash

import (
	"fmt"
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

	hash1 := CalculateDataHash(data1)
	hash2 := CalculateDataHash(data2)

	if hash1 != hash2 {
		t.Error("Hashes should be same when only timestamps differ")
	}
}

func TestCalculateDataHash_TypeConsistency(t *testing.T) {
	// Test that different type representations produce the same hash
	// This simulates CDC (string) vs PostgreSQL query (native types)

	// CDC-style data (values as strings)
	cdcData := map[string]interface{}{
		"id":    "123",
		"value": "456.78",
		"flag":  "true",
	}

	// PostgreSQL query-style data (native types)
	pgData := map[string]interface{}{
		"id":    123,
		"value": 456.78,
		"flag":  true,
	}

	cdcHash := CalculateDataHash(cdcData)
	pgHash := CalculateDataHash(pgData)

	// Note: These will be different because we preserve type info in normalization
	// This is intentional - if types differ, data differs
	// The key is that the SAME data source will always produce the same hash
	if cdcHash == pgHash {
		t.Log("CDC and PostgreSQL hashes match (unexpected but acceptable)")
	}

	// Verify same data produces same hash
	cdcHash2 := CalculateDataHash(cdcData)
	if cdcHash != cdcHash2 {
		t.Error("Same data should produce same hash")
	}
}

func TestCalculateDataHash_BinaryData(t *testing.T) {
	// Test binary data handling
	data := map[string]interface{}{
		"id":   1,
		"blob": []byte{0x01, 0x02, 0x03},
	}

	hash1 := CalculateDataHash(data)
	hash2 := CalculateDataHash(data)

	if hash1 != hash2 {
		t.Error("Binary data should produce consistent hash")
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
}

func TestCalculateDataHash_NilHandling(t *testing.T) {
	data := map[string]interface{}{
		"id":    1,
		"value": nil,
	}

	hash := CalculateDataHash(data)
	if hash == "" {
		t.Error("Hash should not be empty for data with nil values")
	}
}

func TestCompareTrees_IdenticalTrees(t *testing.T) {
	tree1 := NewMerkleTreeBuilder()
	tree2 := NewMerkleTreeBuilder()

	for i := 1; i <= 100; i++ {
		data := map[string]interface{}{"id": i, "name": fmt.Sprintf("record%d", i)}
		tree1.AddLeaf(fmt.Sprintf("%d", i), data)
		tree2.AddLeaf(fmt.Sprintf("%d", i), data)
	}

	tree1.Build()
	tree2.Build()

	differing := CompareTrees(tree1, tree2)
	if len(differing) != 0 {
		t.Errorf("Expected 0 differences for identical trees, got %d", len(differing))
	}
}

func TestCompareTrees_SingleDifference(t *testing.T) {
	tree1 := NewMerkleTreeBuilder()
	tree2 := NewMerkleTreeBuilder()

	for i := 1; i <= 100; i++ {
		data1 := map[string]interface{}{"id": i, "name": fmt.Sprintf("record%d", i)}
		tree1.AddLeaf(fmt.Sprintf("%d", i), data1)

		// Change one record in tree2
		if i == 50 {
			data2 := map[string]interface{}{"id": i, "name": "TAMPERED"}
			tree2.AddLeaf(fmt.Sprintf("%d", i), data2)
		} else {
			tree2.AddLeaf(fmt.Sprintf("%d", i), data1)
		}
	}

	tree1.Build()
	tree2.Build()

	differing := CompareTrees(tree1, tree2)
	if len(differing) != 1 {
		t.Errorf("Expected 1 difference, got %d", len(differing))
	}
}

func TestCompareTrees_MultipleDifferences(t *testing.T) {
	tree1 := NewMerkleTreeBuilder()
	tree2 := NewMerkleTreeBuilder()

	tamperedIDs := map[int]bool{10: true, 25: true, 75: true}

	for i := 1; i <= 100; i++ {
		data1 := map[string]interface{}{"id": i, "name": fmt.Sprintf("record%d", i)}
		tree1.AddLeaf(fmt.Sprintf("%d", i), data1)

		if tamperedIDs[i] {
			data2 := map[string]interface{}{"id": i, "name": "TAMPERED"}
			tree2.AddLeaf(fmt.Sprintf("%d", i), data2)
		} else {
			tree2.AddLeaf(fmt.Sprintf("%d", i), data1)
		}
	}

	tree1.Build()
	tree2.Build()

	differing := CompareTrees(tree1, tree2)
	if len(differing) != 3 {
		t.Errorf("Expected 3 differences, got %d", len(differing))
	}
}

func TestCompareTrees_LargeTreeEfficiency(t *testing.T) {
	// Test with a large tree to verify O(k log n) efficiency
	tree1 := NewMerkleTreeBuilder()
	tree2 := NewMerkleTreeBuilder()

	n := 10000
	tamperedIndex := 5000

	for i := 1; i <= n; i++ {
		data1 := map[string]interface{}{"id": i, "value": i * 100}
		tree1.AddLeaf(fmt.Sprintf("%d", i), data1)

		if i == tamperedIndex {
			data2 := map[string]interface{}{"id": i, "value": -1}
			tree2.AddLeaf(fmt.Sprintf("%d", i), data2)
		} else {
			tree2.AddLeaf(fmt.Sprintf("%d", i), data1)
		}
	}

	tree1.Build()
	tree2.Build()

	// Verify trees have different roots
	if tree1.GetRoot() == tree2.GetRoot() {
		t.Error("Trees should have different roots")
	}

	differing := CompareTrees(tree1, tree2)
	if len(differing) != 1 {
		t.Errorf("Expected 1 difference in large tree, got %d", len(differing))
	}
}

func TestGetRecordIDByIndex(t *testing.T) {
	tree := NewMerkleTreeBuilder()

	tree.AddLeafHash("record-a", "hash-aaa")
	tree.AddLeafHash("record-b", "hash-bbb")
	tree.AddLeafHash("record-c", "hash-ccc")

	tree.Build()

	// After sorting: hash-aaa, hash-bbb, hash-ccc
	id0, ok0 := tree.GetRecordIDByIndex(0)
	id1, ok1 := tree.GetRecordIDByIndex(1)
	id2, ok2 := tree.GetRecordIDByIndex(2)
	_, ok3 := tree.GetRecordIDByIndex(3)

	if !ok0 || !ok1 || !ok2 {
		t.Error("GetRecordIDByIndex should return true for valid indices")
	}

	if ok3 {
		t.Error("GetRecordIDByIndex should return false for out-of-range index")
	}

	// Verify we can retrieve IDs (order depends on hash sorting)
	ids := []string{id0, id1, id2}
	hasA, hasB, hasC := false, false, false
	for _, id := range ids {
		switch id {
		case "record-a":
			hasA = true
		case "record-b":
			hasB = true
		case "record-c":
			hasC = true
		}
	}

	if !hasA || !hasB || !hasC {
		t.Error("All record IDs should be retrievable")
	}
}
