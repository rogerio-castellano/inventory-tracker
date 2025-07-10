#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_info "Running tests in CI environment..."

# Ensure test database is up to date
log_info "Ensuring test database is up to date..."
soda migrate -e test

# Reset test database for clean state
log_info "Resetting test database for clean state..."
soda drop -e test || true
createdb -h localhost -U postgres inventory_tests || true
soda migrate -e test

# Run tests with coverage
log_info "Running Go tests with coverage..."
if buffalo test --coverprofile=coverage.out; then
    log_success "Tests passed!"
else
    log_error "Tests failed!"
    exit 1
fi

# Generate coverage report
if [ -f coverage.out ]; then
    log_info "Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    
    # Calculate coverage percentage
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    log_info "Total coverage: $COVERAGE"
    
    # Set minimum coverage threshold (adjust as needed)
    THRESHOLD="70.0%"
    COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')
    THRESHOLD_NUM=$(echo $THRESHOLD | sed 's/%//')
    
    if (( $(echo "$COVERAGE_NUM < $THRESHOLD_NUM" | bc -l) )); then
        log_warning "Coverage $COVERAGE is below threshold $THRESHOLD"
        # Uncomment to fail on low coverage
        # exit 1
    else
        log_success "Coverage $COVERAGE meets threshold $THRESHOLD"
    fi
fi

log_success "CI tests completed successfully!"