#!/usr/bin/env bash
# Test script for upload commands
# Tests basic upload, directory upload, and ignore functionality

source "$(dirname "$0")/framework.bash"

init_test "upload commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-upload-$TEST_ID"

step "Testing single file upload"
LOCAL_FILE="test-file-$TEST_ID.txt"
create_test_file "$LOCAL_FILE" "test file content"
track_local "$LOCAL_FILE"

assert "Upload single file" ./fbcli upload "$LOCAL_FILE" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"
assert_remote_exists "File uploaded successfully" "$REMOTE_DIR/$LOCAL_FILE"

step "Testing directory upload"
LOCAL_DIR="test-dir-$TEST_ID"
create_test_dir "$LOCAL_DIR" 2
track_local "$LOCAL_DIR"
create_test_file "$LOCAL_DIR/file1.txt" "content 1"
create_test_file "$LOCAL_DIR/file2.txt" "content 2"
create_test_file "$LOCAL_DIR/subdir/nested.txt" "nested content"

REMOTE_DIR2="/test-upload-dir-$TEST_ID"
assert "Upload directory" ./fbcli upload "$LOCAL_DIR" "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"
assert_remote_exists "Directory uploaded" "$REMOTE_DIR2/$LOCAL_DIR"
assert_remote_exists "File1 uploaded" "$REMOTE_DIR2/$LOCAL_DIR/file1.txt"
assert_remote_exists "File2 uploaded" "$REMOTE_DIR2/$LOCAL_DIR/file2.txt"
assert_remote_exists "Nested file uploaded" "$REMOTE_DIR2/$LOCAL_DIR/subdir/nested.txt"

step "Testing upload with ignore option"
LOCAL_DIR2="test-ignore-$TEST_ID"
create_test_dir "$LOCAL_DIR2" 2
track_local "$LOCAL_DIR2"
create_test_file "$LOCAL_DIR2/include.txt" "include this"
create_test_file "$LOCAL_DIR2/ignore.log" "ignore this log"
create_test_file "$LOCAL_DIR2/keep.data" "keep this data"

REMOTE_DIR3="/test-upload-ignore-$TEST_ID"
assert "Upload with ignore pattern" ./fbcli upload -i ".*\\.log$" "$LOCAL_DIR2" "$REMOTE_DIR3"
track_remote "$REMOTE_DIR3"
assert_remote_exists "Include file uploaded" "$REMOTE_DIR3/$LOCAL_DIR2/include.txt"
assert_remote_exists "Data file uploaded" "$REMOTE_DIR3/$LOCAL_DIR2/keep.data"
assert_remote_not_exists "Log file ignored" "$REMOTE_DIR3/$LOCAL_DIR2/ignore.log"

step "Testing error handling"
assert_fails "Upload fails on non-existent file" ./fbcli upload "/non-existent-file-$TEST_ID.txt" "$REMOTE_DIR"

step "Testing command aliases"
# Test up alias
UP_FILE="up-alias-$TEST_ID.txt"
create_test_file "$UP_FILE" "testing up alias"
track_local "$UP_FILE"
UP_REMOTE="/test-up-alias-$TEST_ID"
assert "up alias works" ./fbcli up "$UP_FILE" "$UP_REMOTE/"
track_remote "$UP_REMOTE"
assert_remote_exists "File uploaded with up alias" "$UP_REMOTE/$UP_FILE"

finish_test