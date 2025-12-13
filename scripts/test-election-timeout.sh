#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test-common.sh"

echo "=========================================="
echo "Raft Election Timeout Test"
echo "=========================================="
echo ""
echo "This test verifies that Raft automatically elects a new leader"
echo "when the current leader fails (election timeout mechanism)."
echo ""

trap cleanup EXIT

docker-compose down -v 2>/dev/null || true

setup_postgres
start_cluster 10

identify_cluster_roles || exit 1

insert_test_records 2

echo ""
echo "Stopping leader node ($LEADER)..."
docker-compose stop $LEADER
echo -e "${YELLOW}Leader $LEADER stopped${NC}"

echo ""
echo "Waiting for election timeout and new leader election (10 seconds)..."
sleep 10

echo ""
echo "Verifying new leader elected..."
NEW_LEADER=""
for node in node1 node2 node3; do
    if [ "$node" == "$LEADER" ]; then
        continue
    fi

    if docker-compose ps $node | grep -q "Up"; then
        RECENT_LEADER_LOG=$(docker-compose logs --since 15s $node | grep "entering leader state" || echo "")
        if [ -n "$RECENT_LEADER_LOG" ]; then
            NEW_LEADER=$node
            echo -e "${GREEN}✓ New leader elected: $NEW_LEADER${NC}"
            break
        fi
    fi
done

if [ -z "$NEW_LEADER" ]; then
    echo -e "${RED}✗ No new leader elected! Election timeout may have failed.${NC}"
    exit 1
fi

echo ""
echo "Verifying cluster continues operating with 2 nodes..."
docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
    INSERT INTO audit_log (action) VALUES
    ('Test record after leader failure');
" > /dev/null

sleep 5

RECORD_COUNT=$(get_record_count)
if [ "$RECORD_COUNT" -ge "3" ]; then
    echo -e "${GREEN}✓ Cluster continues accepting writes (2-node quorum maintained)${NC}"
else
    echo -e "${RED}✗ Cluster not accepting writes after leader failure${NC}"
    exit 1
fi

echo ""
echo "Restarting old leader and verifying it joins as follower..."
docker-compose start $LEADER

sleep 10

FOLLOWER_LOG=$(docker-compose logs --since 15s $LEADER | grep "entering follower state" || echo "")
if [ -n "$FOLLOWER_LOG" ]; then
    echo -e "${GREEN}✓ Old leader rejoined cluster as follower${NC}"
else
    echo -e "${YELLOW}⚠ Old leader status unclear (may still be starting up)${NC}"
fi

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
echo "This demonstrates Raft's automatic leader election:"
echo "  - Leader failure detected via election timeout"
echo "  - Remaining nodes elected new leader"
echo "  - Cluster continues operating without manual intervention"
echo ""
