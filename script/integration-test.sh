#!/bin/bash
set -e

# Create local files for testing uploads
echo "--- Creating test assets ---"
echo "hello world" > testfile.txt
mkdir -p 'local test dir'
echo "nested file" > 'local test dir/nested.txt'

# Test the show command
echo "--- Testing show command ---"
./fbcli show

# Test creating a directory on the remote
echo "--- Testing mkdir ---"
./fbcli mkdir '/test dir'

# Test uploading a single file
echo "--- Testing single file upload ---"
./fbcli upload testfile.txt '/test dir'

# Verify the single file upload by listing the remote directory
echo "--- Verifying single file upload with ls ---"
./fbcli ls '/test dir' | grep 'testfile.txt'

# Test uploading a local directory
echo "--- Testing directory upload ---"
./fbcli upload 'local test dir' '/'

# Verify the directory upload by listing its contents on the remote
echo "--- Verifying directory upload with ls ---"
./fbcli ls '/local test dir' | grep 'nested.txt'

# Test downloading a directory
echo "--- Testing directory download ---"
./fbcli download '/local test dir' 'downloaded_dir'

# Verify the downloaded directory's contents
echo "--- Verifying directory download ---"
ls | grep 'downloaded_dir.zip'
unzip -l downloaded_dir.zip | grep 'nested.txt'

# Test renaming a remote directory
echo "--- Testing rename ---"
./fbcli mv '/test dir' '/renamed dir'

# Verify the rename by listing the parent directory
echo "--- Verifying rename with ls ---"
./fbcli ls '/' | grep 'renamed dir'

# Test removing the remote files and directories created during the test
echo "--- Testing rm (cleanup) ---"
./fbcli rm '/renamed dir/testfile.txt'
./fbcli rm '/renamed dir'
./fbcli rm '/local test dir/nested.txt'
./fbcli rm '/local test dir'

# Verify that the remote files and directories were deleted
echo "--- Verifying deletion with ls ---"
if ./fbcli ls '/' | grep -E 'renamed dir|local test dir'; then
  echo "Error: A directory still exists after deletion."
  exit 1
fi

echo "All tests passed!"