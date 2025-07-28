#!/usr/bin/env bash
set -euo pipefail

FBCLI=./fbcli
TESTDIR="test-dir-$$"
NESTEDDIR="local-test-dir-$$"
TMPDL="downloaded_dir"
TESTFILE="testfile.txt"
NESTEDFILE="nested.txt"

fail() { echo "[FAIL] $*" >&2; exit 1; }
pass() { echo "[PASS] $*"; }

cleanup() {
  rm -rf "$TESTFILE" "$NESTEDDIR" "$TMPDL"
  # Remove all test/renamed dirs on remote
  for d in $($FBCLI ls / | grep -E '^(test|renamed)[^ ]*' | awk '{print $1}'); do
    $FBCLI rm "/$d" &>/dev/null || true
  done
  # Remove all test/renamed dirs locally
  rm -rf test-* local-test-dir-* renamed-* downloaded_dir*
}
trap cleanup EXIT

setup() {
  echo "hello world" > "$TESTFILE"
  mkdir -p "$NESTEDDIR"
  echo "nested file" > "$NESTEDDIR/$NESTEDFILE"
}

main() {
  setup

  $FBCLI show || fail "show failed"
  $FBCLI mkdir "/$TESTDIR" || fail "mkdir failed"
  $FBCLI upload "$TESTFILE" "/$TESTDIR" || fail "upload file failed"
  $FBCLI ls "/$TESTDIR" | grep "$TESTFILE" || fail "ls verify failed"
  $FBCLI upload "$NESTEDDIR" "/" || fail "upload dir failed"
  $FBCLI ls "/$NESTEDDIR" | grep "$NESTEDFILE" || fail "ls nested failed"
  $FBCLI download "/$NESTEDDIR" "$TMPDL" || fail "download dir failed"
  [ -f "$TMPDL/$NESTEDFILE" ] || fail "download verify failed"
  $FBCLI mv "/$TESTDIR" "/renamed-dir" || fail "rename failed"
  $FBCLI ls "/" | grep "renamed-dir" || fail "ls renamed failed"

  # Remove files/directories with wait/retry to ensure deletion
  for path in "/renamed-dir/$TESTFILE" "/renamed-dir" "/$NESTEDDIR/$NESTEDFILE" "/$NESTEDDIR"; do
    for i in {1..5}; do
      $FBCLI rm "$path" && break
      sleep 1
    done
  done

  # Wait for deletions to propagate
  for i in {1..5}; do
    ! $FBCLI ls "/" | grep -qE "renamed-dir|$NESTEDDIR" && break
    sleep 1
  done
  ! $FBCLI ls "/" | grep -qE "renamed-dir|$NESTEDDIR" || fail "cleanup verify failed"

  pass "integration-test.bash"
}



main "$@"


# Run all sub test scripts in ./testsuite/*.bash except this one, in sorted order
for subtest in $(ls testsuite/*.bash | grep -v integration-test.bash | sort); do
  echo "--- Running $(basename "$subtest") ---"
  bash "$subtest" || fail "$(basename "$subtest") failed"
done

echo "All tests passed!"
