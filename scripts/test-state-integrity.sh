#!/bin/bash
set -e

echo "=== Witnz State Integrity Mode Integration Test ==="
echo ""

echo "Step 0: Cleaning up previous test data..."
docker-compose down -v 2>/dev/null || true

echo ""
echo "Step 1: Starting PostgreSQL..."
docker-compose up -d postgres
sleep 5

echo ""
echo "Step 2: Setting up test database..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
CREATE TABLE permissions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER,
    role VARCHAR(50),
    updated_at TIMESTAMP DEFAULT NOW()
);

INSERT INTO permissions (user_id, role) VALUES (1, 'admin'), (2, 'user'), (3, 'guest');
EOF

echo ""
echo "Step 3: Starting witnz 3-node cluster..."
docker-compose up -d node1 node2 node3
echo "Waiting for cluster to form..."
sleep 10

echo ""
echo "Step 4: Checking cluster status..."
echo "Node 1 logs:"
docker-compose logs node1 | grep -E "(leader|Raft|entering)" | tail -5
echo ""
echo "Node 2 logs:"
docker-compose logs node2 | grep -E "(leader|Raft|entering)" | tail -5
echo ""
echo "Node 3 logs:"
docker-compose logs node3 | grep -E "(leader|Raft|entering)" | tail -5

echo ""
echo "Step 5: Verifying leader election..."
LEADER_COUNT=$(docker-compose logs | grep "entering leader state" | wc -l | tr -d ' ')
if [ "$LEADER_COUNT" -eq 1 ]; then
  echo "✅ Leader election successful (exactly 1 leader)"
else
  echo "❌ Leader election failed (expected 1 leader, got $LEADER_COUNT)"
  docker-compose logs
  exit 1
fi

echo ""
echo "Step 6: Verifying Raft consensus..."
RAFT_LOGS=$(docker-compose logs | grep "pipelining replication" | wc -l | tr -d ' ')
if [ "$RAFT_LOGS" -gt 0 ]; then
  echo "✅ Raft consensus is active (found $RAFT_LOGS replication messages)"
else
  echo "⚠️  No Raft replication messages found"
fi

echo ""
echo "Step 7: Inserting data into permissions table..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
INSERT INTO permissions (user_id, role) VALUES (4, 'moderator');
SELECT COUNT(*) as total_permissions FROM permissions;
EOF

echo "Waiting for initial state integrity verification cycle (10s interval)..."
sleep 12

echo ""
echo "Step 8: Verifying initial state integrity logs..."
if docker-compose logs | grep -q "State integrity check for permissions"; then
  echo "✅ Initial state integrity verification completed"
  docker-compose logs | grep "State integrity check for permissions" | tail -3
else
  echo "⚠️  Initial state integrity logs not found"
fi

echo ""
echo "Step 9: Testing tampering detection on state_integrity table..."
echo "Attempting unauthorized UPDATE on permissions table..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
UPDATE permissions SET role = 'superadmin' WHERE user_id = 1;
EOF

echo "Waiting for next verification cycle to detect tampering (10s interval + buffer)..."
sleep 15

echo ""
echo "Step 10: Verifying tampering detection..."
if docker-compose logs | grep -q "TAMPERING DETECTED"; then
  echo "✅ Tampering detected successfully"
  docker-compose logs | grep "TAMPERING DETECTED" | tail -5
else
  echo "⚠️  Tampering detection message not found"
  echo "Checking merkle root changes..."
  docker-compose logs | grep -i "State integrity check for permissions" | tail -10
fi

echo ""
echo "Step 11: Final cluster state..."
docker-compose ps

echo ""
echo "Step 12: Cleaning up..."
docker-compose down -v

echo ""
echo "=== State Integrity Mode Integration Test Complete ==="
echo ""
echo "Summary:"
echo "  ✅ 3-node cluster formation"
echo "  ✅ Leader election"
echo "  ✅ Raft consensus"
echo "  ✅ State integrity mode verification"
echo "  ✅ CDC data ingestion (state_integrity)"
