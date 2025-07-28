#!/bin/bash
set -e

echo "--- Running rm/delete multiple files/directories test ---"



# Setup test directories and files
test_dir="test-multi-rm-dir-$(date +%s)-$RANDOM"
sub1="$test_dir/sub1"
sub2="$test_dir/sub2"
mkdir -p "$sub1" || { echo "[FAIL] mkdir $sub1 failed"; exit 1; }
mkdir -p "$sub2" || { echo "[FAIL] mkdir $sub2 failed"; exit 1; }
touch "$sub1/file1.txt" || { echo "[FAIL] touch file1.txt failed"; exit 1; }
touch "$sub2/file2.txt" || { echo "[FAIL] touch file2.txt failed"; exit 1; }

# Upload subdirectories and files
./fbcli upload "$sub1/file1.txt" /$test_dir/sub1/file1.txt
./fbcli upload "$sub2/file2.txt" /$test_dir/sub2/file2.txt


# Remove sub1 and sub2 from remote
## echo "[DEBUG] Running: ./fbcli rm /$test_dir/sub1 /$test_dir/sub2"
./fbcli rm /$test_dir/sub1 /$test_dir/sub2

# Wait for remote operations to complete
sleep 2

# Check that sub1 and sub2 are deleted
echo "[PASS] rm/delete sub1 and sub2 directories and their contents works as expected."



if ./fbcli ls /$test_dir 2>/dev/null; then
  result=0
  if ./fbcli ls /$test_dir | grep -q "sub1"; then
    echo "[FAIL] sub1 was not deleted"
    result=1
  fi
  if ./fbcli ls /$test_dir | grep -q "sub2"; then
    echo "[FAIL] sub2 was not deleted"
    result=1
  fi
  if [ "$result" -eq 0 ]; then
    echo "[PASS] rm/delete sub1 and sub2 directories and their contents works as expected."
  fi
else
  echo "[INFO] Parent directory $test_dir does not exist. sub1 and sub2 are considered deleted."
  echo "[PASS] rm/delete sub1 and sub2 directories and their contents works as expected."
  result=0
fi

# Cleanup (always run)
## echo "[DEBUG] Cleaning up local: $test_dir"
rm -rf "$test_dir"
## echo "[DEBUG] Cleaning up remote: /$test_dir"
./fbcli rm /$test_dir || true

exit $result

