#!/usr/bin/env bash
# Test script for mkdir/md commands
# Tests all aliases and multiple directory creation

source "$(dirname "$0")/framework.bash"

init_test "mkdir/md commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR1="/test-mkdir1-$TEST_ID"
REMOTE_DIR2="/test-mkdir2-$TEST_ID"
REMOTE_DIR3="/test-mkdir3-$TEST_ID"
REMOTE_NESTED="/test-nested-$TEST_ID/subdir/deep"

step "Testing mkdir command functionality"

# Test 1: Basic mkdir
step "Testing basic mkdir"
assert "mkdir creates directory" ./fbcli mkdir "$REMOTE_DIR1"
track_remote "$REMOTE_DIR1"
assert_remote_exists "Directory exists after mkdir" "$REMOTE_DIR1"

# Test 2: md alias
step "Testing md alias"
assert "md alias works" ./fbcli md "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"
assert_remote_exists "Directory exists after md" "$REMOTE_DIR2"

# Test 3: Multiple directories at once
step "Testing multiple directory creation"
assert "mkdir multiple directories" ./fbcli mkdir "$REMOTE_DIR3" "/test-multi1-$TEST_ID" "/test-multi2-$TEST_ID"
track_remote "$REMOTE_DIR3"
track_remote "/test-multi1-$TEST_ID"
track_remote "/test-multi2-$TEST_ID"
assert_remote_exists "First directory exists" "$REMOTE_DIR3"
assert_remote_exists "Second directory exists" "/test-multi1-$TEST_ID"
assert_remote_exists "Third directory exists" "/test-multi2-$TEST_ID"

# Test 4: md with multiple directories
step "Testing md with multiple directories"
assert "md multiple directories" ./fbcli md "/test-md-multi1-$TEST_ID" "/test-md-multi2-$TEST_ID"
track_remote "/test-md-multi1-$TEST_ID"
track_remote "/test-md-multi2-$TEST_ID"
assert_remote_exists "md multi dir 1 exists" "/test-md-multi1-$TEST_ID"
assert_remote_exists "md multi dir 2 exists" "/test-md-multi2-$TEST_ID"

# Test 5: Nested directory creation
step "Testing nested directory creation"
assert "mkdir creates nested directories" ./fbcli mkdir "$REMOTE_NESTED"
track_remote "/test-nested-$TEST_ID"
assert_remote_exists "Nested directory exists" "$REMOTE_NESTED"

# Test 6: Error handling - already exists
step "Testing directory already exists"
assert "mkdir on existing directory succeeds" ./fbcli mkdir "$REMOTE_DIR1"

# Test 7: Error handling - invalid path
step "Testing invalid path"
assert_fails "mkdir fails with invalid path" ./fbcli mkdir "/invalid/deeply/nested/path/that/should/fail"

finish_test
