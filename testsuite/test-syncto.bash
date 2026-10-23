#!/usr/bin/env bash
# Test script for syncto/to commands
# Tests basic syncto, directory sync, ignore functionality, and aliases

source "$(dirname "$0")/framework.bash"

init_test "syncto/to commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-syncto-$TEST_ID"

step "Setting up test environment"
# Create local directory structure for syncing
LOCAL_DIR="local-sync-$TEST_ID"
create_test_dir "$LOCAL_DIR" 2
track_local "$LOCAL_DIR"
create_test_file "$LOCAL_DIR/file1.txt" "content of file1"
create_test_file "$LOCAL_DIR/file2.txt" "content of file2"
create_test_file "$LOCAL_DIR/subdir/nested.txt" "nested content"
create_test_file "$LOCAL_DIR/data.json" "json data"

step "Testing basic syncto operation"
assert "Sync local to remote" ./fbcli syncto "$LOCAL_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"
assert_remote_exists "File1 synced" "$REMOTE_DIR/file1.txt"
assert_remote_exists "File2 synced" "$REMOTE_DIR/file2.txt"
assert_remote_exists "Nested file synced" "$REMOTE_DIR/subdir/nested.txt"
assert_remote_exists "JSON file synced" "$REMOTE_DIR/data.json"
assert_remote_content "File1 content synced" "$REMOTE_DIR/file1.txt" "content of file1"
assert_remote_content "Nested content synced" "$REMOTE_DIR/subdir/nested.txt" "nested content"
assert_remote_content "JSON content synced" "$REMOTE_DIR/data.json" "json data"

step "Testing repeat sync detects unchanged files"
assert_contains "Repeat sync reports in-sync files" "already in sync" ./fbcli syncto "$LOCAL_DIR" "$REMOTE_DIR"

step "Testing remote-only entries are deleted"
REMOTE_EXTRA_LOCAL="remote-extra-$TEST_ID.txt"
create_test_file "$REMOTE_EXTRA_LOCAL" "remote only"
assert "Upload a remote-only file" ./fbcli upload "$REMOTE_EXTRA_LOCAL" "$REMOTE_DIR"
assert_remote_exists "Remote-only file exists before sync" "$REMOTE_DIR/$REMOTE_EXTRA_LOCAL"
assert "Sync removes remote-only entries" ./fbcli syncto "$LOCAL_DIR" "$REMOTE_DIR"
assert_remote_not_exists "Remote-only file deleted by sync" "$REMOTE_DIR/$REMOTE_EXTRA_LOCAL"

step "Testing syncto with file modifications"
# Modify a local file and add a new one
create_test_file "$LOCAL_DIR/file1.txt" "updated content of file1"
create_test_file "$LOCAL_DIR/new-file.txt" "new file content"

assert "Sync updated files" ./fbcli syncto "$LOCAL_DIR" "$REMOTE_DIR"
assert_remote_exists "Updated file synced" "$REMOTE_DIR/file1.txt"
assert_remote_exists "New file synced" "$REMOTE_DIR/new-file.txt"
assert_remote_content "Updated file content synced" "$REMOTE_DIR/file1.txt" "updated content of file1"
assert_remote_content "New file content synced" "$REMOTE_DIR/new-file.txt" "new file content"

step "Testing syncto with a single file"
SINGLE_LOCAL="single-$TEST_ID.txt"
SINGLE_REMOTE_DIR="/test-syncto-single-$TEST_ID"
create_test_file "$SINGLE_LOCAL" "single file sync"
assert "Create single-file remote directory" ./fbcli mkdir "$SINGLE_REMOTE_DIR"
track_remote "$SINGLE_REMOTE_DIR"
assert "Sync a single local file" ./fbcli syncto "$SINGLE_LOCAL" "$SINGLE_REMOTE_DIR"
assert_remote_exists "Single file synced" "$SINGLE_REMOTE_DIR/$SINGLE_LOCAL"
assert_remote_content "Single file content synced" "$SINGLE_REMOTE_DIR/$SINGLE_LOCAL" "single file sync"

