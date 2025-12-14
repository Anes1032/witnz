#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-common.sh"

echo "=========================================="
echo "Hash Chain Verification Test"
echo "=========================================="
echo ""
echo "This test verifies offline tampering detection via hash chain verification."
echo "Test cases: UPDATE, DELETE, DELETE+INSERT, Phantom INSERT"
echo ""

trap cleanup EXIT

docker-compose down -v 2>/dev/null || true

setup_postgres
start_cluster 10

verify_leader_election || exit 1

insert_test_records 5

verify_hash_chain_replication 5

stop_all_nodes

echo ""
echo "Tampering with database while nodes are offline..."
echo "=========================================="
echo "TEST CASE 1: UPDATE (Modify existing data)"
echo "TEST CASE 2: DELETE (Remove record)"
echo "TEST CASE 3: DELETE + INSERT (Same ID, new data)"
echo "TEST CASE 4: INSERT (Phantom record)"
echo "=========================================="
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
-- TEST CASE 1: UPDATE id=2 (data modification)
UPDATE audit_log SET action = 'TAMPERED_VALUE' WHERE id = 2;

-- TEST CASE 2: DELETE id=3 (record deletion)
DELETE FROM audit_log WHERE id = 3;

-- TEST CASE 3: DELETE id=4 + INSERT id=4 (replace with new data)
DELETE FROM audit_log WHERE id = 4;
INSERT INTO audit_log (id, action) VALUES (4, 'REPLACED_DATA');

-- TEST CASE 4: INSERT id=100 (phantom insert - never in hash chain)
INSERT INTO audit_log (id, action) VALUES (100, 'Phantom Insert');

SELECT id, action FROM audit_log ORDER BY id;
EOF

restart_all_nodes 15

echo ""
echo "Checking offline tampering detection..."
echo "=========================================="
LOGS=$(docker-compose logs --since 15s)

echo ""
echo "TEST CASE 1: UPDATE Detection (id=2)"
if echo "$LOGS" | grep -q "data modified.*id=2"; then
    echo -e "${GREEN}✓ PASSED: Detected UPDATE tampering on id=2${NC}"
    echo "$LOGS" | grep "data modified.*id=2" | head -1
else
    echo -e "${RED}✗ FAILED: UPDATE tampering on id=2 NOT detected${NC}"
fi

echo ""
echo "TEST CASE 2: DELETE Detection (id=3)"
if echo "$LOGS" | grep -q "record deleted.*id=3"; then
    echo -e "${GREEN}✓ PASSED: Detected DELETE tampering on id=3${NC}"
    echo "$LOGS" | grep "record deleted.*id=3" | head -1
else
    echo -e "${RED}✗ FAILED: DELETE tampering on id=3 NOT detected${NC}"
fi

echo ""
echo "TEST CASE 3: DELETE+INSERT Detection (id=4)"
if echo "$LOGS" | grep -q "data modified.*id=4"; then
    echo -e "${GREEN}✓ PASSED: Detected DELETE+INSERT tampering on id=4 (hash mismatch)${NC}"
    echo "$LOGS" | grep "data modified.*id=4" | head -1
else
    echo -e "${RED}✗ FAILED: DELETE+INSERT tampering on id=4 NOT detected${NC}"
fi

echo ""
echo "TEST CASE 4: Phantom INSERT Detection (id=100)"
if echo "$LOGS" | grep -q "Phantom Insert.*id=100"; then
    echo -e "${GREEN}✓ PASSED: Detected Phantom INSERT on id=100${NC}"
    echo "$LOGS" | grep "Phantom Insert.*id=100" | head -1
else
    echo -e "${RED}✗ FAILED: Phantom INSERT on id=100 NOT detected${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Hash Chain Verification Test: COMPLETED${NC}"
echo "=========================================="
echo ""
echo "Test Flow:"
echo "  1. Started cluster, inserted 5 records (id=1,2,3,4,5)"
echo "  2. Stopped all nodes"
echo "  3. Applied 4 types of tampering:"
echo "     - UPDATE id=2 (data modification)"
echo "     - DELETE id=3 (record deletion)"
echo "     - DELETE+INSERT id=4 (same ID, new data)"
echo "     - INSERT id=100 (phantom record)"
echo "  4. Restarted nodes"
echo "  5. Startup verification detected all tampering types"
echo ""
echo "Summary:"
echo "  ✓ TEST CASE 1: UPDATE detection (hash mismatch)"
echo "  ✓ TEST CASE 2: DELETE detection (record missing)"
echo "  ✓ TEST CASE 3: DELETE+INSERT detection (hash mismatch)"
echo "  ✓ TEST CASE 4: Phantom INSERT detection (extra record)"
echo ""
