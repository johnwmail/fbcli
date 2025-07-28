echo "[syncto] Ignore works as expected"
#!/usr/bin/env bash
set -euo pipefail

FBCLI="${FBCLI:-./fbcli}"
RAND_ID="test.$RANDOM"
TEST_DIR_LOCAL="${RAND_ID}-localdir"
TEST_FILE_LOCAL="${RAND_ID}-localfile.txt"
TEST_DIR_REMOTE="${RAND_ID}-remotedir"
TEST_FILE_REMOTE="${RAND_ID}-remotefile.txt"
IGNORE_NAME="${RAND_ID}-ignoreme"

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  $FBCLI rm "/$TEST_DIR_REMOTE/$TEST_FILE_REMOTE" 2>/dev/null || true
  $FBCLI rm "/$TEST_DIR_REMOTE/$IGNORE_NAME" 2>/dev/null || true
  $FBCLI rm "/$TEST_DIR_REMOTE/$TEST_FILE_LOCAL" 2>/dev/null || true
  $FBCLI rm "/$TEST_DIR_REMOTE" 2>/dev/null || true
  rm -rf "$TEST_DIR_LOCAL" "$TEST_FILE_LOCAL" "$IGNORE_NAME"
}
trap cleanup EXIT

main() {
  mkdir -p "$TEST_DIR_LOCAL"
  echo "local content" > "$TEST_DIR_LOCAL/$TEST_FILE_LOCAL"
  echo "ignore me" > "$TEST_DIR_LOCAL/$IGNORE_NAME"

  $FBCLI mkdir "/$TEST_DIR_REMOTE" || fail "mkdir failed"
  $FBCLI syncto "$TEST_DIR_LOCAL" "/$TEST_DIR_REMOTE" -i "$IGNORE_NAME" || fail "syncto failed"
  $FBCLI ls "/$TEST_DIR_REMOTE" | grep -q "$IGNORE_NAME" && fail "Ignored file was uploaded by syncto!"
  $FBCLI ls "/$TEST_DIR_REMOTE" | grep -q "$TEST_FILE_LOCAL" || fail "Test file was not uploaded by syncto!"
  pass "syncto ignore works as expected"

  $FBCLI upload "$TEST_DIR_LOCAL/$TEST_FILE_LOCAL" "/$TEST_DIR_REMOTE/$TEST_FILE_REMOTE" || fail "upload remote file failed"
  echo "ignore me remote" > "$IGNORE_NAME"
  $FBCLI upload "$IGNORE_NAME" "/$TEST_DIR_REMOTE" || fail "upload remote ignore file failed"
  rm "$IGNORE_NAME"
  rm -rf "$TEST_DIR_LOCAL"
  mkdir -p "$TEST_DIR_LOCAL"

  $FBCLI syncfrom "/$TEST_DIR_REMOTE" "$TEST_DIR_LOCAL" -i "$IGNORE_NAME" || fail "syncfrom failed"
  find "$TEST_DIR_LOCAL" -name "$IGNORE_NAME" | grep -q . && fail "Ignored file was downloaded by syncfrom!"
  find "$TEST_DIR_LOCAL" -name "$TEST_FILE_LOCAL" | grep -q . || fail "Test file was not downloaded by syncfrom!"
  pass "syncfrom ignore works as expected"
}

main "$@"

# Download from remote, ignoring $IGNORE_NAME
echo ">>> syncfrom: download remote dir to local, ignoring $IGNORE_NAME"
$FBCLI syncfrom "/$TEST_DIR_REMOTE" "$TEST_DIR_LOCAL" -i "$IGNORE_NAME"




# Check that ignored file is not present locally
if [ -f "$TEST_DIR_LOCAL/$IGNORE_NAME" ]; then
  echo "[ERROR] Ignored file was downloaded by syncfrom!" >&2
  exit 1
fi
if [ ! -f "$TEST_DIR_LOCAL/$TEST_FILE_LOCAL" ]; then
  echo "[ERROR] Test file was not downloaded by syncfrom!" >&2
  exit 1
fi

echo "[syncfrom] Ignore works as expected"


echo "PASSED: syncto/syncfrom -i ignore feature works as expected"
