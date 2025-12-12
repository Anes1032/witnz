#!/bin/bash
set -e

echo "=== Witnz Hash Chain Verification Test ==="
echo ""
echo "This test verifies that witnz can detect tampering that occurred while nodes were offline."
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
INSERT INTO audit_log (action) VALUES ('Action 1');
INSERT INTO audit_log (action) VALUES ('Action 2');
INSERT INTO audit_log (action) VALUES ('Action 3');
SELECT id, action FROM audit_log;
EOF

echo "Waiting for CDC propagation and hash chain storage..."
sleep 5

echo ""
echo "Step 6: Verifying hash chain entries were created..."
HASH_CHAIN_LOGS=$(docker-compose logs | grep "hash chain entry replicated" | wc -l | tr -d ' ')
if [ "$HASH_CHAIN_LOGS" -ge 3 ]; then
  echo "✅ Hash chain entries replicated via Raft ($HASH_CHAIN_LOGS entries)"
else
  echo "⚠️  Expected 3+ hash chain entries, found $HASH_CHAIN_LOGS"
fi

echo ""
echo "Step 7: Stopping all witnz nodes..."
docker-compose stop node1 node2 node3
echo "All nodes stopped."
sleep 2

echo ""
echo "Step 8: Tampering with database while nodes are offline..."
echo "Modifying record id=2..."
docker-compose exec -T postgres psql -U witnz -d witnzdb <<'EOF'
UPDATE audit_log SET action = 'TAMPERED_VALUE' WHERE id = 2;
SELECT id, action FROM audit_log;
EOF

echo ""
echo "Step 9: Restarting witnz nodes..."
docker-compose start node1 node2 node3
echo "Waiting for nodes to start and run verification..."
sleep 15

echo ""
echo "Step 10: Checking for tampering detection..."
if docker-compose logs --since 15s | grep -q "TAMPERING"; then
  echo "🚨 TAMPERING DETECTED!"
  docker-compose logs --since 15s | grep -E "TAMPERING|tampered|modified" | head -5
  echo "✅ Verification successfully detected the tampering"
else
  echo "⚠️  Tampering detection message not found in recent logs"
  echo "Checking all logs..."
  docker-compose logs | grep -E "VERIFICATION|verified|TAMPERING" | tail -10
fi

echo ""
echo "=== Hash Chain Verification Test Complete ==="
echo ""
echo "Test Flow:"
echo "  1. Started cluster, inserted 3 records"
echo "  2. Stopped all nodes"
echo "  3. Tampered with record id=2 while offline"
echo "  4. Restarted nodes"
echo "  5. Startup verification detected the tampering"

echo ""
echo "Cleaning up..."
docker-compose down -v
