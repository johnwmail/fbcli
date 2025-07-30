#!/usr/bin/env bash
# Test script for ls command variations
# Tests ls, ls -l, ls -s with and without -i ignore patterns

source "$(dirname "$0")/framework.bash"

init_test "ls command variations"

# Generate unique test identifiers
TEST_ID=$(gen_id)
REMOTE_DIR="/test-ls-$TEST_ID"

step "Setting up test environment"
# Create test files with different patterns for ignore testing
create_test_file "file1.txt" "test content 1"
create_test_file "file2.txt" "test content 2"  
create_test_file "ignore-me.log" "ignore this"
create_test_file "normal.md" "normal file"
track_local "file1.txt"
track_local "file2.txt"
track_local "ignore-me.log"
track_local "normal.md"

# Create remote directory and upload test files
assert "Create remote test directory" ./fbcli mkdir "$REMOTE_DIR"
track_remote "$REMOTE_DIR"

assert "Upload file1.txt" ./fbcli upload "file1.txt" "$REMOTE_DIR/"
assert "Upload file2.txt" ./fbcli upload "file2.txt" "$REMOTE_DIR/"
assert "Upload ignore-me.log" ./fbcli upload "ignore-me.log" "$REMOTE_DIR/"
assert "Upload normal.md" ./fbcli upload "normal.md" "$REMOTE_DIR/"

step "Testing basic ls command (multi-column format)"
assert "ls works" ./fbcli ls "$REMOTE_DIR"
assert_contains "ls shows file1.txt" "file1.txt" ./fbcli ls "$REMOTE_DIR"
assert_contains "ls shows file2.txt" "file2.txt" ./fbcli ls "$REMOTE_DIR"
assert_contains "ls shows all files" "ignore-me.log" ./fbcli ls "$REMOTE_DIR"

step "Testing ls -l (detailed list format)"
assert "ls -l works" ./fbcli ls -l "$REMOTE_DIR"
assert_contains "ls -l shows file1.txt" "file1.txt" ./fbcli ls -l "$REMOTE_DIR"
assert_contains "ls -l shows headers" "Name" ./fbcli ls -l "$REMOTE_DIR"
assert_contains "ls -l shows size column" "Size" ./fbcli ls -l "$REMOTE_DIR"

step "Testing ls -s (script format)"
assert "ls -s works" ./fbcli ls -s "$REMOTE_DIR"
assert_contains "ls -s shows file1.txt" "file1.txt" ./fbcli ls -s "$REMOTE_DIR"

# Verify script format is one file per line
output=$(./fbcli ls -s "$REMOTE_DIR")
line_count=$(echo "$output" | wc -l)
file_count=$(echo "$output" | grep -c -E "\.(txt|log|md)$")
assert "ls -s produces one file per line" test "$line_count" -eq "$file_count"

step "Testing ignore functionality with regex patterns"

# Test ls with ignore pattern (should hide ignore-*.log files)
assert "ls -i works" ./fbcli ls -i "ignore-.*" "$REMOTE_DIR"
assert_not_contains "ls -i filters ignore-me.log" "ignore-me.log" ./fbcli ls -i "ignore-.*" "$REMOTE_DIR"
assert_contains "ls -i keeps other files" "file1.txt" ./fbcli ls -i "ignore-.*" "$REMOTE_DIR"

# Test ls -l with ignore pattern  
assert "ls -l -i works" ./fbcli ls -l -i "ignore-.*" "$REMOTE_DIR"
assert_not_contains "ls -l -i filters ignore-me.log" "ignore-me.log" ./fbcli ls -l -i "ignore-.*" "$REMOTE_DIR"
assert_contains "ls -l -i keeps other files" "file1.txt" ./fbcli ls -l -i "ignore-.*" "$REMOTE_DIR"
assert_contains "ls -l -i shows headers" "Name" ./fbcli ls -l -i "ignore-.*" "$REMOTE_DIR"

# Test ls -s with ignore pattern
assert "ls -s -i works" ./fbcli ls -s -i "ignore-.*" "$REMOTE_DIR"
assert_not_contains "ls -s -i filters ignore-me.log" "ignore-me.log" ./fbcli ls -s -i "ignore-.*" "$REMOTE_DIR"  
assert_contains "ls -s -i keeps other files" "file1.txt" ./fbcli ls -s -i "ignore-.*" "$REMOTE_DIR"

# Test different file extension ignores
assert "ls -i with .log pattern" ./fbcli ls -i ".*\\.log$" "$REMOTE_DIR"
assert_not_contains "ls -i .log filters .log files" "ignore-me.log" ./fbcli ls -i ".*\\.log$" "$REMOTE_DIR"
assert_contains "ls -i .log keeps .txt files" "file1.txt" ./fbcli ls -i ".*\\.log$" "$REMOTE_DIR"

step "Testing command aliases"
assert "list command works" ./fbcli list "$REMOTE_DIR"
assert "dir command works" ./fbcli dir "$REMOTE_DIR"

# Test that list and dir use detailed format by default
assert_contains "list shows headers" "Name" ./fbcli list "$REMOTE_DIR"
assert_contains "dir shows headers" "Name" ./fbcli dir "$REMOTE_DIR"

# Test list/dir with ignore
assert "list -i works" ./fbcli list -i "ignore-.*" "$REMOTE_DIR"
assert "dir -i works" ./fbcli dir -i "ignore-.*" "$REMOTE_DIR"

step "Testing flag order variations"
assert "flags work: -i then -s" ./fbcli ls -i "ignore-.*" -s "$REMOTE_DIR"
assert "flags work: -s then -i" ./fbcli ls -s -i "ignore-.*" "$REMOTE_DIR"
assert "flags work: -i then -l" ./fbcli ls -i "ignore-.*" -l "$REMOTE_DIR"
assert "flags work: -l then -i" ./fbcli ls -l -i "ignore-.*" "$REMOTE_DIR"

step "Testing edge cases"

# Test empty directory
EMPTY_DIR="/test-ls-empty-$TEST_ID"
assert "Create empty directory" ./fbcli mkdir "$EMPTY_DIR"
track_remote "$EMPTY_DIR"

assert "ls works on empty directory" ./fbcli ls "$EMPTY_DIR"
assert "ls -l works on empty directory" ./fbcli ls -l "$EMPTY_DIR"
assert "ls -s works on empty directory" ./fbcli ls -s "$EMPTY_DIR"

# Test invalid regex in ignore pattern
assert_fails "Invalid regex should cause error" ./fbcli ls -i "[invalid" "$REMOTE_DIR"

step "Testing script automation capability"
# Verify that ls -s can be used effectively in shell scripts
script_test_output=""
while IFS= read -r file; do
    if [[ -n "$file" ]]; then
        script_test_output="success"
        break
    fi
done < <(./fbcli ls -s "$REMOTE_DIR")
assert "ls -s works properly in shell loops" test "$script_test_output" = "success"

finish_test