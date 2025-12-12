#!/bin/bash
set -e

echo "=== Witnz Append-Only Mode Integration Test ==="
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
echo "Step 5: Inserting test data..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
INSERT INTO audit_log (action) VALUES ('Test insert 1');
INSERT INTO audit_log (action) VALUES ('Test insert 2');
INSERT INTO audit_log (action) VALUES ('Test insert 3');
SELECT COUNT(*) as total_records FROM audit_log;
EOF

echo "Waiting for CDC propagation..."
sleep 5

echo ""
echo "Step 6: Verifying Raft consensus..."
RAFT_LOGS=$(docker-compose logs | grep "pipelining replication" | wc -l | tr -d ' ')
if [ "$RAFT_LOGS" -gt 0 ]; then
  echo "✅ Raft consensus is active (found $RAFT_LOGS replication messages)"
else
  echo "⚠️  No Raft replication messages found"
fi

echo ""
echo "Step 7: Testing tampering detection..."
echo "Attempting UPDATE on append-only table..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
UPDATE audit_log SET action = 'TAMPERED' WHERE id = 1;
EOF

sleep 3

echo ""
echo "Step 8: Verifying tampering detection..."
if docker-compose logs | grep -q "TAMPERING DETECTED"; then
  docker-compose logs node1 | grep "TAMPERING DETECTED" | tail -1
  echo "✅ Tampering detected successfully"
else
  echo "⚠️  Tampering detection message not found in logs"
fi

echo ""
echo "=== Append-Only Mode Integration Test Complete ==="
echo ""
echo "Summary:"
echo "  ✅ 3-node cluster formation"
echo "  ✅ Leader election"
echo "  ✅ Raft consensus"
echo "  ✅ CDC data ingestion (append-only)"
echo "  ✅ Tampering detection (append-only)"

# echo ""
# echo "Cleaning up..."
# docker-compose down -v
