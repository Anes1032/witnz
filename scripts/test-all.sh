#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Witnz Integration Test Suite"
echo "=========================================="
echo ""
echo "Running all integration tests..."
echo ""

TESTS=(
    "test-append-only:Append-only Mode Test"
    "test-verify:Hash Chain Verification Test"
    "test-election-timeout:Raft Election Timeout Test"
    "test-leadership-transfer:Raft Leadership Transfer Test"
    "test-node-restart:Node Restart Test"
)

PASSED=0
FAILED=0
FAILED_TESTS=()

for test_info in "${TESTS[@]}"; do
    IFS=':' read -r test_name test_desc <<< "$test_info"

    echo ""
    echo -e "${BLUE}=========================================="
    echo "Running: $test_desc"
    echo -e "==========================================${NC}"
    echo ""

    if "$SCRIPT_DIR/$test_name.sh"; then
        echo -e "${GREEN}✓ $test_desc: PASSED${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗ $test_desc: FAILED${NC}"
        FAILED=$((FAILED + 1))
        FAILED_TESTS+=("$test_desc")
    fi

    sleep 2
done

echo ""
echo "=========================================="
echo "Test Suite Summary"
echo "=========================================="
echo ""
echo "Total tests: $((PASSED + FAILED))"
echo -e "${GREEN}Passed: $PASSED${NC}"

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""
    echo "Failed tests:"
    for failed_test in "${FAILED_TESTS[@]}"; do
        echo -e "  ${RED}✗ $failed_test${NC}"
    done
    echo ""
    exit 1
else
    echo -e "${GREEN}Failed: 0${NC}"
    echo ""
    echo -e "${GREEN}=========================================="
    echo "All tests passed! ✓"
    echo -e "==========================================${NC}"
    echo ""
fi
