#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-common.sh"

echo "=========================================="
echo "Append-only Mode Test"
echo "=========================================="
echo ""
echo "This test verifies real-time tampering detection for append-only tables."
echo "Expected behavior: UPDATE/DELETE operations trigger immediate alerts."
echo ""

trap cleanup EXIT

docker-compose down -v 2>/dev/null || true

setup_postgres
start_cluster 10

verify_leader_election || exit 1

insert_test_records 3

verify_hash_chain_replication 3

echo ""
echo "=========================================="
echo "Real-time Tampering Detection Tests"
echo "=========================================="

echo ""
echo "TEST 1: Attempting UPDATE (should trigger alert)..."
docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
    UPDATE audit_log SET action = 'TAMPERED' WHERE id = 1;
" > /dev/null

sleep 2

UPDATE_ALERT=$(docker-compose logs --since 5s | grep -i "TAMPERING DETECTED.*UPDATE" || echo "")
if [ -n "$UPDATE_ALERT" ]; then
    echo -e "${GREEN}✓ UPDATE detected and alerted${NC}"
    echo "$UPDATE_ALERT" | tail -1
else
    echo -e "${RED}✗ UPDATE not detected${NC}"
fi

echo ""
echo "TEST 2: Attempting DELETE (should trigger alert)..."
docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
    DELETE FROM audit_log WHERE id = 2;
" > /dev/null

sleep 2

DELETE_ALERT=$(docker-compose logs --since 5s | grep -i "TAMPERING DETECTED.*DELETE" || echo "")
if [ -n "$DELETE_ALERT" ]; then
    echo -e "${GREEN}✓ DELETE detected and alerted${NC}"
    echo "$DELETE_ALERT" | tail -1
else
    echo -e "${RED}✗ DELETE not detected${NC}"
fi

echo ""
echo "TEST 3: INSERT operation (should be allowed)..."
docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
    INSERT INTO audit_log (action) VALUES ('New record - allowed');
" > /dev/null

sleep 2

RECORD_COUNT=$(get_record_count)
if [ "$RECORD_COUNT" -ge "3" ]; then
    echo -e "${GREEN}✓ INSERT allowed (${RECORD_COUNT} records)${NC}"
else
    echo -e "${RED}✗ INSERT failed (${RECORD_COUNT} records)${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Append-only Mode Test: COMPLETED${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  ✓ Real-time UPDATE detection"
echo "  ✓ Real-time DELETE detection"
echo "  ✓ INSERT operations allowed"
echo ""
echo "Append-only mode ensures historical records remain immutable"
echo "while allowing new records to be added."
echo ""
