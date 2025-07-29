#!/usr/bin/env bash
# Modern Test Framework for fbcli
# Clean, unified test framework with consistent output and guaranteed cleanup

# Color codes for output (only set if not already defined)
if [ -z "${RED:-}" ]; then
    readonly RED='\033[0;31m'
    readonly GREEN='\033[0;32m'
    readonly YELLOW='\033[1;33m'
    readonly BLUE='\033[0;34m'
    readonly PURPLE='\033[0;35m'
    readonly CYAN='\033[0;36m'
    readonly NC='\033[0m' # No Color
fi

# Test statistics
TEST_COUNT=0
PASS_COUNT=0
FAIL_COUNT=0
START_TIME=""
TEST_NAME=""
CREATED_REMOTE_PATHS=()
CREATED_LOCAL_PATHS=()

# Initialize test
init_test() {
    local test_name="$1"
    TEST_NAME="$test_name"
    TEST_COUNT=0
    PASS_COUNT=0
    FAIL_COUNT=0
    CREATED_REMOTE_PATHS=()
    CREATED_LOCAL_PATHS=()
    START_TIME=$(date +%s)
    
    echo -e "${BLUE}▶ Testing: ${test_name}${NC}"
    
    # Verify fbcli exists
    if [ ! -x "./fbcli" ]; then
        echo -e "${RED}❌ ERROR: fbcli binary not found${NC}" >&2
        exit 1
    fi
    
    # Set up cleanup trap - but don't exit on cleanup failure
    trap 'cleanup_all_resources 2>/dev/null; true' EXIT
}

# Test step indicator
step() {
    echo -e "  ${CYAN}→${NC} $*"
}

# Test assertion with command execution
assert() {
    local description="$1"
    shift
    TEST_COUNT=$((TEST_COUNT + 1))
    
    local output
    local exit_code=0
    
    # Temporarily disable exit on error for this command
    set +e
    output=$("$@" 2>&1)
    exit_code=$?
    set -e
    
    if [ $exit_code -eq 0 ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}Command:${NC} $*" >&2
        echo -e "    ${RED}Output:${NC} $output" >&2
        return 1
    fi
}

