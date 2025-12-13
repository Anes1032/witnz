#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-common.sh"

echo "=========================================="
echo "Follower Compromise Test (Framework Validation)"
echo "=========================================="
echo ""
echo "This test validates that BoltDB tampering can be detected."
echo "Note: Full Raft Feudalism implementation (follower self-termination"
echo "based on leader consensus) is scheduled for Phase 2."
echo ""

trap cleanup EXIT

docker-compose down -v 2>/dev/null || true

setup_postgres
start_cluster 10

verify_leader_election || exit 1

identify_cluster_roles || exit 1

if [ ${#FOLLOWERS[@]} -lt 2 ]; then
    echo -e "${RED}✗ Not enough followers! Found ${#FOLLOWERS[@]}${NC}"
    exit 1
fi

TARGET_FOLLOWER=${FOLLOWERS[0]}
HEALTHY_FOLLOWER=${FOLLOWERS[1]}

echo ""
echo -e "Target follower for tampering: ${YELLOW}$TARGET_FOLLOWER${NC}"
echo -e "Healthy follower for comparison: $HEALTHY_FOLLOWER"

insert_test_records 3

verify_hash_chain_replication 3

echo ""
echo "=========================================="
echo "BoltDB Tampering Simulation"
echo "=========================================="

stop_all_nodes

echo ""
echo -e "${YELLOW}Attack simulation:${NC}"
echo "  1. Attacker gains root access to $TARGET_FOLLOWER"
echo "  2. Attacker modifies BoltDB hash entries while offline"
echo "  3. System is restarted"
echo ""

echo "Building BoltDB tampering tool..."
docker build -f Dockerfile.tamper -t witnz-tamper:latest . > /dev/null 2>&1

echo "Tampering with $TARGET_FOLLOWER's BoltDB..."

# Get the volume name for the target follower
VOLUME_NAME="witnz_${TARGET_FOLLOWER}_data"

# Run tampering tool with volume mounted
docker run --rm \
    -v "$VOLUME_NAME:/data" \
    witnz-tamper:latest \
    /data/witnz.db \
    audit_log

echo -e "${GREEN}✓ BoltDB successfully tampered${NC}"

restart_all_nodes 20

echo ""
echo "Verifying cluster is running after restart..."
check_cluster_status || exit 1

if [ $RUNNING_NODES -lt 2 ]; then
    echo -e "${RED}✗ Cluster failed to start properly${NC}"
    docker-compose logs | tail -100
    exit 1
fi

echo -e "${GREEN}✓ Cluster started successfully ($RUNNING_NODES/3 nodes)${NC}"
echo ""
echo "Waiting for periodic verification to detect tampering (verify_interval: 30s)..."
echo "This may take up to 60 seconds..."
sleep 60

TARGET_LOGS=$(docker-compose logs $TARGET_FOLLOWER 2>&1 || echo "")

# Check if tampering was detected
if echo "$TARGET_LOGS" | grep -qi "TAMPERING\|integrity violation"; then
    echo -e "${GREEN}✓ Hash tampering successfully detected${NC}"
    echo ""
    echo "Tampering detection logs from $TARGET_FOLLOWER:"
    echo "$TARGET_LOGS" | grep -i "TAMPERING\|integrity violation" | head -10
else
    echo -e "${RED}✗ No tampering detection logs found${NC}"
    exit 1
fi

echo ""
echo "Verifying cluster continues operating despite detection..."
check_cluster_status

if [ $RUNNING_NODES -eq 3 ]; then
    echo -e "${GREEN}✓ All 3 nodes still running${NC}"
    echo ""
    echo "IMPORTANT: This demonstrates Phase 1 limitation:"
    echo "  - BoltDB tampering WAS detected"
    echo "  - But tampered follower continues running"
    echo "  - No leader comparison performed"
    echo "  - No self-termination triggered"
else
    echo -e "${RED}✗ Expected 3 nodes running, got $RUNNING_NODES${NC}"
    echo ""
    echo "Dumping node logs for debugging..."
    echo "=== node1 logs (last 50 lines) ==="
    docker-compose logs --tail=50 node1 2>&1
    echo ""
    echo "=== node2 logs (last 50 lines) ==="
    docker-compose logs --tail=50 node2 2>&1
    echo ""
    echo "=== node3 logs (last 50 lines) ==="
    docker-compose logs --tail=50 node3 2>&1
    exit 1
fi

echo ""
echo "Inserting new record to verify cluster functionality..."
docker-compose exec -T postgres psql -U witnz -d witnzdb -c "INSERT INTO audit_log (action) VALUES ('Post-tampering test');" > /dev/null

sleep 5

NEW_RECORD_COUNT=$(get_record_count)
if [ "$NEW_RECORD_COUNT" -eq 4 ]; then
    echo -e "${GREEN}✓ Cluster accepts new writes (cluster still functional)${NC}"
else
    echo -e "${RED}✗ Failed to insert new record${NC}"
    exit 1
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Follower Compromise Test: COMPLETED${NC}"
echo "=========================================="
echo ""
echo "Test Results:"
echo "  ✅ BoltDB tampering successfully simulated"
echo "  ✅ Hash chain integrity violation detected"
echo "  ✅ Tampered follower continues running (Phase 1 behavior)"
echo "  ✅ Cluster remains fully functional (3/3 nodes)"
echo "  ✅ New writes accepted successfully"
echo ""
echo "Summary:"
echo "  - Initial cluster: 3 nodes ($LEADER, ${FOLLOWERS[*]})"
echo "  - Tampered follower: $TARGET_FOLLOWER"
echo "  - Running nodes: $RUNNING_NODES/3"
echo "  - Records after test: $NEW_RECORD_COUNT"
echo ""
echo "Phase 1 Current Behavior:"
echo "  ✓ Follower detects BoltDB tampering via periodic verification"
echo "  ✓ Logs integrity violation warning"
echo "  ❌ Does NOT compare with leader's BoltDB"
echo "  ❌ Does NOT self-terminate"
echo "  → Tampered follower continues operating"
echo ""
echo "Phase 2 Implementation (TODO):"
echo "  1. Follower queries leader for authoritative hash chain"
echo "  2. Follower compares local BoltDB with leader's hashes"
echo "  3. If mismatch detected:"
echo "     - Follower assumes local corruption (Raft Feudalism)"
echo "     - Follower logs: \"Leader disagrees, assuming local corruption\""
echo "     - Follower calls os.Exit(1) to self-terminate"
echo "  4. Cluster continues with remaining 2/3 nodes"
echo ""
echo "Raft Feudalism Principle:"
echo "  - Leader is ALWAYS the source of truth"
echo "  - Follower NEVER questions the leader"
echo "  - If follower disagrees with leader → follower assumes IT is wrong"
echo "  - Compromised follower removes itself from cluster"
echo ""
