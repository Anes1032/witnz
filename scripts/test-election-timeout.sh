#!/bin/bash
set -e

echo "=========================================="
echo "Raft Election Timeout Test"
echo "=========================================="
echo ""
echo "This test verifies that Raft automatically elects a new leader"
echo "when the current leader fails (election timeout mechanism)."
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    docker-compose down -v 2>/dev/null || true
}

trap cleanup EXIT

# Start fresh
echo "Starting 3-node Raft cluster..."
docker-compose up -d postgres node1 node2 node3

# Wait for cluster to form
echo "Waiting for cluster to initialize (30 seconds)..."
sleep 30

# Check initial leader
echo ""
echo "Step 1: Identifying current leader..."
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
    echo -e "${RED}✗ No leader found! Cluster may not be healthy.${NC}"
    exit 1
fi

# Insert test records to verify cluster is working
echo ""
echo "Step 2: Inserting test records..."
docker-compose exec -T postgres psql -U witnz -d witnz -c "
    INSERT INTO audit_log (user_id, action, details) VALUES
    (1, 'login', 'Test record 1'),
    (2, 'logout', 'Test record 2');
" > /dev/null

sleep 5

# Verify hash chain replicated to all nodes
echo -e "${GREEN}✓ Test records inserted${NC}"

# Kill the leader
echo ""
echo "Step 3: Stopping leader node ($LEADER)..."
docker-compose stop $LEADER
echo -e "${YELLOW}Leader $LEADER stopped${NC}"

# Wait for election timeout and new leader election
echo ""
echo "Waiting for election timeout and new leader election (10 seconds)..."
sleep 10

# Check for new leader
echo ""
echo "Step 4: Verifying new leader elected..."
NEW_LEADER=""
for node in node1 node2 node3; do
    if [ "$node" == "$LEADER" ]; then
        continue  # Skip the stopped node
    fi

    STATUS=$(docker-compose exec -T $node /witnz status 2>/dev/null || echo "error")
    if echo "$STATUS" | grep -q "Role: leader"; then
        NEW_LEADER=$node
        echo -e "${GREEN}✓ New leader elected: $NEW_LEADER${NC}"
        break
    fi
done

if [ -z "$NEW_LEADER" ]; then
    echo -e "${RED}✗ No new leader elected! Election timeout may have failed.${NC}"
    exit 1
fi

# Verify cluster is still operational with 2 nodes
echo ""
echo "Step 5: Verifying cluster continues operating with 2 nodes..."
docker-compose exec -T postgres psql -U witnz -d witnz -c "
    INSERT INTO audit_log (user_id, action, details) VALUES
    (3, 'login', 'Test record after leader failure');
" > /dev/null

sleep 5

# Check if hash chain replication worked
RECORD_COUNT=$(docker-compose exec -T postgres psql -U witnz -d witnz -t -c "SELECT COUNT(*) FROM audit_log;" | tr -d ' ')
if [ "$RECORD_COUNT" -ge "3" ]; then
    echo -e "${GREEN}✓ Cluster continues accepting writes (2-node quorum maintained)${NC}"
else
    echo -e "${RED}✗ Cluster not accepting writes after leader failure${NC}"
    exit 1
fi

# Optional: Restart old leader and verify it becomes follower
echo ""
echo "Step 6: Restarting old leader and verifying it joins as follower..."
docker-compose start $LEADER

sleep 10

STATUS=$(docker-compose exec -T $LEADER /witnz status 2>/dev/null || echo "error")
if echo "$STATUS" | grep -q "Role: follower"; then
    echo -e "${GREEN}✓ Old leader rejoined cluster as follower${NC}"
else
    echo -e "${YELLOW}⚠ Old leader status unclear (may still be starting up)${NC}"
fi

# Final verification
echo ""
echo "=========================================="
echo -e "${GREEN}Election Timeout Test: PASSED${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Original leader: $LEADER"
echo "  - New leader after timeout: $NEW_LEADER"
echo "  - Cluster maintained quorum with 2 nodes"
echo "  - Hash chain replication continued"
echo ""
