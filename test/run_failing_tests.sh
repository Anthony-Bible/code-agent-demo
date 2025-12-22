#!/bin/bash

echo "=== Testing Red Phase ==="
echo "Running domain entity tests - they should FAIL (Red Phase)"
echo ""

# Change to project root
cd "$(dirname "$0")/.."

# Run the tests and collect exit code
go test ./test/internal/domain/ -v
TEST_EXIT_CODE=$?

echo ""
if [ $TEST_EXIT_CODE -ne 0 ]; then
    echo "SUCCESS: Tests are failing as expected in Red Phase! ✓"
    echo "Exit code: $TEST_EXIT_CODE"
    exit 0
else
    echo "ERROR: Tests should be failing in Red Phase but they passed!"
    echo "Exit code: $TEST_EXIT_CODE"
    exit 1
fi