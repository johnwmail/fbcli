#!/usr/bin/env bash
# test-download-zip-noconflict.bash
# Test download -z zip file logic, especially zip/dir conflict avoidance
set -euo pipefail

FBCLI="$(cd "$(dirname "$0")/.." && pwd)/fbcli"
REMOTE_BASE=/test-zip-noconflict-$$
LOCAL_BASE=$(mktemp -d /tmp/fbcli-zip-noconflict.XXXXXX)

cleanup() {
  rm -rf "$LOCAL_BASE"
  $FBCLI rm "$REMOTE_BASE" || true
}
trap cleanup EXIT

# Setup remote test dir
mkdir -p "$LOCAL_BASE/dir1"
echo "foo" > "$LOCAL_BASE/dir1/file.txt"
echo "bar" > "$LOCAL_BASE/file.txt"
$FBCLI upload "$LOCAL_BASE" "$REMOTE_BASE"

# 1. Download zip to current dir (no local path)
cd "$LOCAL_BASE"
$FBCLI download -z "$REMOTE_BASE"  # should create test-zip-noconflict-$$.zip in $LOCAL_BASE
if [ ! -f "$(basename $REMOTE_BASE).zip" ]; then
  echo "[FAIL] Zip not created in current dir"
  exit 1
fi
rm -f "$(basename $REMOTE_BASE).zip"

# 2. Download zip to a directory, where <dir>/<remote_base>.zip would conflict with a directory
mkdir -p "conflict"
mkdir -p "conflict/$(basename $REMOTE_BASE).zip" # create a directory with the same name as the intended zip
$FBCLI download -z "$REMOTE_BASE" "conflict"
# Should create conflict/$(basename $REMOTE_BASE)-1.zip
if [ ! -f "conflict/$(basename $REMOTE_BASE)-1.zip" ]; then
  echo "[FAIL] Zip conflict avoidance failed"
  exit 1
fi
rm -rf "conflict/$(basename $REMOTE_BASE)-1.zip" "conflict/$(basename $REMOTE_BASE).zip"

# 3. Download zip to explicit .zip path
$FBCLI download -z "$REMOTE_BASE" "explicit.zip"
if [ ! -f "explicit.zip" ]; then
  echo "[FAIL] Explicit zip path failed"
  exit 1
fi
rm -f "explicit.zip"

# 4. Download zip to a file path without .zip
$FBCLI download -z "$REMOTE_BASE" "foo"
if [ ! -f "foo.zip" ]; then
  echo "[FAIL] foo.zip not created"
  exit 1
fi
rm -f "foo.zip"

echo "[PASS] All zip/dir conflict and naming tests passed."
