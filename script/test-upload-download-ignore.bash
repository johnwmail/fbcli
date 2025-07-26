#!/usr/bin/env bash
# test-upload-download-ignore.bash
# Test upload and download with -i ignore regex, with full cleanup
set -euo pipefail

# CONFIG
FBCLI=./fbcli
REMOTE_BASE=/test-upload-ignore-$$
LOCAL_BASE=$(mktemp -d /tmp/fbcli-upload-ignore.XXXXXX)
LOCAL_DOWNLOAD=$(mktemp -d /tmp/fbcli-download-ignore.XXXXXX)
IGNORE_REGEX='^ignoreme($|/)' # ignore any file/dir named 'ignoreme'

# Cleanup function to remove all test artifacts (local and remote)
cleanup() {
  rm -rf "$LOCAL_BASE" "$LOCAL_DOWNLOAD"
  $FBCLI rm "$REMOTE_BASE" || true
}
trap cleanup EXIT

# Setup test data
mkdir -p "$LOCAL_BASE/dir1/ignoreme" "$LOCAL_BASE/dir1/keepme"
echo "should be ignored" > "$LOCAL_BASE/ignoreme"
echo "should be kept" > "$LOCAL_BASE/keepme"
echo "should be ignored" > "$LOCAL_BASE/dir1/ignoreme/file.txt"
echo "should be kept" > "$LOCAL_BASE/dir1/keepme/file.txt"

# Upload with ignore
$FBCLI upload -i "$IGNORE_REGEX" "$LOCAL_BASE" "$REMOTE_BASE"

# List remote after upload
REMOTE_LIST=$($FBCLI list "$REMOTE_BASE")
echo "$REMOTE_LIST"
if echo "$REMOTE_LIST" | grep -q ignoreme; then
  echo "[FAIL] 'ignoreme' was uploaded but should have been ignored!"
  exit 1
else
  echo "[PASS] 'ignoreme' was correctly ignored on upload."
fi
if ! echo "$REMOTE_LIST" | grep -q keepme; then
  echo "[FAIL] 'keepme' missing after upload!"
  exit 1
fi

# Download with ignore
$FBCLI download -i "$IGNORE_REGEX" "$REMOTE_BASE" "$LOCAL_DOWNLOAD"

# Check local after download
if find "$LOCAL_DOWNLOAD" -name ignoreme | grep -q .; then
  echo "[FAIL] 'ignoreme' was downloaded but should have been ignored!"
  exit 1
else
  echo "[PASS] 'ignoreme' was correctly ignored on download."
fi
if ! find "$LOCAL_DOWNLOAD" -name keepme | grep -q .; then
  echo "[FAIL] 'keepme' missing after download!"
  exit 1
fi

echo "[PASS] Upload/Download with -i ignore feature works as expected."