# Test assertion that expects failure
assert_fails() {
    local description="$1"
    shift
    TEST_COUNT=$((TEST_COUNT + 1))
    
    local exit_code=0
    
    # Temporarily disable exit on error
    set +e
    "$@" &>/dev/null
    exit_code=$?
    set -e
    
    if [ $exit_code -ne 0 ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description (expected failure but succeeded)"
        return 1
    fi
}

# Test file/directory exists
assert_exists() {
    local description="$1"
    local path="$2"
    TEST_COUNT=$((TEST_COUNT + 1))
    
    if [ -e "$path" ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}Path not found:${NC} $path" >&2
        return 1
    fi
}

# Test file/directory does not exist
assert_not_exists() {
    local description="$1"
    local path="$2"
    TEST_COUNT=$((TEST_COUNT + 1))
    
    if [ ! -e "$path" ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}Path should not exist:${NC} $path" >&2
        return 1
    fi
}

# Test remote file/directory exists
assert_remote_exists() {
    local description="$1"
    local remote_path="$2"
    TEST_COUNT=$((TEST_COUNT + 1))
    
    local exit_code=0
    
    # Temporarily disable exit on error
    set +e
    ./fbcli ls "$(dirname "$remote_path")" 2>/dev/null | grep -q "$(basename "$remote_path")"
    exit_code=$?
    set -e
    
    if [ $exit_code -eq 0 ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}Remote path not found:${NC} $remote_path" >&2
        return 1
    fi
}

# Test remote file/directory does not exist
assert_remote_not_exists() {
    local description="$1"
    local remote_path="$2"
    TEST_COUNT=$((TEST_COUNT + 1))
    
    local exit_code=0
    
    # Temporarily disable exit on error
    set +e
    ./fbcli ls "$(dirname "$remote_path")" 2>/dev/null | grep -q "$(basename "$remote_path")"
    exit_code=$?
    set -e
    
    if [ $exit_code -ne 0 ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}Remote path should not exist:${NC} $remote_path" >&2
        return 1
    fi
}

# Test output contains expected string
assert_contains() {
    local description="$1"
    local expected="$2"
    shift 2
    TEST_COUNT=$((TEST_COUNT + 1))
    
    local output
    local exit_code=0
    
    # Temporarily disable exit on error
    set +e
    output=$("$@" 2>&1)
    exit_code=$?
    set -e
    
    if [ $exit_code -eq 0 ] && echo "$output" | grep -q "$expected"; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        if [ $exit_code -ne 0 ]; then
            echo -e "    ${RED}Command failed with exit code:${NC} $exit_code" >&2
        else
            echo -e "    ${RED}Expected to contain:${NC} $expected" >&2
        fi
        echo -e "    ${RED}Actual output:${NC} $output" >&2
        return 1
    fi
}

# Test output does not contain expected string
assert_not_contains() {
    local description="$1"
    local not_expected="$2"
    shift 2
    TEST_COUNT=$((TEST_COUNT + 1))
    
    local output
    local exit_code=0
    
    # Temporarily disable exit on error
    set +e
    output=$("$@" 2>&1)
    exit_code=$?
    set -e
    
    if [ $exit_code -eq 0 ] && ! echo "$output" | grep -q "$not_expected"; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        if [ $exit_code -ne 0 ]; then
            echo -e "    ${RED}Command failed with exit code:${NC} $exit_code" >&2
        else
            echo -e "    ${RED}Should not contain:${NC} $not_expected" >&2
        fi
        echo -e "    ${RED}Actual output:${NC} $output" >&2
        return 1
    fi
}

# Generate unique test identifier
gen_id() {
    echo "test-$(date +%s)-$$-$RANDOM"
}

# Track created local path for cleanup
track_local() {
    CREATED_LOCAL_PATHS+=("$1")
}

# Track created remote path for cleanup
track_remote() {
    CREATED_REMOTE_PATHS+=("$1")
}

# Create test file with content
create_test_file() {
    local file_path="$1"
    local content="${2:-test content $(date)}"
    
    mkdir -p "$(dirname "$file_path")"
    echo "$content" > "$file_path"
    track_local "$file_path"
}

# Create test directory with files
create_test_dir() {
    local dir_path="$1"
    local num_files="${2:-3}"
    
    mkdir -p "$dir_path"
    track_local "$dir_path"
    
    for i in $(seq 1 "$num_files"); do
        create_test_file "$dir_path/file$i.txt" "Test file $i content"
    done
}

# Cleanup all created resources
cleanup_all_resources() {
    local failed=false
    
    # Disable strict error handling for cleanup
    set +e
    
    # Clean up remote paths first
    for remote_path in "${CREATED_REMOTE_PATHS[@]}"; do
        if ./fbcli ls "$remote_path" &>/dev/null; then
            ./fbcli rm "$remote_path" &>/dev/null || failed=true
        fi
    done
    
    # Clean up local paths
    for local_path in "${CREATED_LOCAL_PATHS[@]}"; do
        if [ -e "$local_path" ]; then
            rm -rf "$local_path" 2>/dev/null || failed=true
        fi
    done
    
    # Clean up any remaining test artifacts
    rm -rf test-* local-test-* downloaded_* conflict/ explicit.zip foo.zip 2>/dev/null || true
    
    # Clean up any remaining remote test directories
    if [ -x "./fbcli" ]; then
        for dir in $(./fbcli ls / 2>/dev/null | grep -E '^test-' | awk '{print $1}' 2>/dev/null || true); do
            ./fbcli rm "/$dir" &>/dev/null 2>&1 || true
        done
    fi
    
    # Re-enable strict error handling
    set -e
    
    if [ "$failed" = true ]; then
        echo -e "  ${YELLOW}⚠ Some cleanup operations failed${NC}" >&2
    fi
}

# Finish test and show summary
finish_test() {
    local end_time=$(date +%s)
    local duration=$((end_time - START_TIME))
    
    echo ""
    if [ $FAIL_COUNT -eq 0 ]; then
        echo -e "${GREEN}✅ PASS${NC} $TEST_NAME (${PASS_COUNT}/${TEST_COUNT} tests, ${duration}s)"
        exit 0
    else
        echo -e "${RED}❌ FAIL${NC} $TEST_NAME (${PASS_COUNT}/${TEST_COUNT} tests, ${duration}s)"
        exit 1
    fi
}

# Execute command silently (no output unless it fails)
silent() {
    "$@" &>/dev/null
}

# Execute command and capture output
capture() {
    "$@" 2>&1
}
