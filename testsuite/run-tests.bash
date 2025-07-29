#!/usr/bin/env bash
# Unified Test Runner for fbcli
# Runs all tests with consistent output and summary

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

print_header() {
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                            FBCLI Test Suite                                 ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

print_summary() {
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME_GLOBAL))
    
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                              Test Summary                                    ║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════════════════════╣${NC}"
    
    if [ $TOTAL_FAILED -eq 0 ]; then
        echo -e "${CYAN}║${NC} ${GREEN}✅ ALL TESTS PASSED${NC}                                                        ${CYAN}║${NC}"
    else
        echo -e "${CYAN}║${NC} ${RED}❌ SOME TESTS FAILED${NC}                                                       ${CYAN}║${NC}"
    fi
    
    echo -e "${CYAN}║${NC}                                                                              ${CYAN}║${NC}"
    printf "${CYAN}║${NC} Total Tests: %-3d  Passed: ${GREEN}%-3d${NC}  Failed: ${RED}%-3d${NC}  Duration: %-6ss ${CYAN}║${NC}\n" \
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
    local test_name=$(basename "$test_file" .bash)
    
    echo -e "${BLUE}▶ Running: ${test_name}${NC}"
    
    # Run the test and capture its exit code
    if [ "$DETAILED_OUTPUT" = true ]; then
        # Show detailed output when -d flag is used
        if bash "$test_file"; then
            ((TOTAL_PASSED++))
            echo -e "${GREEN}[PASSED]${NC} $test_name"
        else
            ((TOTAL_FAILED++))
            FAILED_TESTS+=("$test_name")
            echo -e "${RED}[FAILED]${NC} $test_name"
        fi
    else
        # Hide detailed output, just show pass/fail
        if bash "$test_file" >/dev/null 2>&1; then
            ((TOTAL_PASSED++))
            echo -e "${GREEN}[PASSED]${NC} $test_name"
        else
            ((TOTAL_FAILED++))
            FAILED_TESTS+=("$test_name")
            echo -e "${RED}[FAILED]${NC} $test_name"
        fi
    fi
    ((TOTAL_TESTS++))
    echo ""
}

main() {
    START_TIME_GLOBAL=$(date +%s)
    
    print_header
    
    # Change to the fbcli directory (parent of testsuite)
    cd "$SCRIPT_DIR/.."
    
    # Verify fbcli binary exists
    if [ ! -x "./fbcli" ]; then
        echo -e "${RED}Error: fbcli binary not found. Please build it first with 'go build'${NC}" >&2
        exit 1
    fi
    
    # Show fbcli version
    echo -e "${YELLOW}Testing with:${NC}"
    ./fbcli show | head -1
    echo ""
    
    # Find all test files (exclude this runner and the framework)
    local test_files=($(find testsuite -name "*.bash" -not -name "run-tests.bash" -not -name "framework.bash" | sort))
    
    if [ ${#test_files[@]} -eq 0 ]; then
        echo -e "${YELLOW}No test files found in testsuite directory${NC}"
        exit 0
    fi
    
    echo -e "${YELLOW}Found ${#test_files[@]} test files${NC}"
    echo ""
    
    # Run each test file
    for test_file in "${test_files[@]}"; do
        run_test_file "$test_file"
    done
    
    print_summary
    
    # Exit with failure if any tests failed
    if [ $TOTAL_FAILED -gt 0 ]; then
        exit 1
    fi
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [OPTIONS] [test-file]"
        echo ""
        echo "Run all fbcli tests or a specific test file"
        echo ""
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo "  -d            Show detailed test output (default: only pass/fail)"
        echo ""
        echo "Examples:"
        echo "  $0                                    # Run all tests (summary only)"
        echo "  $0 -d                                 # Run all tests with detailed output"
        echo "  $0 testsuite/test-show-simple.bash   # Run specific test"
        echo "  $0 -d testsuite/test-show-simple.bash # Run specific test with details"
        exit 0
        ;;
    -d)
        # Enable detailed output and run all tests
        DETAILED_OUTPUT=true
        main
        ;;
    "")
        # Run all tests
        main
        ;;
    *)
        # Check if first argument is -d
        if [ "$1" = "-d" ] && [ -n "$2" ]; then
            # Run specific test file with detailed output
            DETAILED_OUTPUT=true
            if [ -f "$2" ]; then
                START_TIME_GLOBAL=$(date +%s)
                print_header
                cd "$SCRIPT_DIR/.."
                run_test_file "$2"
                print_summary
                exit $TOTAL_FAILED
            else
                echo -e "${RED}Error: Test file '$2' not found${NC}" >&2
                exit 1
            fi
        elif [ -f "$1" ]; then
            # Run specific test file
            START_TIME_GLOBAL=$(date +%s)
            print_header
            cd "$SCRIPT_DIR/.."
            run_test_file "$1"
            print_summary
            exit $TOTAL_FAILED
        else
            echo -e "${RED}Error: Test file '$1' not found${NC}" >&2
            exit 1
        fi
        ;;
esac
