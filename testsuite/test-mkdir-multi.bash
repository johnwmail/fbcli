#!/usr/bin/env bash
set -euo pipefail

FBCLI=./fbcli
FOLDER1="test-mkdir-multi-folder1-$$"
FOLDER2="test-mkdir-multi-folder2-$$"

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  $FBCLI rm "/$FOLDER1" &>/dev/null || true
  $FBCLI rm "/$FOLDER2" &>/dev/null || true
}
trap cleanup EXIT

echo "--- Running mkdir multiple folders test ---"
$FBCLI mkdir "/$FOLDER1" "/$FOLDER2" || fail "mkdir multiple folders failed"
$FBCLI ls / | grep -q "$FOLDER1" || fail "$FOLDER1 was not created!"
$FBCLI ls / | grep -q "$FOLDER2" || fail "$FOLDER2 was not created!"
pass "mkdir multiple folders works as expected."
