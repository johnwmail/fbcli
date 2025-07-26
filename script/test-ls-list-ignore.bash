#!/usr/bin/env bash
# test-ls-list-ignore.bash
# Utility to test fbcli ls/list with and without -i ignore

set -euo pipefail

FBCLI="${FBCLI:-./fbcli}"
IGNORE_NAME="${1:-fbcli}"
REMOTE_PATH="${2:-/}"

function test_ls_ignore() {
  echo "==> fbcli ls $REMOTE_PATH (no ignore)"
  $FBCLI ls $REMOTE_PATH
  echo
  echo "==> fbcli ls $REMOTE_PATH -i $IGNORE_NAME (ignore)"
  $FBCLI ls $REMOTE_PATH -i $IGNORE_NAME
  echo
}

function test_list_ignore() {
  echo "==> fbcli list $REMOTE_PATH (no ignore)"
  $FBCLI list $REMOTE_PATH
  echo
  echo "==> fbcli list $REMOTE_PATH -i $IGNORE_NAME (ignore)"
  $FBCLI list $REMOTE_PATH -i $IGNORE_NAME
  echo
}


# Create a unique test directory and file using $RANDOM for uniqueness
RAND_ID="test.$RANDOM"
TEST_DIR="${RAND_ID}-dir"
TEST_FILE="${RAND_ID}-file.txt"

echo "==> Creating test directory: $TEST_DIR"
$FBCLI mkdir "/$TEST_DIR"

# Check mkdir success
if ! $FBCLI ls / | grep -q "$TEST_DIR"; then
  echo "[ERROR] Test directory was not created!" >&2
  exit 1
fi

echo "==> Creating test file: $TEST_FILE in /$TEST_DIR"
echo "hello" | $FBCLI upload /dev/stdin "/$TEST_DIR/$TEST_FILE"

# Check upload success
if ! $FBCLI ls "/$TEST_DIR" | grep -q "$TEST_FILE"; then
  echo "[ERROR] Test file was not uploaded!" >&2
  exit 1
fi

# Check directory with and without -i
echo "\n==> Checking for test directory with and without -i"
$FBCLI ls / | grep "$TEST_DIR" && echo "[ls] Directory found (expected)" || { echo "[ERROR] Directory not found!" >&2; exit 1; }
$FBCLI ls / -i "$TEST_DIR" | grep "$TEST_DIR" && { echo "[ERROR] Directory should be ignored but was found!" >&2; exit 1; } || echo "[ls] Directory ignored (expected)"

# Check file with and without -i
echo "\n==> Checking for test file with and without -i"
$FBCLI ls "/$TEST_DIR" | grep "$TEST_FILE" && echo "[ls] File found (expected)" || { echo "[ERROR] File not found!" >&2; exit 1; }
$FBCLI ls "/$TEST_DIR" -i "$TEST_FILE" | grep "$TEST_FILE" && { echo "[ERROR] File should be ignored but was found!" >&2; exit 1; } || echo "[ls] File ignored (expected)"

# Clean up
echo "\n==> Cleaning up test directory and file"
$FBCLI rm "/$TEST_DIR/$TEST_FILE"
$FBCLI rm "/$TEST_DIR"

echo "PASSED: ls/list -i ignore feature works as expected"

# Also run the original ignore tests for reference
test_ls_ignore
test_list_ignore
