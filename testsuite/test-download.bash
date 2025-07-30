#!/usr/bin/env bash
# Test script for download commands
# Tests basic download, directory download, and ignore functionality

source "$(dirname "$0")/framework.bash"

init_test "download commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-download-$TEST_ID"
LOCAL_SETUP_DIR="setup-download-$TEST_ID"

step "Setting up test environment"
# Create local files to upload first
create_test_dir "$LOCAL_SETUP_DIR" 2
track_local "$LOCAL_SETUP_DIR"
create_test_file "$LOCAL_SETUP_DIR/download-me.txt" "download this file"
create_test_file "$LOCAL_SETUP_DIR/another.txt" "another file"
create_test_file "$LOCAL_SETUP_DIR/subdir/nested.txt" "nested file"

# Upload test files to remote
assert "Upload test files to remote" ./fbcli upload "$LOCAL_SETUP_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

step "Testing single file download"
DOWNLOAD_FILE="downloaded-file-$TEST_ID.txt"
assert "Download single file" ./fbcli download "$REMOTE_DIR/$LOCAL_SETUP_DIR/download-me.txt" "$DOWNLOAD_FILE"
assert_exists "File downloaded successfully" "$DOWNLOAD_FILE"
track_local "$DOWNLOAD_FILE"

step "Testing directory download"
DOWNLOAD_DIR="downloaded-dir-$TEST_ID"
assert "Download directory" ./fbcli download "$REMOTE_DIR/$LOCAL_SETUP_DIR" "$DOWNLOAD_DIR.zip"
assert_exists "Directory downloaded as zip" "$DOWNLOAD_DIR.zip"
track_local "$DOWNLOAD_DIR.zip"

step "Testing download with ignore option"
LOCAL_SETUP_DIR2="setup-download-ignore-$TEST_ID"
REMOTE_DIR2="/test-download-ignore-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR2" 2
track_local "$LOCAL_SETUP_DIR2"
create_test_file "$LOCAL_SETUP_DIR2/include.txt" "include this"
create_test_file "$LOCAL_SETUP_DIR2/ignore.log" "ignore this log"
create_test_file "$LOCAL_SETUP_DIR2/keep.data" "keep this data"

assert "Upload ignore test files" ./fbcli upload "$LOCAL_SETUP_DIR2" "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"

DOWNLOAD_IGNORE_DIR="downloaded-ignore-$TEST_ID"
assert "Download with ignore pattern" ./fbcli download -i ".*\\.log$" "$REMOTE_DIR2/$LOCAL_SETUP_DIR2" "$DOWNLOAD_IGNORE_DIR.zip"
assert_exists "Ignored download completed" "$DOWNLOAD_IGNORE_DIR.zip"
track_local "$DOWNLOAD_IGNORE_DIR.zip"

step "Testing error handling"
assert_fails "Download fails on non-existent file" ./fbcli download "/non-existent-file-$TEST_ID.txt" "fail-download.txt"

step "Testing command aliases"
# Test down alias
DOWN_FILE="down-alias-$TEST_ID.txt"
assert "down alias works" ./fbcli down "$REMOTE_DIR/$LOCAL_SETUP_DIR/another.txt" "$DOWN_FILE"
assert_exists "File downloaded with down alias" "$DOWN_FILE"
track_local "$DOWN_FILE"

# Test dl alias  
DL_FILE="dl-alias-$TEST_ID.txt"
assert "dl alias works" ./fbcli dl "$REMOTE_DIR/$LOCAL_SETUP_DIR/another.txt" "$DL_FILE"
assert_exists "File downloaded with dl alias" "$DL_FILE"
track_local "$DL_FILE"

finish_test
