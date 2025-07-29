#!/usr/bin/env bash
# Debug test to isolate the issue

# Clear any existing color variables
unset RED GREEN YELLOW BLUE PURPLE CYAN NC 2>/dev/null || true

source "$(dirname "$0")/framework.bash"

init_test "debug test"

# Test simple commands directly without assert
step "Testing commands directly"
echo "Testing ./fbcli ls:"
./fbcli ls >/dev/null
echo "Exit code: $?"

echo "Testing ./fbcli show:"
./fbcli show >/dev/null
echo "Exit code: $?"

# Now test with assert
step "Testing with assert"
assert "ls root directory works" ./fbcli ls
echo "After assert, continuing..."

step "Another test"
assert "show command works" ./fbcli show
echo "After second assert, continuing..."

echo "About to call finish_test"
finish_test
