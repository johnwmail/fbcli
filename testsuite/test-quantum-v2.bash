#!/usr/bin/env bash
# FileBrowser Quantum v2 specific integration tests.
# Covers multi-source selection, query/path encoding, and v2 command semantics.

source "$(dirname "$0")/framework.bash"

init_test "FileBrowser Quantum v2 behaviour"

TEST_ID=$(gen_id)
REMOTE_DIR="/test-v2-$TEST_ID"

step "Testing configuration and source selection"
assert_contains "show includes a source line" "Source:" ./fbcli show
if [ "$FILEBROWSER_PASSWORD" != "$FILEBROWSER_USERNAME" ]; then
    assert_not_contains "show does not reveal the password" "$FILEBROWSER_PASSWORD" ./fbcli show
fi

# FBCLI_TEST_SOURCE is optional; CI sets it so the explicit flag/env paths are
# exercised without forcing every local run to know the source name.
if [ -n "${FBCLI_TEST_SOURCE:-}" ]; then
    assert_contains "show honours --source" "Source: ${FBCLI_TEST_SOURCE}" ./fbcli show --source "$FBCLI_TEST_SOURCE"
    assert_contains "show honours FILEBROWSER_SOURCE" "Source: ${FBCLI_TEST_SOURCE}" \
        env FILEBROWSER_SOURCE="$FBCLI_TEST_SOURCE" ./fbcli show
    assert "explicit --source is accepted" ./fbcli ls -s / --source "$FBCLI_TEST_SOURCE"
    assert "FILEBROWSER_SOURCE is accepted" env FILEBROWSER_SOURCE="$FBCLI_TEST_SOURCE" ./fbcli ls -s /
else
    step "FBCLI_TEST_SOURCE not set; skipping explicit valid-source checks"
fi

assert_fails "invalid --source is rejected" ./fbcli ls -s / --source "missing-source-$TEST_ID"
assert_fails "invalid FILEBROWSER_SOURCE is rejected" \
    env FILEBROWSER_SOURCE="missing-source-$TEST_ID" ./fbcli ls -s /

step "Testing special characters in remote paths"
SPECIAL_DIR="setup-special-$TEST_ID"
mkdir -p "$SPECIAL_DIR"
track_local "$SPECIAL_DIR"
create_test_file "$SPECIAL_DIR/plain file.txt" "space content"
create_test_file "$SPECIAL_DIR/weird #%&? ünïcode-$TEST_ID.txt" "weird content"

assert "Upload a directory with special file names" ./fbcli upload "$SPECIAL_DIR" "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

assert_remote_content "Space file content synced" "$REMOTE_DIR/$SPECIAL_DIR/plain file.txt" "space content"
assert_remote_content "Special-character file content synced" \
    "$REMOTE_DIR/$SPECIAL_DIR/weird #%&? ünïcode-$TEST_ID.txt" "weird content"
assert_contains "ls -s shows the space file" "plain file.txt" ./fbcli ls -s "$REMOTE_DIR/$SPECIAL_DIR"
assert_contains "ls -s shows the unicode file" "ünïcode-$TEST_ID.txt" ./fbcli ls -s "$REMOTE_DIR/$SPECIAL_DIR"

SPECIAL_DOWNLOAD="special-download-$TEST_ID.txt"
assert "Download a special-character file" \
    ./fbcli download "$REMOTE_DIR/$SPECIAL_DIR/weird #%&? ünïcode-$TEST_ID.txt" "$SPECIAL_DOWNLOAD"
track_local "$SPECIAL_DOWNLOAD"
assert_content "Special-character download content matches" "$SPECIAL_DOWNLOAD" "weird content"

SPECIAL_RENAMED="$REMOTE_DIR/$SPECIAL_DIR/renamed file #2.txt"
assert "Rename a special-character file" \
    ./fbcli rename "$REMOTE_DIR/$SPECIAL_DIR/plain file.txt" "$SPECIAL_RENAMED"
assert_remote_not_exists "Old special-character name is gone" "$REMOTE_DIR/$SPECIAL_DIR/plain file.txt"
assert_remote_content "Renamed special-character content preserved" "$SPECIAL_RENAMED" "space content"

step "Testing -- end-of-options handling"
DASH_FILE="-dash-file-$TEST_ID.txt"
create_test_file "$DASH_FILE" "dash content"
assert "Upload a file whose name starts with a dash" ./fbcli upload -- "$DASH_FILE" "$REMOTE_DIR"
assert_remote_content "Dash-prefixed file uploaded" "$REMOTE_DIR/$DASH_FILE" "dash content"
assert "ls accepts -- before a path" ./fbcli ls -s -- "$REMOTE_DIR"

step "Testing v2 upload-to-existing-file semantics"
OVERWRITE_FILE="v2-overwrite-$TEST_ID.txt"
create_test_file "$OVERWRITE_FILE" "initial v2 content"
assert "Upload the initial file" ./fbcli upload "$OVERWRITE_FILE" "$REMOTE_DIR"
create_test_file "$OVERWRITE_FILE" "replaced v2 content"
assert "Upload directly to the existing remote file" \
    ./fbcli upload "$OVERWRITE_FILE" "$REMOTE_DIR/$OVERWRITE_FILE"
assert_remote_content "Existing remote file was replaced" "$REMOTE_DIR/$OVERWRITE_FILE" "replaced v2 content"

step "Testing command validation"
assert_fails "unknown command is rejected" ./fbcli not-a-real-command
assert_fails "upload without arguments is rejected" ./fbcli upload
assert_fails "download -z with -i is rejected" \
    ./fbcli download -z -i ".*" "$REMOTE_DIR" "conflict-$TEST_ID"
assert_contains "help mentions FILEBROWSER_SOURCE" "FILEBROWSER_SOURCE" ./fbcli --help
assert_contains "help mentions --source" "--source" ./fbcli --help

finish_test
