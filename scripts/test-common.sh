#!/bin/bash

# Common test utilities for Witnz integration tests

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    docker-compose down -v 2>/dev/null || true
}

# Setup PostgreSQL and database
setup_postgres() {
    echo "Starting PostgreSQL..."
    docker-compose up -d postgres
    sleep 5

    echo "Setting up test database..."
    docker-compose exec -T postgres psql -U witnz -d witnzdb < test/integration/setup.sql
}

# Start 3-node Raft cluster
start_cluster() {
    local wait_time=${1:-15}

    echo "Starting 3-node Raft cluster..."
    docker-compose up -d node1 node2 node3

    echo "Waiting for cluster to initialize (${wait_time} seconds)..."
    sleep "$wait_time"
}

# Verify leader election
verify_leader_election() {
    echo ""
    echo "Verifying leader election..."
    LEADER_COUNT=$(docker-compose logs | grep "entering leader state" | wc -l | tr -d ' ')
    if [ "$LEADER_COUNT" -eq 1 ]; then
        echo -e "${GREEN}✓ Leader election successful (exactly 1 leader)${NC}"
        return 0
    else
        echo -e "${RED}✗ Leader election failed (expected 1 leader, got $LEADER_COUNT)${NC}"
        echo "Cluster logs:"
        docker-compose logs | grep -E "entering leader state|entering follower state|candidate state" | tail -20
        return 1
    fi
}

# Identify leader and followers
identify_cluster_roles() {
    echo ""
    echo "Identifying cluster roles..."
    LEADER=""
    FOLLOWERS=()

    for node in node1 node2 node3; do
        LOGS=$(docker-compose logs $node | grep "entering leader state" | tail -1)
        if [ -n "$LOGS" ]; then
            LEADER=$node
            echo -e "${GREEN}Leader: $LEADER${NC}"
        else
            FOLLOWERS+=($node)
            echo "Follower: $node"
        fi
    done

    if [ -z "$LEADER" ]; then
        echo -e "${RED}✗ No leader found!${NC}"
        return 1
    fi

    return 0
}

# Insert test records
insert_test_records() {
    local count=${1:-3}

    echo ""
    echo "Inserting $count test records..."

    case $count in
        2)
            docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
                INSERT INTO audit_log (action) VALUES
                ('Test record 1'),
                ('Test record 2');
            " > /dev/null
            ;;
        3)
            docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
                INSERT INTO audit_log (action) VALUES
                ('Record 1'),
                ('Record 2'),
                ('Record 3');
            " > /dev/null
            ;;
        5)
            docker-compose exec -T postgres psql -U witnz -d witnzdb -c "
                INSERT INTO audit_log (action) VALUES
                ('Action 1'),
                ('Action 2'),
                ('Action 3'),
                ('Action 4'),
                ('Action 5');
            " > /dev/null
            ;;
        *)
            echo -e "${RED}✗ Unsupported record count: $count${NC}"
            return 1
            ;;
    esac

    sleep 5
    echo -e "${GREEN}✓ Test records inserted${NC}"
}

# Verify hash chain replication
verify_hash_chain_replication() {
    local expected_count=${1:-3}

    echo ""
    echo "Verifying hash chain entries created..."
    HASH_COUNT=$(docker-compose logs | grep "hash chain entry replicated" | wc -l | tr -d ' ')
    if [ "$HASH_COUNT" -ge "$expected_count" ]; then
        echo -e "${GREEN}✓ Hash chain entries replicated ($HASH_COUNT entries)${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠ Expected $expected_count+ hash entries, found $HASH_COUNT${NC}"
        return 1
    fi
}

# Stop all nodes
stop_all_nodes() {
    echo ""
    echo "Stopping all witnz nodes..."
    docker-compose stop node1 node2 node3
    echo "All nodes stopped."
    sleep 2
}

# Restart all nodes
restart_all_nodes() {
    local wait_time=${1:-15}

    echo ""
    echo "Restarting witnz nodes..."
    docker-compose start node1 node2 node3
    echo "Waiting for nodes to start (${wait_time} seconds)..."
    sleep "$wait_time"
}

# Check cluster status
# Sets global variable RUNNING_NODES with the count
check_cluster_status() {
    echo ""
    echo "Checking cluster status..."

    RUNNING_NODES=0
    for node in node1 node2 node3; do
        if docker-compose ps $node | grep -q "Up"; then
            RUNNING_NODES=$((RUNNING_NODES + 1))
            echo -e "${GREEN}✓ $node is running${NC}"
        else
            echo -e "${YELLOW}⚠ $node is down${NC}"
        fi
    done

    echo ""
    echo "Running nodes: $RUNNING_NODES/3"

    if [ $RUNNING_NODES -ge 2 ]; then
        echo -e "${GREEN}✓ Quorum maintained (${RUNNING_NODES}/3 nodes)${NC}"
        return 0
    else
        echo -e "${RED}✗ Quorum lost! Only ${RUNNING_NODES}/3 nodes running${NC}"
        return 1
    fi
}

# Get record count
get_record_count() {
    docker-compose exec -T postgres psql -U witnz -d witnzdb -t -c "SELECT COUNT(*) FROM audit_log;" | tr -d ' '
}
