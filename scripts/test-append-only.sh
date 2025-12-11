#!/bin/bash
set -e

echo "=== Witnz Integration Test (Append-only Mode) ==="
echo ""

echo "Step 0: Cleaning up previous test data..."
docker-compose -f docker-compose.test.yml down -v 2>/dev/null || true

echo ""
echo "Step 1: Starting PostgreSQL..."
docker-compose -f docker-compose.test.yml up -d postgres
sleep 5

echo ""
echo "Step 2: Setting up test database..."
docker-compose -f docker-compose.test.yml exec -T postgres psql -U witnz -d witnzdb < test/integration/setup.sql

echo ""
echo "Step 3: Starting witnz node..."
docker-compose -f docker-compose.test.yml up -d node1
sleep 5

echo ""
echo "Step 4: Checking node status..."
docker-compose -f docker-compose.test.yml logs node1 | tail -20

echo ""
echo "Step 5: Inserting test data and triggering tampering..."
docker-compose -f docker-compose.test.yml exec -T postgres psql -U witnz -d witnzdb < test/integration/test_append_only.sql

sleep 2

echo ""
echo "Step 6: Checking witnz logs..."
docker-compose -f docker-compose.test.yml logs node1 | tail -30

echo ""
echo "Step 7: Verifying automatic tampering detection..."
if docker-compose -f docker-compose.test.yml logs node1 | grep -q "TAMPERING DETECTED"; then
  echo "✅ Success: Tampering detected automatically (Append-only mode working)."
else
  echo "❌ Failed: Tampering NOT detected in logs."
  exit 1
fi

echo ""
echo "Step 8: Cleaning up..."
docker-compose -f docker-compose.test.yml down

echo ""
echo "=== Integration Test Complete (Append-only Mode) ==="
