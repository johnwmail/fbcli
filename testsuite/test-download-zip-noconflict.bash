echo "[PASS] All zip/dir conflict and naming tests passed."
#!/usr/bin/env bash
set -euo pipefail

FBCLI="$(cd "$(dirname "$0")/.." && pwd)/fbcli"
REMOTE_BASE="/test-zip-noconflict-$$"
LOCAL_BASE=$(mktemp -d /tmp/fbcli-zip-noconflict.XXXXXX)

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  rm -rf "$LOCAL_BASE"
  $FBCLI rm "$REMOTE_BASE" || true
}
trap cleanup EXIT

main() {
  mkdir -p "$LOCAL_BASE/dir1"
  echo "foo" > "$LOCAL_BASE/dir1/file.txt"
  echo "bar" > "$LOCAL_BASE/file.txt"
  $FBCLI upload "$LOCAL_BASE" "$REMOTE_BASE" || fail "upload failed"

  cd "$LOCAL_BASE"
  $FBCLI download -z "$REMOTE_BASE" || fail "download zip to current dir failed"
  [ -f "$(basename $REMOTE_BASE).zip" ] || fail "Zip not created in current dir"
  rm -f "$(basename $REMOTE_BASE).zip"

  mkdir -p "conflict"
  mkdir -p "conflict/$(basename $REMOTE_BASE).zip"
  $FBCLI download -z "$REMOTE_BASE" "conflict" || fail "download zip to conflict dir failed"
  [ -f "conflict/$(basename $REMOTE_BASE)-1.zip" ] || fail "Zip conflict avoidance failed"
  rm -rf "conflict/$(basename $REMOTE_BASE)-1.zip" "conflict/$(basename $REMOTE_BASE).zip"

  $FBCLI download -z "$REMOTE_BASE" "explicit.zip" || fail "download zip to explicit path failed"
  [ -f "explicit.zip" ] || fail "Explicit zip path failed"
  rm -f "explicit.zip"

  $FBCLI download -z "$REMOTE_BASE" "foo" || fail "download zip to foo failed"
  [ -f "foo.zip" ] || fail "foo.zip not created"
  rm -f "foo.zip"

  pass "All zip/dir conflict and naming tests passed."
}

main "$@"
