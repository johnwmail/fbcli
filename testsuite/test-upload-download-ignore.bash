echo "[PASS] Upload/Download with -i ignore feature works as expected."
#!/usr/bin/env bash
set -euo pipefail

FBCLI=./fbcli
REMOTE_BASE="/test-upload-ignore-$$"
LOCAL_BASE="fbcli-upload-ignore-local-$$"
LOCAL_DOWNLOAD="fbcli-download-ignore-local-$$"
IGNORE_REGEX='^ignoreme($|/)' # ignore any file/dir named 'ignoreme'

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  rm -rf "$LOCAL_BASE" "$LOCAL_DOWNLOAD"
  $FBCLI rm "$REMOTE_BASE" || true
}
trap cleanup EXIT

setup() {
  mkdir -p "$LOCAL_BASE/dir1/ignoreme" "$LOCAL_BASE/dir1/keepme"
  echo "should be ignored" > "$LOCAL_BASE/ignoreme"
  echo "should be kept" > "$LOCAL_BASE/keepme"
  echo "should be ignored" > "$LOCAL_BASE/dir1/ignoreme/file.txt"
  echo "should be kept" > "$LOCAL_BASE/dir1/keepme/file.txt"
}

main() {
  setup

  $FBCLI upload -i "$IGNORE_REGEX" "$LOCAL_BASE" "$REMOTE_BASE" || fail "upload with ignore failed"
  UPLOADED_DIR="$REMOTE_BASE/$(basename "$LOCAL_BASE")"
  echo "[DEBUG] Remote directory listing after upload:"
  $FBCLI list "$UPLOADED_DIR"
  REMOTE_LIST=$($FBCLI list "$UPLOADED_DIR")
  echo "$REMOTE_LIST" | grep -q ignoreme && fail "'ignoreme' was uploaded but should have been ignored!"
  echo "$REMOTE_LIST" | grep -q keepme || fail "'keepme' missing after upload!"

  $FBCLI download -i "$IGNORE_REGEX" "$REMOTE_BASE" "$LOCAL_DOWNLOAD" || fail "download with ignore failed"
  find "$LOCAL_DOWNLOAD" -name ignoreme | grep -q . && fail "'ignoreme' was downloaded but should have been ignored!"
  find "$LOCAL_DOWNLOAD" -name keepme | grep -q . || fail "'keepme' missing after download!"

  pass "Upload/Download with -i ignore feature works as expected."
}

main "$@"
# Download with ignore
