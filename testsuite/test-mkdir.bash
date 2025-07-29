#!/usr/bin/env bash
# Test script for mkdir/md commands
# Tests directory creation and aliases

source "$(dirname "$0")/framework.bash"

init_test "mkdir/md commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)

step "Testing basic mkdir command"
REMOTE_DIR="/test-mkdir-$TEST_ID"
assert "mkdir creates directory" ./fbcli mkdir "$REMOTE_DIR"
track_remote "$REMOTE_DIR"
assert_remote_exists "Directory created successfully" "$REMOTE_DIR"

step "Testing md alias"
REMOTE_DIR2="/test-md-$TEST_ID"
assert "md alias works" ./fbcli md "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"
assert_remote_exists "Directory created with md" "$REMOTE_DIR2"

step "Testing nested directory creation"
NESTED_DIR="/test-nested-$TEST_ID/subdir/deep"
assert "mkdir creates nested directory" ./fbcli mkdir "$NESTED_DIR"
track_remote "/test-nested-$TEST_ID"
assert_remote_exists "Nested directory created" "$NESTED_DIR"

step "Testing error handling"
assert_fails "mkdir fails on invalid path" ./fbcli mkdir ""
assert_fails "mkdir fails on root directory" ./fbcli mkdir "/"

# Test that duplicate directory creation succeeds (409 response is handled)
step "Testing duplicate directory creation"
assert "mkdir handles existing directory" ./fbcli mkdir "$REMOTE_DIR"

finish_test
