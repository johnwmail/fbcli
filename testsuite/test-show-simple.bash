#!/usr/bin/env bash
# Test the show/help commands and configuration display.
# show must work offline (it does not log in) and must redact the password.

source "$(dirname "$0")/framework.bash"

init_test "show/help commands"

step "Testing show with the live configuration"
assert "show displays configuration" ./fbcli show
assert_contains "show contains version" "Version:" ./fbcli show
assert_contains "show contains URL" "URL:" ./fbcli show
assert_contains "show contains username" "Username:" ./fbcli show
assert_contains "show contains source" "Source:" ./fbcli show
assert_contains "show contains redacted password" "Password:" ./fbcli show
if [ "$FILEBROWSER_PASSWORD" != "$FILEBROWSER_USERNAME" ]; then
    assert_not_contains "show does not reveal the password" "$FILEBROWSER_PASSWORD" ./fbcli show
fi

step "Testing show without contacting the server"
# Port 1 is never a valid FileBrowser server; show must still succeed.
set +e
offline_output=$(
    FILEBROWSER_URL="http://127.0.0.1:1" \
    FILEBROWSER_USERNAME="offline-user" \
    FILEBROWSER_PASSWORD="offline-secret" \
    FILEBROWSER_SOURCE="offline-source" \
    ./fbcli show 2>&1
)
offline_status=$?
set -e
assert "show works with an unreachable URL" test "$offline_status" -eq 0
assert_contains "show reports the offline URL" "URL: http://127.0.0.1:1" echo "$offline_output"
assert_contains "show reports the offline source" "Source: offline-source" echo "$offline_output"
assert_not_contains "show redacts the offline password" "offline-secret" echo "$offline_output"

step "Testing --source on show"
assert_contains "show honours --source" "Source: review-source" ./fbcli show --source review-source

step "Testing help and version"
assert "help command works" ./fbcli help
assert_contains "help documents FILEBROWSER_SOURCE" "FILEBROWSER_SOURCE" ./fbcli --help
assert_contains "help documents --source" "--source" ./fbcli --help
assert "version command works" ./fbcli --version
assert_contains "version output" "fbcli version" ./fbcli version

finish_test
