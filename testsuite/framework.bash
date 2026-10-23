#!/usr/bin/env bash
# Test framework for fbcli (FileBrowser Quantum v2)
# Provides assertions, environment checks, and guaranteed cleanup.

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

# Repository root (the parent of testsuite).
FBCLI_TEST_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FBCLI_PREFLIGHT_STATUS=""

# Test statistics
TEST_COUNT=0
PASS_COUNT=0
FAIL_COUNT=0
START_TIME=""
TEST_NAME=""
CREATED_REMOTE_PATHS=()
CREATED_LOCAL_PATHS=()

# Verify the binary and a usable Quantum v2 session before running a test.
check_environment() {
    if [ "$FBCLI_PREFLIGHT_STATUS" = "ok" ]; then
        return 0
    fi

    local missing=0
    for var in FILEBROWSER_URL FILEBROWSER_USERNAME FILEBROWSER_PASSWORD; do
        if [ -z "${!var:-}" ]; then
            echo -e "${RED}ERROR: ${var} is not set${NC}" >&2
            missing=1
        fi
    done
    if [ "$missing" -ne 0 ]; then
        echo -e "${YELLOW}Set the FILEBROWSER_* variables before running the integration tests.${NC}" >&2
        echo -e "${YELLOW}FILEBROWSER_SOURCE is optional; it is auto-detected when omitted.${NC}" >&2
        exit 1
    fi

    if [ ! -x "./fbcli" ]; then
        echo -e "${RED}ERROR: ./fbcli not found. Build it with: go build -o fbcli .${NC}" >&2
        exit 1
    fi

    local output
    local exit_code=0
    set +e
    output=$(./fbcli ls -s / 2>&1)
    exit_code=$?
    set -e

    if [ "$exit_code" -ne 0 ]; then
        echo -e "${RED}ERROR: cannot use the FileBrowser Quantum server at ${FILEBROWSER_URL}${NC}" >&2
        echo -e "${RED}${output}${NC}" >&2
        echo -e "${YELLOW}Check the URL, credentials, source access, and that the server is v2.0.1-stable or newer.${NC}" >&2
        exit 1
    fi

    FBCLI_PREFLIGHT_STATUS="ok"
}

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

    cd "$FBCLI_TEST_ROOT" || exit 1
    check_environment

    echo -e "${BLUE}▶ Testing: ${test_name}${NC}"

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

    if [ "$exit_code" -eq 0 ]; then
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

    local output
    local exit_code=0

    # Temporarily disable exit on error
    set +e
    output=$("$@" 2>&1)
    exit_code=$?
    set -e

    if [ "$exit_code" -ne 0 ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description (expected failure but succeeded)"
        echo -e "    ${RED}Output:${NC} $output" >&2
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

# Return the names in the remote parent directory of a path.
remote_sibling_names() {
    local remote_path="$1"
    local parent
    parent="$(dirname "$remote_path")"
    if [ "$parent" = "." ]; then
        parent="/"
    fi
    ./fbcli ls -s "$parent" 2>/dev/null | sed 's:/$::'
}

# Check a remote path exactly (names are compared literally).
remote_path_exists() {
    local remote_path="$1"
    local name
    name="$(basename "$remote_path")"
    remote_sibling_names "$remote_path" | grep -Fxq -- "$name"
}

# Test remote file/directory exists
assert_remote_exists() {
    local description="$1"
    local remote_path="$2"
    TEST_COUNT=$((TEST_COUNT + 1))

    if remote_path_exists "$remote_path"; then
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

    if ! remote_path_exists "$remote_path"; then
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

# Test output contains expected string (literal match)
assert_contains() {
    local description="$1"
    local expected="$2"
    shift 2
    TEST_COUNT=$((TEST_COUNT + 1))

    local output
    local exit_code=0

    set +e
    output=$("$@" 2>&1)
    exit_code=$?
    set -e

    if [ "$exit_code" -eq 0 ] && printf '%s\n' "$output" | grep -Fq -- "$expected"; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        if [ "$exit_code" -ne 0 ]; then
            echo -e "    ${RED}Command failed with exit code:${NC} $exit_code" >&2
        else
            echo -e "    ${RED}Expected to contain:${NC} $expected" >&2
        fi
        echo -e "    ${RED}Actual output:${NC} $output" >&2
        return 1
    fi
}

# Test output does not contain expected string (literal match)
assert_not_contains() {
    local description="$1"
    local not_expected="$2"
    shift 2
    TEST_COUNT=$((TEST_COUNT + 1))

    local output
    local exit_code=0

    set +e
    output=$("$@" 2>&1)
    exit_code=$?
    set -e

    if [ "$exit_code" -eq 0 ] && ! printf '%s\n' "$output" | grep -Fq -- "$not_expected"; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        if [ "$exit_code" -ne 0 ]; then
            echo -e "    ${RED}Command failed with exit code:${NC} $exit_code" >&2
        else
            echo -e "    ${RED}Should not contain:${NC} $not_expected" >&2
        fi
        echo -e "    ${RED}Actual output:${NC} $output" >&2
        return 1
    fi
}

# Test a local file has exactly the expected content.
assert_content() {
    local description="$1"
    local file_path="$2"
    local expected="$3"
    TEST_COUNT=$((TEST_COUNT + 1))

    local actual=""
    if [ -f "$file_path" ]; then
        actual="$(cat "$file_path")"
    fi
    if [ -f "$file_path" ] && [ "$actual" = "$expected" ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}File:${NC} $file_path" >&2
        echo -e "    ${RED}Expected:${NC} $expected" >&2
        echo -e "    ${RED}Actual:${NC} $actual" >&2
        return 1
    fi
}

# Test a remote file has exactly the expected content by downloading it.
assert_remote_content() {
    local description="$1"
    local remote_path="$2"
    local expected="$3"
    TEST_COUNT=$((TEST_COUNT + 1))

    local tmp
    local output
    local exit_code=0
    tmp="$(mktemp "${TMPDIR:-/tmp}/fbcli-remote-content.XXXXXX")"

    set +e
    output=$(./fbcli download "$remote_path" "$tmp" 2>&1)
    exit_code=$?
    set -e

    local actual=""
    if [ -f "$tmp" ]; then
        actual="$(cat "$tmp")"
    fi
    rm -f "$tmp"

    if [ "$exit_code" -eq 0 ] && [ "$actual" = "$expected" ]; then
        PASS_COUNT=$((PASS_COUNT + 1))
        echo -e "  ${GREEN}✓${NC} $description"
        return 0
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
        echo -e "  ${RED}✗${NC} $description"
        echo -e "    ${RED}Remote path:${NC} $remote_path" >&2
        echo -e "    ${RED}Expected:${NC} $expected" >&2
        echo -e "    ${RED}Actual:${NC} $actual" >&2
        if [ "$exit_code" -ne 0 ]; then
            echo -e "    ${RED}Download output:${NC} $output" >&2
        fi
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
    printf '%s\n' "$content" > "$file_path"
    track_local "$file_path"
}

# Create test directory with files
create_test_dir() {
    local dir_path="$1"
    local num_files="${2:-3}"

    mkdir -p "$dir_path"
    track_local "$dir_path"

    local i
    for i in $(seq 1 "$num_files"); do
        create_test_file "$dir_path/file$i.txt" "Test file $i content"
    done
}

# Cleanup all created resources
cleanup_all_resources() {
    local failed=false

    # Disable strict error handling for cleanup
    set +e

    # Clean up tracked remote paths first
    local remote_path
    for remote_path in "${CREATED_REMOTE_PATHS[@]}"; do
        if remote_path_exists "$remote_path"; then
            ./fbcli rm "$remote_path" >/dev/null 2>&1 || failed=true
        fi
    done

    # Clean up tracked local paths
    local local_path
    for local_path in "${CREATED_LOCAL_PATHS[@]}"; do
        if [ -e "$local_path" ]; then
            rm -rf "$local_path" 2>/dev/null || failed=true
        fi
    done

    # Remove remote test directories left behind by an aborted test.
    if [ -x "./fbcli" ]; then
        while IFS= read -r remote_path; do
            [ -z "$remote_path" ] && continue
            remote_path="${remote_path%/}"
            case "$remote_path" in
                test-*) ./fbcli rm "/$remote_path" >/dev/null 2>&1 || true ;;
            esac
        done < <(./fbcli ls -s / 2>/dev/null || true)
    fi

    # Local fallback for aborted tests; these prefixes are reserved by the suite.
    rm -rf test-* .fbcli-download-* 2>/dev/null || true

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
    if [ "$FAIL_COUNT" -eq 0 ]; then
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
