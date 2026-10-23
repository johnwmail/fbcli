#!/usr/bin/env bash
# Unified Test Runner for fbcli (FileBrowser Quantum v2)
# Runs all test files with consistent output and a summary.

# Source the test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/framework.bash"

# Global test statistics
TOTAL_TESTS=0
TOTAL_PASSED=0
TOTAL_FAILED=0
FAILED_TESTS=()
START_TIME_GLOBAL=""
DETAILED_OUTPUT=false
LIST_ONLY=false

print_header() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                            FBCLI Test Suite                                 ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_summary() {
    local end_time
    local total_duration
    end_time=$(date +%s)
    total_duration=$((end_time - START_TIME_GLOBAL))

    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                              Test Summary                                    ║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════════════════════╣${NC}"

    if [ "$TOTAL_FAILED" -eq 0 ]; then
        echo -e "${CYAN}║${NC} ${GREEN}✅ ALL TESTS PASSED${NC}                                                        ${CYAN}║${NC}"
    else
        echo -e "${CYAN}║${NC} ${RED}❌ SOME TESTS FAILED${NC}                                                       ${CYAN}║${NC}"
    fi

    echo -e "${CYAN}║${NC}                                                                              ${CYAN}║${NC}"
    printf "${CYAN}║${NC} Test files: %-3d  Passed: ${GREEN}%-3d${NC}  Failed: ${RED}%-3d${NC}  Duration: %-6ss ${CYAN}║${NC}\n" \
           "$TOTAL_TESTS" "$TOTAL_PASSED" "$TOTAL_FAILED" "$total_duration"

    if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
        echo -e "${CYAN}║${NC}                                                                              ${CYAN}║${NC}"
        echo -e "${CYAN}║${NC} ${RED}Failed Tests:${NC}                                                            ${CYAN}║${NC}"
        for test in "${FAILED_TESTS[@]}"; do
            printf "${CYAN}║${NC}   ${RED}•${NC} %-67s ${CYAN}║${NC}\n" "$test"
        done
    fi

    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
}

run_test_file() {
    local test_file="$1"
    local test_name
    test_name=$(basename "$test_file" .bash)

    echo -e "${BLUE}▶ Running: ${test_name}${NC}"

    # Run the test and capture its exit code
    if [ "$DETAILED_OUTPUT" = true ]; then
        # Show detailed output when -d flag is used
        if bash "$test_file"; then
            TOTAL_PASSED=$((TOTAL_PASSED + 1))
            echo -e "${GREEN}[PASSED]${NC} $test_name"
        else
            TOTAL_FAILED=$((TOTAL_FAILED + 1))
            FAILED_TESTS+=("$test_name")
            echo -e "${RED}[FAILED]${NC} $test_name"
        fi
    else
        # Hide detailed output, just show pass/fail
        if bash "$test_file" >/dev/null 2>&1; then
            TOTAL_PASSED=$((TOTAL_PASSED + 1))
            echo -e "${GREEN}[PASSED]${NC} $test_name"
        else
            TOTAL_FAILED=$((TOTAL_FAILED + 1))
            FAILED_TESTS+=("$test_name")
            echo -e "${RED}[FAILED]${NC} $test_name"
        fi
    fi
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    echo ""
}

discover_tests() {
    find testsuite -maxdepth 1 -name "test-*.bash" -type f | sort
}

print_environment() {
    echo -e "${YELLOW}Testing with:${NC}"
    ./fbcli show
    echo ""
}

main() {
    START_TIME_GLOBAL=$(date +%s)

    print_header

    # Change to the fbcli directory (parent of testsuite)
    cd "$SCRIPT_DIR/.." || exit 1

    mapfile -t test_files < <(discover_tests)

    if [ ${#test_files[@]} -eq 0 ]; then
        echo -e "${YELLOW}No test files found in testsuite directory${NC}"
        exit 0
    fi

    if [ "$LIST_ONLY" = true ]; then
        echo -e "${YELLOW}Found ${#test_files[@]} test files:${NC}"
        for test_file in "${test_files[@]}"; do
            echo "  $(basename "$test_file" .bash)"
        done
        exit 0
    fi

    # Verify the binary and server before running any test.
    check_environment

    print_environment

    echo -e "${YELLOW}Found ${#test_files[@]} test files${NC}"
    echo ""

    # Run each test file
    for test_file in "${test_files[@]}"; do
        run_test_file "$test_file"
    done

    print_summary

    # Exit with failure if any tests failed
    if [ "$TOTAL_FAILED" -gt 0 ]; then
        exit 1
    fi
}

run_single_test() {
    local test_file="$1"
    START_TIME_GLOBAL=$(date +%s)
    print_header
    cd "$SCRIPT_DIR/.." || exit 1
    check_environment
    run_test_file "$test_file"
    print_summary
    exit "$TOTAL_FAILED"
}

print_usage() {
    echo "Usage: $0 [OPTIONS] [test-file]"
    echo ""
    echo "Run all fbcli integration tests or one specific test file."
    echo ""
    echo "Options:"
    echo "  -h, --help        Show this help message"
    echo "  -d, --detailed    Show detailed test output (default: only pass/fail)"
    echo "  -l, --list        List discovered test files without running them"
    echo ""
    echo "Environment:"
    echo "  FILEBROWSER_URL        FileBrowser Quantum server URL (required)"
    echo "  FILEBROWSER_USERNAME   Login username (required)"
    echo "  FILEBROWSER_PASSWORD   Login password (required)"
    echo "  FILEBROWSER_SOURCE     Source name (optional; auto-detected)"
    echo "  FBCLI_TEST_SOURCE      Optional source name for explicit --source tests"
    echo ""
    echo "Examples:"
    echo "  $0                                     # Run all tests (summary only)"
    echo "  $0 -d                                  # Run all tests with details"
    echo "  $0 -l                                  # List test files"
    echo "  $0 testsuite/test-ls.bash              # Run one test file"
    echo "  $0 -d testsuite/test-ls.bash           # Run one test file with details"
}

case "${1:-}" in
    --help|-h)
        print_usage
        exit 0
        ;;
    -d|--detailed)
        DETAILED_OUTPUT=true
        if [ -n "${2:-}" ]; then
            if [ -f "$2" ]; then
                run_single_test "$2"
            else
                echo -e "${RED}Error: test file '$2' not found${NC}" >&2
                exit 1
            fi
        else
            main
        fi
        ;;
    -l|--list)
        LIST_ONLY=true
        main
        ;;
    "")
        main
        ;;
    *)
        if [ -f "$1" ]; then
            run_single_test "$1"
        else
            echo -e "${RED}Error: test file '$1' not found${NC}" >&2
            exit 1
        fi
        ;;
esac
