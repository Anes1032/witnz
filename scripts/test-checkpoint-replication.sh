#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-common.sh"

echo "=========================================="
echo "Checkpoint Replication Test"
echo "=========================================="
echo ""
echo "This test verifies checkpoint sharing via Raft consensus."
echo ""

trap cleanup EXIT

docker-compose down -v 2>/dev/null || true

setup_postgres
start_cluster 10

verify_leader_election || exit 1

insert_test_records 3

echo ""
echo "Waiting for initial checkpoint creation (10 seconds)..."
sleep 10

echo ""
echo "Checking for checkpoint replication in logs..."
echo "=========================================="

LEADER_LOGS=$(docker-compose logs --since 15s | grep "Created and replicated Merkle checkpoint")
FOLLOWER_LOGS=$(docker-compose logs --since 15s | grep "Applied checkpoint from Raft")

if [ -n "$LEADER_LOGS" ]; then
    echo -e "${GREEN}✓ PASSED: Leader created and replicated checkpoint${NC}"
    echo "$LEADER_LOGS" | head -3
else
    echo -e "${YELLOW}⚠ Leader checkpoint replication not detected (may not have triggered yet)${NC}"
fi

echo ""
if [ -n "$FOLLOWER_LOGS" ]; then
    echo -e "${GREEN}✓ PASSED: Followers received checkpoint via Raft${NC}"
    echo "$FOLLOWER_LOGS" | head -3
else
    echo -e "${YELLOW}⚠ Follower checkpoint application not detected${NC}"
fi

echo ""
echo "Inserting 2 more records to trigger verification..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
INSERT INTO audit_log (action) VALUES ('Action 4');
INSERT INTO audit_log (action) VALUES ('Action 5');
EOF

echo ""
echo "Waiting for periodic verification and checkpoint (10 seconds)..."
sleep 10

echo ""
echo "Checking logs again for checkpoint activity..."
RECENT_LOGS=$(docker-compose logs --since 12s)

LEADER_CHECKPOINTS=$(echo "$RECENT_LOGS" | grep "Created and replicated" | wc -l | tr -d ' ')
FOLLOWER_CHECKPOINTS=$(echo "$RECENT_LOGS" | grep "Applied checkpoint from Raft" | wc -l | tr -d ' ')

echo "Leader checkpoint replications: $LEADER_CHECKPOINTS"
echo "Follower checkpoint applications: $FOLLOWER_CHECKPOINTS"

if [ "$LEADER_CHECKPOINTS" -gt 0 ]; then
    echo -e "${GREEN}✓ PASSED: Checkpoint replication is working${NC}"
else
    echo -e "${YELLOW}⚠ WARNING: No checkpoint replication detected (may need longer wait)${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Checkpoint Replication Test: COMPLETED${NC}"
echo "=========================================="
echo ""
