#!/usr/bin/env bash
# Test script for ls/list/dir commands
# Tests all aliases, ignore functionality, and error handling

source "$(dirname "$0")/framework.bash"

init_test "ls/list/dir commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-ls-$TEST_ID"
LOCAL_SETUP_DIR="setup-ls-$TEST_ID"

step "Setting up test environment"

# Create local files to upload first
create_test_dir "$LOCAL_SETUP_DIR" 3
create_test_file "$LOCAL_SETUP_DIR/file1.txt" "test file 1"
create_test_file "$LOCAL_SETUP_DIR/file2.txt" "test file 2"
create_test_file "$LOCAL_SETUP_DIR/ignore-me.log" "ignore this file"
create_test_file "$LOCAL_SETUP_DIR/subdir/nested.txt" "nested file"

# Upload test files to remote
assert "Upload test files to remote" ./fbcli upload "$LOCAL_SETUP_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

# Test 1: Basic ls command
step "Testing basic ls command"
assert "ls lists directory contents" ./fbcli ls "$REMOTE_DIR"

# Test 2: ls alias as 'list'
step "Testing ls alias as 'list'"
assert "list alias works" ./fbcli list "$REMOTE_DIR"

# Test 3: ls alias as 'dir'
step "Testing ls alias as 'dir'"
assert "dir alias works" ./fbcli dir "$REMOTE_DIR"

# Test 4: ls with ignore option
step "Testing ls with ignore option"
assert "ls -i works" ./fbcli ls -i ".*\\.log$" "$REMOTE_DIR"

# Test 5: list with ignore option
step "Testing list with ignore option"
assert "list -i works" ./fbcli list -i "ignore-.*" "$REMOTE_DIR"

# Test 6: dir with ignore option
step "Testing dir with ignore option"
assert "dir -i works" ./fbcli dir -i "ignore-.*" "$REMOTE_DIR"

# Test 7: Error handling - non-existent directory
step "Testing error handling"
assert_fails "ls fails on non-existent directory" ./fbcli ls "/non-existent-dir-$TEST_ID"
assert_fails "list fails on non-existent directory" ./fbcli list "/non-existent-dir-$TEST_ID"
assert_fails "dir fails on non-existent directory" ./fbcli dir "/non-existent-dir-$TEST_ID"

finish_test