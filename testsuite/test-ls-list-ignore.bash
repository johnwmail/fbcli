#!/usr/bin/env bash
set -euo pipefail

FBCLI="${FBCLI:-./fbcli}"
RAND_ID="test.$RANDOM"
TEST_DIR="${RAND_ID}-dir"
TEST_FILE="${RAND_ID}-file.txt"
IGNORE_NAME="${1:-fbcli}"

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  rm -f "$TEST_FILE"
  $FBCLI rm "/$TEST_DIR" &>/dev/null || true
}
trap cleanup EXIT

# Create test directory and file
$FBCLI mkdir "/$TEST_DIR" || fail "mkdir failed"
$FBCLI ls / | grep -q "$TEST_DIR" || fail "Test directory was not created!"
echo "hello" > "$TEST_FILE"
$FBCLI upload "$TEST_FILE" "/$TEST_DIR" || fail "upload failed"
rm "$TEST_FILE"
$FBCLI ls "/$TEST_DIR" | grep -q "$TEST_FILE" || fail "Test file was not uploaded!"

# Test ls with and without -i ignore
$FBCLI ls / | grep "$TEST_DIR" || fail "Directory not found!"
! $FBCLI ls / -i "$TEST_DIR" | grep -q "$TEST_DIR" || fail "Directory should be ignored but was found!"
$FBCLI ls "/$TEST_DIR" | grep "$TEST_FILE" || fail "File not found!"
! $FBCLI ls "/$TEST_DIR" -i "$TEST_FILE" | grep -q "$TEST_FILE" || fail "File should be ignored but was found!"

# Test list with and without -i ignore
$FBCLI list / | grep "$TEST_DIR" || fail "Directory not found in list!"
! $FBCLI list / -i "$TEST_DIR" | grep -q "$TEST_DIR" || fail "Directory should be ignored in list but was found!"
$FBCLI list "/$TEST_DIR" | grep "$TEST_FILE" || fail "File not found in list!"
! $FBCLI list "/$TEST_DIR" -i "$TEST_FILE" | grep -q "$TEST_FILE" || fail "File should be ignored in list but was found!"

pass "ls/list -i ignore feature works as expected."
echo "PASSED: ls/list -i ignore feature works as expected"

