#!/usr/bin/env bash
# Test script for show command
# Tests configuration display

source "$(dirname "$0")/framework.bash"

init_test "show command"

step "Testing show command"
assert "show displays configuration" ./fbcli show
assert_contains "show contains version" "Version:" ./fbcli show
assert_contains "show contains URL" "URL:" ./fbcli show

finish_test
