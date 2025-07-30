#!/usr/bin/env bash
# Test script for syncfrom/from commands
# Tests basic syncfrom, directory sync, ignore functionality, and aliases

source "$(dirname "$0")/framework.bash"

init_test "syncfrom/from commands"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-syncfrom-$TEST_ID"
LOCAL_SETUP_DIR="setup-syncfrom-$TEST_ID"

step "Setting up test environment"
# Create local files to upload first (setup remote state)
create_test_dir "$LOCAL_SETUP_DIR" 2
track_local "$LOCAL_SETUP_DIR"
create_test_file "$LOCAL_SETUP_DIR/remote1.txt" "content of remote file 1"
create_test_file "$LOCAL_SETUP_DIR/remote2.txt" "content of remote file 2"
create_test_file "$LOCAL_SETUP_DIR/subdir/nested-remote.txt" "nested remote content"
create_test_file "$LOCAL_SETUP_DIR/data-remote.json" "remote json data"

# Upload to create remote state
assert "Setup remote files" ./fbcli upload "$LOCAL_SETUP_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

step "Testing basic syncfrom operation"
LOCAL_SYNC_DIR="synced-from-remote-$TEST_ID"
assert "Sync remote to local" ./fbcli syncfrom "$REMOTE_DIR/$LOCAL_SETUP_DIR" "$LOCAL_SYNC_DIR"
track_local "$LOCAL_SYNC_DIR"
assert_exists "Remote file1 synced locally" "$LOCAL_SYNC_DIR/remote1.txt"
assert_exists "Remote file2 synced locally" "$LOCAL_SYNC_DIR/remote2.txt"
assert_exists "Nested remote file synced locally" "$LOCAL_SYNC_DIR/subdir/nested-remote.txt"
assert_exists "JSON remote file synced locally" "$LOCAL_SYNC_DIR/data-remote.json"

step "Testing syncfrom with remote file modifications"
# Modify a remote file by uploading a new version
create_test_file "updated-remote1.txt" "updated remote content 1"
assert "Update remote file" ./fbcli upload "updated-remote1.txt" "$REMOTE_DIR/$LOCAL_SETUP_DIR/remote1.txt"
track_local "updated-remote1.txt"

# Add a new remote file
create_test_file "new-remote.txt" "new remote file content"
assert "Add new remote file" ./fbcli upload "new-remote.txt" "$REMOTE_DIR/$LOCAL_SETUP_DIR/"
track_local "new-remote.txt"

assert "Sync updated remote files" ./fbcli syncfrom "$REMOTE_DIR/$LOCAL_SETUP_DIR" "$LOCAL_SYNC_DIR"
assert_exists "Updated remote file synced" "$LOCAL_SYNC_DIR/remote1.txt"
assert_exists "New remote file synced" "$LOCAL_SYNC_DIR/new-remote.txt"

step "Testing syncfrom with ignore pattern"
# Setup remote files with different types
LOCAL_SETUP_DIR2="setup-ignore-syncfrom-$TEST_ID"
REMOTE_DIR2="/test-syncfrom-ignore-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR2" 2
track_local "$LOCAL_SETUP_DIR2"
create_test_file "$LOCAL_SETUP_DIR2/important-remote.txt" "important remote data"
create_test_file "$LOCAL_SETUP_DIR2/debug-remote.log" "remote debug info"
create_test_file "$LOCAL_SETUP_DIR2/error-remote.log" "remote error messages"
create_test_file "$LOCAL_SETUP_DIR2/config-remote.json" "remote configuration"

assert "Setup ignore test remote files" ./fbcli upload "$LOCAL_SETUP_DIR2" "$REMOTE_DIR2"
track_remote "$REMOTE_DIR2"

LOCAL_IGNORE_DIR="ignore-synced-$TEST_ID"
assert "Sync from remote with ignore pattern" ./fbcli syncfrom -i ".*\\.log$" "$REMOTE_DIR2/$LOCAL_SETUP_DIR2" "$LOCAL_IGNORE_DIR"
track_local "$LOCAL_IGNORE_DIR"
assert_exists "Important remote file synced" "$LOCAL_IGNORE_DIR/important-remote.txt"
assert_exists "Config remote file synced" "$LOCAL_IGNORE_DIR/config-remote.json"
assert_not_exists "Debug log ignored" "$LOCAL_IGNORE_DIR/debug-remote.log"
assert_not_exists "Error log ignored" "$LOCAL_IGNORE_DIR/error-remote.log"

