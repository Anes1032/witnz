#!/bin/bash
set -e

echo "=== Witnz Integration Test (State Integrity Mode) ==="
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
echo "Step 4: Waiting for initial state integrity verification (interval: 5s)..."
sleep 6

echo ""
echo "Step 5: Checking initial Merkle root was calculated..."
docker-compose -f docker-compose.test.yml logs node1 | grep "State integrity check" | tail -1

echo ""
echo "Step 6: Simulating tampering (direct UPDATE on permissions table)..."
docker-compose -f docker-compose.test.yml exec -T postgres psql -U witnz -d witnzdb < test/integration/test_state_integrity.sql

echo ""
echo "Step 7: Waiting for next verification cycle..."
sleep 6

echo ""
echo "Step 8: Checking witnz logs for Merkle root change..."
docker-compose -f docker-compose.test.yml logs node1 | grep "State integrity check" | tail -3

echo ""
echo "Step 9: Verifying state integrity checks ran..."
FIRST_ROOT=$(docker-compose -f docker-compose.test.yml logs node1 | grep "State integrity check for permissions" | head -1 | grep -o 'root=[a-f0-9]*' | cut -d= -f2)
LAST_ROOT=$(docker-compose -f docker-compose.test.yml logs node1 | grep "State integrity check for permissions" | tail -1 | grep -o 'root=[a-f0-9]*' | cut -d= -f2)

echo "First Merkle root: $FIRST_ROOT"
echo "Last Merkle root:  $LAST_ROOT"

if [ "$FIRST_ROOT" != "$LAST_ROOT" ]; then
  echo "✅ Success: Merkle root changed after tampering was detected!"
  echo "   The system recorded that table state changed from $FIRST_ROOT to $LAST_ROOT"
else
  echo "❌ Failed: Merkle root did not change after tampering."
  exit 1
fi

echo ""
echo "Step 10: Cleaning up..."
docker-compose -f docker-compose.test.yml down -v

echo ""
echo "=== Integration Test Complete (State Integrity Mode) ==="
