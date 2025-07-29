#!/usr/bin/env bash
# Minimal mkdir test to isolate the issue

source "$(dirname "$0")/framework.bash"

init_test "minimal mkdir test"

# Test variables
TEST_ID=$(gen_id)
REMOTE_DIR="/test-minimal-$TEST_ID"

echo "DEBUG: TEST_ID = $TEST_ID"
echo "DEBUG: REMOTE_DIR = $REMOTE_DIR"

step "Testing simple mkdir"
echo "DEBUG: About to run assert for mkdir command"
assert "mkdir creates directory" ./fbcli mkdir "$REMOTE_DIR"
echo "DEBUG: mkdir assert completed successfully!"
track_remote "$REMOTE_DIR"
echo "DEBUG: tracked remote path"

echo "DEBUG: About to test if directory exists"
if ./fbcli ls / | grep -q "$(basename "$REMOTE_DIR")"; then
    echo "DEBUG: directory exists"
    echo "✓ Directory was created successfully"
else
    echo "DEBUG: directory not found"
    echo "✗ Directory was not created"
    exit 1
fi

finish_test