step "Testing syncfrom with complex ignore patterns"
LOCAL_SETUP_DIR3="setup-complex-syncfrom-$TEST_ID"
REMOTE_DIR3="/test-syncfrom-complex-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR3" 2
track_local "$LOCAL_SETUP_DIR3"
create_test_file "$LOCAL_SETUP_DIR3/keep-remote.txt" "keep this remote file"
create_test_file "$LOCAL_SETUP_DIR3/temp-remote.tmp" "temporary remote file"
create_test_file "$LOCAL_SETUP_DIR3/backup-remote.bak" "backup remote file"
create_test_file "$LOCAL_SETUP_DIR3/document-remote.pdf" "remote pdf document"

assert "Setup complex ignore test files" ./fbcli upload "$LOCAL_SETUP_DIR3" "$REMOTE_DIR3"
track_remote "$REMOTE_DIR3"

LOCAL_COMPLEX_DIR="complex-synced-$TEST_ID"
# Ignore temp and backup files
assert "Sync from remote with complex ignore" ./fbcli syncfrom -i "(.*\\.tmp$|.*\\.bak$)" "$REMOTE_DIR3/$LOCAL_SETUP_DIR3" "$LOCAL_COMPLEX_DIR"
track_local "$LOCAL_COMPLEX_DIR"
assert_exists "Text file synced from remote" "$LOCAL_COMPLEX_DIR/keep-remote.txt"
assert_exists "PDF file synced from remote" "$LOCAL_COMPLEX_DIR/document-remote.pdf"
assert_not_exists "Temp file ignored" "$LOCAL_COMPLEX_DIR/temp-remote.tmp"
assert_not_exists "Backup file ignored" "$LOCAL_COMPLEX_DIR/backup-remote.bak"

step "Testing command aliases"
# Test 'from' alias
LOCAL_SETUP_DIR4="alias-setup-$TEST_ID"
REMOTE_DIR4="/test-from-alias-$TEST_ID"
create_test_file "$LOCAL_SETUP_DIR4/from-alias-test.txt" "testing from alias"
track_local "$LOCAL_SETUP_DIR4"
assert "Setup from alias test" ./fbcli upload "$LOCAL_SETUP_DIR4/from-alias-test.txt" "$REMOTE_DIR4/"
track_remote "$REMOTE_DIR4"

LOCAL_ALIAS_DIR="from-alias-test-$TEST_ID"
assert "from alias works" ./fbcli from "$REMOTE_DIR4" "$LOCAL_ALIAS_DIR"
track_local "$LOCAL_ALIAS_DIR"
assert_exists "File synced with from alias" "$LOCAL_ALIAS_DIR/from-alias-test.txt"

# Test 'from' alias with ignore
LOCAL_SETUP_DIR5="alias-ignore-setup-$TEST_ID"
REMOTE_DIR5="/test-from-alias-ignore-$TEST_ID"
create_test_dir "$LOCAL_SETUP_DIR5" 1
track_local "$LOCAL_SETUP_DIR5"
create_test_file "$LOCAL_SETUP_DIR5/include-remote.txt" "include this remote"
create_test_file "$LOCAL_SETUP_DIR5/ignore-remote.log" "ignore this remote"

assert "Setup from alias ignore test" ./fbcli upload "$LOCAL_SETUP_DIR5" "$REMOTE_DIR5"
track_remote "$REMOTE_DIR5"

LOCAL_ALIAS_IGNORE_DIR="from-alias-ignore-$TEST_ID"
assert "from alias with ignore works" ./fbcli from -i ".*\\.log$" "$REMOTE_DIR5/$LOCAL_SETUP_DIR5" "$LOCAL_ALIAS_IGNORE_DIR"
track_local "$LOCAL_ALIAS_IGNORE_DIR"
assert_exists "Include file synced with from alias" "$LOCAL_ALIAS_IGNORE_DIR/include-remote.txt"
assert_not_exists "Log file ignored with from alias" "$LOCAL_ALIAS_IGNORE_DIR/ignore-remote.log"

step "Testing error handling"
assert_fails "Sync fails with non-existent remote path" ./fbcli syncfrom "/non-existent-remote-$TEST_ID" "local-dir"
assert_fails "Sync fails with invalid local path" ./fbcli syncfrom "$REMOTE_DIR" ""

step "Testing flag order variations"
assert "Flags work: -i then paths" ./fbcli syncfrom -i ".*\\.log$" "$REMOTE_DIR2/$LOCAL_SETUP_DIR2" "$LOCAL_IGNORE_DIR"

finish_test
