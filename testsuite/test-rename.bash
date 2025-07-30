#!/usr/bin/env bash
# Test script for rename/mv commands
# Tests file and directory renaming

source "$(dirname "$0")/framework.bash"

init_test "rename/mv commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-rename-$TEST_ID"
LOCAL_SETUP_DIR="setup-rename-$TEST_ID"

step "Setting up test environment"
# Create local files to upload first
create_test_dir "$LOCAL_SETUP_DIR" 2
track_local "$LOCAL_SETUP_DIR"
create_test_file "$LOCAL_SETUP_DIR/original.txt" "original file content"
create_test_file "$LOCAL_SETUP_DIR/subdir/nested.txt" "nested file"

# Upload test files to remote
assert "Upload test files to remote" ./fbcli upload "$LOCAL_SETUP_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

step "Testing file rename with rename command"
OLD_FILE="$REMOTE_DIR/$LOCAL_SETUP_DIR/original.txt"
NEW_FILE="$REMOTE_DIR/$LOCAL_SETUP_DIR/renamed.txt"
assert_remote_exists "Original file exists" "$OLD_FILE"
assert "rename command works" ./fbcli rename "$OLD_FILE" "$NEW_FILE"
assert_remote_not_exists "Original file moved" "$OLD_FILE"
assert_remote_exists "File renamed successfully" "$NEW_FILE"

step "Testing file rename with mv alias"
OLD_FILE2="$REMOTE_DIR/$LOCAL_SETUP_DIR/subdir/nested.txt"
NEW_FILE2="$REMOTE_DIR/$LOCAL_SETUP_DIR/subdir/moved.txt"
assert_remote_exists "Nested file exists" "$OLD_FILE2"
assert "mv alias works" ./fbcli mv "$OLD_FILE2" "$NEW_FILE2"
assert_remote_not_exists "Original nested file moved" "$OLD_FILE2"
assert_remote_exists "Nested file renamed" "$NEW_FILE2"

step "Testing directory rename"
# Create another test setup for directory rename
LOCAL_SETUP_DIR2="setup-rename-dir-$TEST_ID"
REMOTE_DIR2="/test-rename-dir-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR2" 1
create_test_file "$LOCAL_SETUP_DIR2/content.txt" "directory content"

assert "Upload directory test files" ./fbcli upload "$LOCAL_SETUP_DIR2" "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"

OLD_DIR="$REMOTE_DIR2/$LOCAL_SETUP_DIR2"
NEW_DIR="$REMOTE_DIR2/renamed-dir-$TEST_ID"
assert "rename directory works" ./fbcli rename "$OLD_DIR" "$NEW_DIR"
assert_remote_not_exists "Original directory moved" "$OLD_DIR"
assert_remote_exists "Directory renamed successfully" "$NEW_DIR"
assert_remote_exists "Directory content preserved" "$NEW_DIR/content.txt"

step "Testing error handling"
assert_fails "rename fails on non-existent file" ./fbcli rename "/non-existent-$TEST_ID.txt" "/also-non-existent-$TEST_ID.txt"
assert_fails "mv fails on non-existent file" ./fbcli mv "/non-existent-$TEST_ID.txt" "/also-non-existent-$TEST_ID.txt"

finish_test