step "Testing syncto with ignore pattern"
LOCAL_DIR2="local-ignore-sync-$TEST_ID"
REMOTE_DIR2="/test-syncto-ignore-$TEST_ID"
create_test_dir "$LOCAL_DIR2" 2
track_local "$LOCAL_DIR2"
create_test_file "$LOCAL_DIR2/important.txt" "important data"
create_test_file "$LOCAL_DIR2/debug.log" "debug information"
create_test_file "$LOCAL_DIR2/error.log" "error messages"
create_test_file "$LOCAL_DIR2/config.json" "configuration"

assert "Sync with ignore pattern" ./fbcli syncto -i ".*\\.log$" "$LOCAL_DIR2" "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"
assert_remote_exists "Important file synced" "$REMOTE_DIR2/important.txt"
assert_remote_exists "Config file synced" "$REMOTE_DIR2/config.json"
assert_remote_not_exists "Debug log ignored" "$REMOTE_DIR2/debug.log"
assert_remote_not_exists "Error log ignored" "$REMOTE_DIR2/error.log"
assert_remote_content "Important content synced" "$REMOTE_DIR2/important.txt" "important data"

step "Testing syncto with complex ignore patterns"
LOCAL_DIR3="local-complex-sync-$TEST_ID"
REMOTE_DIR3="/test-syncto-complex-$TEST_ID"
create_test_dir "$LOCAL_DIR3" 2
track_local "$LOCAL_DIR3"
create_test_file "$LOCAL_DIR3/keep-this.txt" "keep this file"
create_test_file "$LOCAL_DIR3/temp-file.tmp" "temporary file"
create_test_file "$LOCAL_DIR3/backup-old.bak" "backup file"
create_test_file "$LOCAL_DIR3/document.pdf" "pdf document"

# Ignore temp and backup files
assert "Sync with complex ignore pattern" ./fbcli syncto -i "(.*\\.tmp$|.*\\.bak$)" "$LOCAL_DIR3" "$REMOTE_DIR3"
track_remote "$REMOTE_DIR3"
assert_remote_exists "Text file synced" "$REMOTE_DIR3/keep-this.txt"
assert_remote_exists "PDF file synced" "$REMOTE_DIR3/document.pdf"
assert_remote_not_exists "Temp file ignored" "$REMOTE_DIR3/temp-file.tmp"
assert_remote_not_exists "Backup file ignored" "$REMOTE_DIR3/backup-old.bak"
assert_remote_content "Complex sync content matches" "$REMOTE_DIR3/keep-this.txt" "keep this file"

step "Testing command aliases"
# Test 'to' alias
LOCAL_DIR4="alias-test-$TEST_ID"
REMOTE_DIR4="/test-to-alias-$TEST_ID"
create_test_dir "$LOCAL_DIR4" 1
track_local "$LOCAL_DIR4"
create_test_file "$LOCAL_DIR4/alias-test.txt" "testing to alias"

assert "to alias works" ./fbcli to "$LOCAL_DIR4" "$REMOTE_DIR4"
track_remote "$REMOTE_DIR4"
assert_remote_exists "File synced with to alias" "$REMOTE_DIR4/alias-test.txt"
assert_remote_content "To alias content synced" "$REMOTE_DIR4/alias-test.txt" "testing to alias"

# Test 'to' alias with ignore
LOCAL_DIR5="alias-ignore-test-$TEST_ID"
REMOTE_DIR5="/test-to-alias-ignore-$TEST_ID"
create_test_dir "$LOCAL_DIR5" 1
track_local "$LOCAL_DIR5"
create_test_file "$LOCAL_DIR5/include.txt" "include this"
create_test_file "$LOCAL_DIR5/ignore-this.log" "ignore this"

assert "to alias with ignore works" ./fbcli to -i ".*\\.log$" "$LOCAL_DIR5" "$REMOTE_DIR5"
track_remote "$REMOTE_DIR5"
assert_remote_exists "Include file synced with to alias" "$REMOTE_DIR5/include.txt"
assert_remote_not_exists "Log file ignored with to alias" "$REMOTE_DIR5/ignore-this.log"
assert_remote_content "To alias ignore content synced" "$REMOTE_DIR5/include.txt" "include this"

step "Testing error handling"
assert_fails "Sync fails with non-existent local path" ./fbcli syncto "/non-existent-$TEST_ID" "$REMOTE_DIR"

step "Testing flag order variations"
assert "Flags work: -i then path" ./fbcli syncto -i ".*\\.log$" "$LOCAL_DIR2" "$REMOTE_DIR2"
# Test reverse order is handled by argument parsing

finish_test
