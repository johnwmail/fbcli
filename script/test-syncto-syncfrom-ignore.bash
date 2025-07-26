#!/usr/bin/env bash
# test-syncto-syncfrom-ignore.bash
# Test fbcli syncto/syncfrom with -i ignore


set -euo pipefail

# Cleanup function to remove all test files and directories (local and remote)
cleanup() {
  $FBCLI rm "/$TEST_DIR_REMOTE/$TEST_FILE_REMOTE" 2>/dev/null || true
  $FBCLI rm "/$TEST_DIR_REMOTE/$IGNORE_NAME" 2>/dev/null || true
  $FBCLI rm "/$TEST_DIR_REMOTE/$TEST_FILE_LOCAL" 2>/dev/null || true
  $FBCLI rm "/$TEST_DIR_REMOTE" 2>/dev/null || true
  rm -rf "$TEST_DIR_LOCAL" "$TEST_FILE_LOCAL" "$IGNORE_NAME"
}
trap cleanup EXIT

FBCLI="${FBCLI:-./fbcli}"
RAND_ID="test.$RANDOM"
TEST_DIR_LOCAL="${RAND_ID}-localdir"
TEST_FILE_LOCAL="${RAND_ID}-localfile.txt"
TEST_DIR_REMOTE="${RAND_ID}-remotedir"
TEST_FILE_REMOTE="${RAND_ID}-remotefile.txt"
IGNORE_NAME="${RAND_ID}-ignoreme"

# Setup local test dir and file
mkdir -p "$TEST_DIR_LOCAL"
echo "local content" > "$TEST_DIR_LOCAL/$TEST_FILE_LOCAL"
echo "ignore me" > "$TEST_DIR_LOCAL/$IGNORE_NAME"



# Test syncto with -i (should not upload ignored file)
echo ">>> syncto: upload local dir to remote, ignoring $IGNORE_NAME"
$FBCLI mkdir "/$TEST_DIR_REMOTE"
$FBCLI syncto "$TEST_DIR_LOCAL" "/$TEST_DIR_REMOTE" -i "$IGNORE_NAME"

# Check that ignored file is not present remotely
if $FBCLI ls "/$TEST_DIR_REMOTE" | grep -q "$IGNORE_NAME"; then
  echo "[ERROR] Ignored file was uploaded by syncto!" >&2
  exit 1
fi
if ! $FBCLI ls "/$TEST_DIR_REMOTE" | grep -q "$TEST_FILE_LOCAL"; then
  echo "[ERROR] Test file was not uploaded by syncto!" >&2
  exit 1
fi

echo "[syncto] Ignore works as expected"

# Now test syncfrom with -i (should not download ignored file)
# First, upload a file and an ignored file to remote
$FBCLI upload "$TEST_DIR_LOCAL/$TEST_FILE_LOCAL" "/$TEST_DIR_REMOTE/$TEST_FILE_REMOTE"
echo "ignore me remote" | $FBCLI upload /dev/stdin "/$TEST_DIR_REMOTE/$IGNORE_NAME"

# Clean up local dir and re-create
rm -rf "$TEST_DIR_LOCAL"
mkdir -p "$TEST_DIR_LOCAL"

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
