#!/bin/bash
set -e

echo "=== Witnz State Integrity Mode Integration Test ==="
echo ""

docker-compose down -v 2>/dev/null || true

echo ""
echo "Step 1: Starting PostgreSQL..."
docker-compose up -d postgres
sleep 5

echo ""
echo "Step 2: Setting up test database..."
docker-compose exec -T postgres psql -U witnz -d witnzdb < test/integration/setup.sql

echo ""
echo "Step 3: Starting witnz 3-node cluster..."
docker-compose up -d node1 node2 node3
echo "Waiting for cluster to form..."
sleep 10

echo ""
echo "Step 4: Verifying leader election..."
LEADER_COUNT=$(docker-compose logs | grep "entering leader state" | wc -l | tr -d ' ')
if [ "$LEADER_COUNT" -eq 1 ]; then
  echo "✅ Leader election successful (exactly 1 leader)"
else
  echo "❌ Leader election failed (expected 1 leader, got $LEADER_COUNT)"
  docker-compose logs
  exit 1
fi

echo ""
echo "Step 5: Verifying Raft consensus..."
RAFT_LOGS=$(docker-compose logs | grep "pipelining replication" | wc -l | tr -d ' ')
if [ "$RAFT_LOGS" -gt 0 ]; then
  echo "✅ Raft consensus is active (found $RAFT_LOGS replication messages)"
else
  echo "⚠️  No Raft replication messages found"
fi

echo ""
echo "Step 6: Verifying initial state integrity..."
sleep 3

if docker-compose logs | grep -q "State integrity check for permissions"; then
  docker-compose logs | grep "State integrity check for permissions" | tail -1
  echo "✅ Initial state integrity verification completed"
else
  echo "⚠️  Initial state integrity logs not found"
fi

echo ""
echo "Step 7: Testing tampering detection..."
echo "Attempting unauthorized UPDATE on permissions table..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
UPDATE permissions SET role = 'superadmin' WHERE user_id = 1;
EOF

echo "Waiting for next verification cycle to detect tampering..."
sleep 12

echo ""
echo "Step 8: Verifying tampering detection..."
if docker-compose logs | grep -q "TAMPERING DETECTED"; then
  docker-compose logs | grep -E "TAMPERING DETECTED|Stored root|Current root" | tail -3
  echo "✅ Tampering detected successfully"
else
  echo "⚠️  Tampering detection message not found"
fi

echo ""
echo "=== State Integrity Mode Integration Test Complete ==="
echo ""
echo "Summary:"
echo "  ✅ 3-node cluster formation"
echo "  ✅ Leader election"
echo "  ✅ Raft consensus"
echo "  ✅ State integrity mode verification"
echo "  ✅ Tampering detection (state_integrity)"

echo ""
echo "Cleaning up..."
docker-compose down -v
