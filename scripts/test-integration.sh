#!/bin/bash
set -e

echo "=== Witnz Integration Test ==="
echo ""

echo "Step 1: Starting PostgreSQL..."
docker-compose up -d postgres
sleep 5

echo ""
echo "Step 2: Setting up test database..."
docker-compose exec -T postgres psql -U witnz -d witnzdb < test/integration/setup.sql

echo ""
echo "Step 3: Starting witnz node..."
docker-compose up -d node1
sleep 5

echo ""
echo "Step 4: Checking node status..."
docker-compose logs node1 | tail -20

echo ""
echo "Step 5: Inserting test data..."
docker-compose exec -T postgres psql -U witnz -d witnzdb < test/integration/test_operations.sql

sleep 2

echo ""
echo "Step 6: Checking witnz logs..."
docker-compose logs node1 | tail -30

echo ""
echo "Step 7: Verifying hash chain..."
docker-compose exec node1 ./witnz verify --config /config/witnz-test.yaml || true

echo ""
echo "=== Integration Test Complete ==="
