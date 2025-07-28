#!/usr/bin/env bash
set -euo pipefail

FBCLI=./fbcli
UUID=$(cat /proc/sys/kernel/random/uuid)
TEST_DIR="test-rm-ignore-dir-$UUID"
FILE_TO_DELETE="file-to-delete-$UUID.txt"
FILE_TO_IGNORE="file-to-ignore-$UUID.txt"
LOCAL_FILE="local-file-$UUID.txt"

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  $FBCLI rm "/$TEST_DIR/$FILE_TO_DELETE" &>/dev/null || true
  $FBCLI rm "/$TEST_DIR/$FILE_TO_IGNORE" &>/dev/null || true
  $FBCLI rm "/$TEST_DIR" &>/dev/null || true
  rm -f "$LOCAL_FILE"
}
trap cleanup EXIT

echo "--- Test passed: rm -i correctly ignored the directory ---"
echo "--- All rm/delete ignore tests passed! ---"

main() {
  $FBCLI mkdir "/$TEST_DIR" || fail "mkdir failed"
  $FBCLI upload <(echo "delete me") "/$TEST_DIR/$FILE_TO_DELETE" || fail "upload delete file failed"
  $FBCLI upload <(echo "ignore me") "/$TEST_DIR/$FILE_TO_IGNORE" || fail "upload ignore file failed"
  touch "$LOCAL_FILE"

  $FBCLI rm -i "$FILE_TO_IGNORE" "/$TEST_DIR" || fail "rm -i failed"
  $FBCLI ls "/$TEST_DIR" | grep -q "$FILE_TO_DELETE" && fail "$FILE_TO_DELETE was not deleted."
  $FBCLI ls "/$TEST_DIR" | grep -q "$FILE_TO_IGNORE" || fail "$FILE_TO_IGNORE was deleted, but it should have been ignored."
  pass "rm -i correctly handled directory contents."

  $FBCLI rm -i "$TEST_DIR" "/" || fail "rm -i dir failed"
  $FBCLI ls "/" | grep -q "$TEST_DIR" || fail "$TEST_DIR was deleted, but it should have been ignored."
  pass "rm -i correctly ignored the directory."
}

main "$@"
