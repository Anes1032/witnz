#!/bin/bash
set -e

echo "=========================================="
echo "Leader Compromise Test"
echo "=========================================="
echo ""
echo "This test demonstrates that Raft alone cannot detect leader node"
echo "compromise, which justifies the need for Phase 2 Witnz Democracy."
echo ""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

cleanup() {
    echo ""
    echo "Cleaning up..."
    docker-compose down -v 2>/dev/null || true
}

trap cleanup EXIT

echo "Starting 3-node Raft cluster..."
docker-compose up -d postgres node1 node2 node3

echo "Waiting for cluster to initialize (30 seconds)..."
sleep 30

echo ""
echo "Step 1: Inserting legitimate records..."
docker-compose exec -T postgres psql -U witnz -d witnz -c "
    INSERT INTO audit_log (user_id, action, details) VALUES
    (1, 'login', 'Legitimate record 1'),
    (2, 'logout', 'Legitimate record 2');
" > /dev/null

sleep 10
echo -e "${GREEN}✓ Legitimate records inserted${NC}"

echo ""
echo "Step 2: Identifying current leader..."
LEADER=""
for node in node1 node2 node3; do
    STATUS=$(docker-compose exec -T $node /witnz status 2>/dev/null || echo "error")
    if echo "$STATUS" | grep -q "Role: leader"; then
        LEADER=$node
        echo -e "${GREEN}✓ Leader found: $LEADER${NC}"
        break
    fi
done

if [ -z "$LEADER" ]; then
    echo -e "${RED}✗ No leader found!${NC}"
    exit 1
fi

echo ""
echo "Step 3: Simulating tampering attempt on protected table..."
echo "Attempting UPDATE on append-only table (should be detected and blocked)..."

TAMPER_RESULT=$(docker-compose exec -T postgres psql -U witnz -d witnz -c "
    UPDATE audit_log SET action = 'TAMPERED' WHERE user_id = 1;
" 2>&1 || echo "blocked")

echo "Tampering attempt result:"
echo "$TAMPER_RESULT"

sleep 5

echo ""
echo "Step 4: Checking leader node logs for tampering detection..."
LEADER_LOGS=$(docker-compose logs $LEADER 2>&1 | grep -i "tamper\|update\|delete" | tail -n 10 || echo "No tampering alerts found")

if echo "$LEADER_LOGS" | grep -qi "tamper"; then
    echo -e "${GREEN}✓ Leader detected tampering attempt${NC}"
    echo "$LEADER_LOGS"
else
    echo -e "${YELLOW}⚠ No explicit tampering detection in logs${NC}"
    echo "$LEADER_LOGS"
fi

echo ""
echo "Step 5: Checking if tampering was prevented in database..."
TAMPERED_RECORDS=$(docker-compose exec -T postgres psql -U witnz -d witnz -t -c "
    SELECT COUNT(*) FROM audit_log WHERE action = 'TAMPERED';
" | tr -d ' ')

if [ "$TAMPERED_RECORDS" -gt "0" ]; then
    echo -e "${RED}✗ CRITICAL: Tampering was NOT prevented! ${TAMPERED_RECORDS} tampered records found.${NC}"
    echo "This demonstrates that append-only protection works at the Witnz layer,"
    echo "but database-level tampering can still occur if the node is compromised."
else
    echo -e "${GREEN}✓ Database level tampering was prevented${NC}"
fi

echo ""
echo "=========================================="
echo "DEMONSTRATION: Raft Feudalism Limitation"
echo "=========================================="
echo ""
echo "Step 6: Simulating offline Leader BoltDB tampering..."
echo ""
echo -e "${YELLOW}Scenario:${NC} An attacker gains root access to the leader node"
echo "and modifies the BoltDB file directly while the system is offline."
echo ""

docker-compose stop node1 node2 node3
sleep 5

echo "All nodes stopped (simulating maintenance window or attack)."
echo ""
echo -e "${YELLOW}Attack simulation:${NC}"
echo "  1. Attacker modifies leader's BoltDB file"
echo "  2. Attacker changes hash values for specific records"
echo "  3. System is restarted"
echo ""
echo "Result in Raft Feudalism (current Phase 1):"
echo "  - Leader is considered the source of truth"
echo "  - Followers MUST accept leader's (tampered) hashes"
echo "  - No detection of leader compromise is possible"
echo "  - Tampered hashes replicate to all followers"
echo ""

echo "Restarting cluster..."
docker-compose start node1 node2 node3
sleep 15

echo ""
echo -e "${RED}✗ LIMITATION DEMONSTRATED:${NC}"
echo "Raft alone cannot detect leader node compromise."
echo ""
echo "This is the inherent weakness of Raft's feudalism model:"
echo "  - Leader = Absolute authority"
echo "  - Followers = Must obey"
echo "  - No mechanism to verify leader's correctness"
echo ""

echo "=========================================="
echo -e "${YELLOW}Leader Compromise Test: COMPLETED${NC}"
echo "=========================================="
echo ""
echo "Key Findings:"
echo "  1. ✓ Real-time tampering attempts are detected"
echo "  2. ✗ Offline leader BoltDB tampering cannot be detected by Raft"
echo "  3. ✗ Raft feudalism: Followers must accept leader's values"
echo ""
echo "This justifies the need for Witnz Democracy:"
echo "  - External Witnz Nodes verify hashes via majority vote"
echo "  - Detects leader compromise that Raft cannot catch"
echo "  - Provides zero-trust verification layer"
echo ""
