#!/usr/bin/env bash
# Test script for rm/delete commands
# Tests all aliases, ignore functionality, and multiple file deletion

source "$(dirname "$0")/framework.bash"

init_test "rm/delete commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-rm-$TEST_ID"
LOCAL_SETUP_DIR="setup-rm-$TEST_ID"

step "Setting up test environment"

# Create local files to upload first
create_test_dir "$LOCAL_SETUP_DIR" 3
track_local "$LOCAL_SETUP_DIR"
create_test_file "$LOCAL_SETUP_DIR/ignore-me.txt" "ignore this file"
create_test_file "$LOCAL_SETUP_DIR/delete-me1.txt" "delete this file 1"
create_test_file "$LOCAL_SETUP_DIR/delete-me2.txt" "delete this file 2"
create_test_file "$LOCAL_SETUP_DIR/keep-me.txt" "keep this file"
create_test_file "$LOCAL_SETUP_DIR/subdir/nested.txt" "nested file"

# Upload test files to remote
assert "Upload test files to remote" ./fbcli upload "$LOCAL_SETUP_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

# Test 1: Delete single file with rm
step "Testing single file deletion with rm"
FILE_TO_DELETE="$REMOTE_DIR/$LOCAL_SETUP_DIR/delete-me1.txt"
assert_remote_exists "File exists before deletion" "$FILE_TO_DELETE"
assert "rm deletes single file" ./fbcli rm "$FILE_TO_DELETE"
assert_remote_not_exists "File deleted with rm" "$FILE_TO_DELETE"

# Test 2: Delete single file with delete alias
step "Testing single file deletion with delete alias"
FILE_TO_DELETE2="$REMOTE_DIR/$LOCAL_SETUP_DIR/delete-me2.txt"
assert_remote_exists "File exists before deletion" "$FILE_TO_DELETE2"
assert "delete alias works" ./fbcli delete "$FILE_TO_DELETE2"
assert_remote_not_exists "File deleted with delete alias" "$FILE_TO_DELETE2"

# Recreate test environment for multiple file tests
step "Recreating test environment for multi-file tests"
LOCAL_SETUP_DIR2="setup-rm2-$TEST_ID"
REMOTE_DIR2="/test-rm2-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR2" 2
create_test_file "$LOCAL_SETUP_DIR2/multi1.txt" "multi delete 1"
create_test_file "$LOCAL_SETUP_DIR2/multi2.txt" "multi delete 2"
create_test_file "$LOCAL_SETUP_DIR2/multi3.txt" "multi delete 3"

assert "Upload multi-delete test files" ./fbcli upload "$LOCAL_SETUP_DIR2" "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"

# Test 3: Delete multiple files with rm
step "Testing multiple file deletion with rm"
FILE1="$REMOTE_DIR2/$LOCAL_SETUP_DIR2/multi1.txt"
FILE2="$REMOTE_DIR2/$LOCAL_SETUP_DIR2/multi2.txt"
assert "rm deletes multiple files" ./fbcli rm "$FILE1" "$FILE2"
assert_remote_not_exists "First file deleted" "$FILE1"
assert_remote_not_exists "Second file deleted" "$FILE2"
assert_remote_exists "Third file still exists" "$REMOTE_DIR2/$LOCAL_SETUP_DIR2/multi3.txt"

# Test 4: Delete multiple files with delete alias
step "Testing multiple file deletion with delete alias"
LOCAL_SETUP_DIR3="setup-delete-multi-$TEST_ID"
REMOTE_DIR3="/test-delete-multi-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR3" 2
create_test_file "$LOCAL_SETUP_DIR3/del1.txt" "delete multi 1"
create_test_file "$LOCAL_SETUP_DIR3/del2.txt" "delete multi 2"

assert "Upload delete-multi test files" ./fbcli upload "$LOCAL_SETUP_DIR3" "$REMOTE_DIR3"
track_remote "$REMOTE_DIR3"

DELFILE1="$REMOTE_DIR3/$LOCAL_SETUP_DIR3/del1.txt"
DELFILE2="$REMOTE_DIR3/$LOCAL_SETUP_DIR3/del2.txt"
assert "delete alias works with multiple files" ./fbcli delete "$DELFILE1" "$DELFILE2"
assert_remote_not_exists "First file deleted with delete" "$DELFILE1"
assert_remote_not_exists "Second file deleted with delete" "$DELFILE2"

# Test 5: Delete directory with rm
step "Testing directory deletion with rm"
REMOTE_SUBDIR="$REMOTE_DIR/$LOCAL_SETUP_DIR/subdir"
assert_remote_exists "Subdirectory exists before deletion" "$REMOTE_SUBDIR"
assert "rm deletes directory" ./fbcli rm "$REMOTE_SUBDIR"
assert_remote_not_exists "Directory deleted with rm" "$REMOTE_SUBDIR"

# Test 6: rm with ignore option
step "Testing rm with ignore option"
LOCAL_SETUP_DIR4="setup-rm-ignore-$TEST_ID"
REMOTE_DIR4="/test-rm-ignore-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR4" 2
create_test_file "$LOCAL_SETUP_DIR4/ignore-me.txt" "do not delete"
create_test_file "$LOCAL_SETUP_DIR4/delete-me.txt" "ok to delete"

assert "Upload ignore test files" ./fbcli upload "$LOCAL_SETUP_DIR4" "$REMOTE_DIR4"
track_remote "$REMOTE_DIR4"

assert "rm -i works" ./fbcli rm -i "ignore-me.txt" "$REMOTE_DIR4/$LOCAL_SETUP_DIR4"
assert_remote_not_exists "Non-ignored files deleted" "$REMOTE_DIR4/$LOCAL_SETUP_DIR4/delete-me.txt"
assert_remote_exists "Ignored file preserved" "$REMOTE_DIR4/$LOCAL_SETUP_DIR4/ignore-me.txt"

# Test 7: delete with ignore option
step "Testing delete with ignore option"
LOCAL_SETUP_DIR5="setup-delete-ignore-$TEST_ID"
REMOTE_DIR5="/test-delete-ignore-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR5" 1
create_test_file "$LOCAL_SETUP_DIR5/ignore-pattern.log" "ignore this log"
create_test_file "$LOCAL_SETUP_DIR5/delete-this.txt" "delete this"

assert "Upload delete-ignore test files" ./fbcli upload "$LOCAL_SETUP_DIR5" "$REMOTE_DIR5"
track_remote "$REMOTE_DIR5"

assert "delete -i works" ./fbcli delete -i ".*\\.log$" "$REMOTE_DIR5/$LOCAL_SETUP_DIR5"
assert_remote_not_exists "Non-ignored file deleted with delete -i" "$REMOTE_DIR5/$LOCAL_SETUP_DIR5/delete-this.txt"
assert_remote_exists "Log file ignored with delete -i" "$REMOTE_DIR5/$LOCAL_SETUP_DIR5/ignore-pattern.log"

# Test 8: Error handling
step "Testing error handling"
assert_fails "rm fails on non-existent file" ./fbcli rm "/non-existent-file-$TEST_ID.txt"
assert_fails "delete fails on non-existent file" ./fbcli delete "/non-existent-file-$TEST_ID.txt"

finish_test
