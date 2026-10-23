#!/usr/bin/env bash
# Tests the sftp-style interactive shell in batch mode (stdin is not a TTY).

source "$(dirname "$0")/framework.bash"

init_test "interactive shell (batch mode)"

TEST_ID=$(gen_id)
REMOTE_DIR="/test-shell-$TEST_ID"
LOCAL_DIR="shell-local-$TEST_ID"
LOCAL_GET="shell-get-$TEST_ID"
LOCAL_GET_ZIP="shell-get-$TEST_ID.zip"

create_test_dir "$LOCAL_DIR" 1
create_test_file "$LOCAL_DIR/data.txt" "shell data"
create_test_file "shell-upload-$TEST_ID.txt" "uploaded via shell"
track_local "shell-upload-$TEST_ID.txt"

# Run a sequence of shell commands through stdin.
run_shell() {
    printf '%s\n' "$@" | ./fbcli shell
}

step "Testing basic shell commands"
assert_contains "pwd prints the remote root" "/" run_shell "pwd" "exit"
assert_contains "help lists get" "get" run_shell "help" "exit"
assert_contains "help lists put" "put" run_shell "help" "exit"
assert_contains "version works in the shell" "fbcli version" run_shell "version" "exit"

step "Testing remote directory context"
track_remote "$REMOTE_DIR"
assert "shell mkdir works" run_shell "mkdir $REMOTE_DIR" "exit"
assert_remote_exists "shell created the remote directory" "$REMOTE_DIR"
assert_contains "shell pwd after cd" "$REMOTE_DIR" run_shell "cd $REMOTE_DIR" "pwd" "exit"
assert "shell relative mkdir works" run_shell "cd $REMOTE_DIR" "mkdir sub" "exit"
assert_remote_exists "shell created a relative directory" "$REMOTE_DIR/sub"

step "Testing put and get"
assert "shell put file works" run_shell "cd $REMOTE_DIR" "put shell-upload-$TEST_ID.txt" "exit"
assert_remote_content "shell put content matches" "$REMOTE_DIR/shell-upload-$TEST_ID.txt" "uploaded via shell"

LOCAL_DOWNLOAD="shell-download-$TEST_ID.txt"
track_local "$LOCAL_DOWNLOAD"
assert "shell get file works" run_shell "cd $REMOTE_DIR" "get shell-upload-$TEST_ID.txt $LOCAL_DOWNLOAD" "exit"
assert_content "shell get content matches" "$LOCAL_DOWNLOAD" "uploaded via shell"

step "Testing recursive put and get"
assert "shell put directory works" run_shell "cd $REMOTE_DIR" "put -r $LOCAL_DIR" "exit"
assert_remote_content "shell recursive put content matches" "$REMOTE_DIR/$LOCAL_DIR/data.txt" "shell data"

track_local "$LOCAL_GET"
assert "shell get directory works" run_shell "cd $REMOTE_DIR" "get -r $LOCAL_DIR $LOCAL_GET" "exit"
assert_exists "shell recursive get created the directory" "$LOCAL_GET"
assert_content "shell recursive get content matches" "$LOCAL_GET/data.txt" "shell data"

track_local "$LOCAL_GET_ZIP"
assert "shell get -z works" run_shell "cd $REMOTE_DIR" "get -z $LOCAL_DIR $LOCAL_GET_ZIP" "exit"
assert_exists "shell zip download exists" "$LOCAL_GET_ZIP"
assert "shell zip download is not empty" test -s "$LOCAL_GET_ZIP"

step "Testing rename and rm"
assert "shell rename works" run_shell "cd $REMOTE_DIR" "rename shell-upload-$TEST_ID.txt renamed.txt" "exit"
assert_remote_not_exists "shell rename removed the old name" "$REMOTE_DIR/shell-upload-$TEST_ID.txt"
assert_remote_content "shell rename preserved content" "$REMOTE_DIR/renamed.txt" "uploaded via shell"

assert "shell rm works" run_shell "cd $REMOTE_DIR" "rm renamed.txt" "exit"
assert_remote_not_exists "shell rm deleted the file" "$REMOTE_DIR/renamed.txt"

step "Testing local context and local commands"
assert_contains "lls lists the local fixture" "$LOCAL_DIR" run_shell "lls" "exit"
assert_contains "! runs a local command" "shell-local-ok" run_shell "!echo shell-local-ok" "exit"
assert "lcd works" run_shell "lcd $LOCAL_DIR" "lpwd" "exit"

step "Testing batch error handling"
assert_fails "shell returns non-zero when a command fails" run_shell "cd /missing-shell-dir-$TEST_ID" "exit"
assert_fails "shell rejects an unknown command" run_shell "not-a-real-shell-command" "exit"

finish_test
