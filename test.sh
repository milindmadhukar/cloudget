#!/bin/bash

# test.sh - Comprehensive test runner for dropbox_downloader with colored output
# This script runs all unit tests with detailed reporting and colored output

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# Unicode symbols
CHECKMARK="✓"
CROSS="✗"
INFO="ℹ"
ROCKET="🚀"
PACKAGE="📦"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Function to print colored output
print_colored() {
    printf "${1}${2}${NC}\n"
}

# Function to print section header
print_header() {
    echo
    print_colored "$WHITE" "================================"
    print_colored "$WHITE" "$1"
    print_colored "$WHITE" "================================"
}

# Function to run tests for a package
run_package_tests() {
    local package=$1
    local description=$2
    
    print_colored "$CYAN" "${PACKAGE} Testing $description..."
    
    # Run the tests and capture output
    if go test "./$package" -v -count=1 2>&1; then
        print_colored "$GREEN" "${CHECKMARK} $description tests PASSED"
        return 0
    else
        print_colored "$RED" "${CROSS} $description tests FAILED"
        return 1
    fi
}

# Function to run tests with coverage
run_coverage() {
    local package=$1
    local description=$2
    
    print_colored "$PURPLE" "${INFO} Running coverage analysis for $description..."
    
    # Generate coverage report
    go test "./$package" -coverprofile=coverage.out -covermode=atomic >/dev/null 2>&1
    
    if [ -f coverage.out ]; then
        # Get coverage percentage
        coverage=$(go tool cover -func=coverage.out | grep total | grep -oE '[0-9]+\.[0-9]+%')
        print_colored "$BLUE" "Coverage for $description: $coverage"
        rm coverage.out
    else
        print_colored "$YELLOW" "Coverage data not available for $description"
    fi
}

# Main test execution
main() {
    print_header "${ROCKET} Dropbox Downloader Test Suite"
    
    print_colored "$WHITE" "Starting comprehensive test execution..."
    print_colored "$YELLOW" "Test run started at: $(date)"
    
    # Test packages in order of dependency
    packages=(
        "pkg/utils:Utils Package (hash, http, resume)"
        "pkg/services/dropbox:Dropbox Service"
        "pkg/services/gdrive:Google Drive Service"
        "pkg/services/wetransfer:WeTransfer Service"
        "pkg/downloader:Download Manager"
        "cmd/downloader:Main Application"
    )
    
    total_packages=${#packages[@]}
    current_package=0
    
    for package_info in "${packages[@]}"; do
        current_package=$((current_package + 1))
        package=$(echo "$package_info" | cut -d: -f1)
        description=$(echo "$package_info" | cut -d: -f2)
        
        print_header "[$current_package/$total_packages] $description"
        
        # Check if package exists
        if [ ! -d "$package" ]; then
            print_colored "$YELLOW" "${INFO} Package $package not found, skipping..."
            continue
        fi
        
        # Check if package has tests
        if ! ls "$package"/*_test.go >/dev/null 2>&1; then
            print_colored "$YELLOW" "${INFO} No test files found in $package, skipping..."
            continue
        fi
        
        # Run tests
        if run_package_tests "$package" "$description"; then
            PASSED_TESTS=$((PASSED_TESTS + 1))
            
            # Run coverage analysis
            run_coverage "$package" "$description"
        else
            FAILED_TESTS=$((FAILED_TESTS + 1))
        fi
        
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    done
    
    # Run all tests together for final verification
    print_header "${ROCKET} Final Integration Test"
    print_colored "$CYAN" "${PACKAGE} Running all tests together..."
    
    if go test ./... -v -count=1 >/dev/null 2>&1; then
        print_colored "$GREEN" "${CHECKMARK} All integration tests PASSED"
    else
        print_colored "$RED" "${CROSS} Integration tests FAILED"
        print_colored "$YELLOW" "${INFO} Running with detailed output..."
        go test ./... -v -count=1
    fi
    
    # Test Summary
    print_header "${INFO} Test Summary"
    
    print_colored "$WHITE" "Test run completed at: $(date)"
    print_colored "$WHITE" "Total packages tested: $TOTAL_TESTS"
    print_colored "$GREEN" "Packages passed: $PASSED_TESTS"
    
    if [ $FAILED_TESTS -gt 0 ]; then
        print_colored "$RED" "Packages failed: $FAILED_TESTS"
        echo
        print_colored "$RED" "${CROSS} Some tests failed!"
        exit 1
    else
        print_colored "$GREEN" "Packages failed: 0"
        echo
        print_colored "$GREEN" "${CHECKMARK} All tests passed successfully!"
        
        # Generate overall coverage report
        print_colored "$PURPLE" "${INFO} Generating overall coverage report..."
        go test ./... -coverprofile=overall_coverage.out -covermode=atomic >/dev/null 2>&1
        
        if [ -f overall_coverage.out ]; then
            overall_coverage=$(go tool cover -func=overall_coverage.out | grep total | grep -oE '[0-9]+\.[0-9]+%')
            print_colored "$BLUE" "Overall test coverage: $overall_coverage"
            
            # Generate HTML coverage report
            go tool cover -html=overall_coverage.out -o coverage.html
            print_colored "$BLUE" "HTML coverage report generated: coverage.html"
            
            rm overall_coverage.out
        fi
        
        print_colored "$GREEN" "${ROCKET} Test suite completed successfully!"
    fi
}

# Show help if requested
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "test.sh - Comprehensive test runner for dropbox_downloader"
    echo
    echo "Usage: $0 [options]"
    echo
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  --verbose      Run tests with verbose output"
    echo "  --coverage     Run tests with coverage analysis only"
    echo "  --package PKG  Run tests for specific package only"
    echo
    echo "Examples:"
    echo "  $0                          # Run all tests"
    echo "  $0 --package pkg/utils      # Test utils package only"
    echo "  $0 --verbose               # Run with verbose output"
    echo
    exit 0
fi

# Handle specific package testing
if [ "$1" = "--package" ] && [ -n "$2" ]; then
    package=$2
    description="Custom Package"
    
    print_header "${ROCKET} Testing Single Package: $package"
    
    if run_package_tests "$package" "$description"; then
        run_coverage "$package" "$description"
        print_colored "$GREEN" "${CHECKMARK} Package test completed successfully!"
    else
        print_colored "$RED" "${CROSS} Package test failed!"
        exit 1
    fi
    exit 0
fi

# Handle coverage-only mode
if [ "$1" = "--coverage" ]; then
    print_header "${INFO} Coverage Analysis Only"
    
    print_colored "$PURPLE" "${INFO} Generating coverage report for all packages..."
    go test ./... -coverprofile=coverage.out -covermode=atomic
    
    if [ -f coverage.out ]; then
        go tool cover -func=coverage.out
        go tool cover -html=coverage.out -o coverage.html
        print_colored "$BLUE" "HTML coverage report generated: coverage.html"
        rm coverage.out
    fi
    exit 0
fi

# Run main function
main