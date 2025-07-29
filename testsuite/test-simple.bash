#!/usr/bin/env bash
# Simple test to verify fbcli basic functionality

cd "$(dirname "$0")/.."

echo "=== Testing fbcli basic functionality ==="

# Test 1: show command
echo "Test 1: show command"
if ./fbcli show >/dev/null 2>&1; then
    echo "  ✓ show command works"
else
    echo "  ✗ show command failed"
    exit 1
fi

# Test 2: ls command
echo "Test 2: ls command"
if ./fbcli ls >/dev/null 2>&1; then
    echo "  ✓ ls command works"
else
    echo "  ✗ ls command failed"
    exit 1
fi

# Test 3: mkdir command
echo "Test 3: mkdir command"
TEST_DIR="/test-simple-$RANDOM"
if ./fbcli mkdir "$TEST_DIR" >/dev/null 2>&1; then
    echo "  ✓ mkdir command works"
    # Clean up
    ./fbcli rm "$TEST_DIR" >/dev/null 2>&1
else
    echo "  ✗ mkdir command failed"
    exit 1
fi

# Test 4: file upload/download
echo "Test 4: file upload/download"
echo "test content" > test-file.txt
if ./fbcli upload test-file.txt /test-file.txt >/dev/null 2>&1; then
    echo "  ✓ upload command works"
    if ./fbcli download /test-file.txt test-downloaded.txt >/dev/null 2>&1; then
        echo "  ✓ download command works"
        # Clean up
        rm -f test-file.txt test-downloaded.txt
        ./fbcli rm /test-file.txt >/dev/null 2>&1
    else
        echo "  ✗ download command failed"
        rm -f test-file.txt
        ./fbcli rm /test-file.txt >/dev/null 2>&1
        exit 1
    fi
else
    echo "  ✗ upload command failed"
    rm -f test-file.txt
    exit 1
fi

echo "=== All basic tests passed! ==="
