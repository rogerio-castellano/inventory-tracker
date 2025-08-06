#!/bin/bash
set -e

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}ℹ️  $1${NC}"; }
log_success() { echo -e "${GREEN}✅ $1${NC}"; }
log_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
log_error()   { echo -e "${RED}❌ $1${NC}"; }

log_info "Running tests in CI environment..."

# Migrate and reset DB
log_info "Resetting database..."
soda drop -e development || true
createdb -h localhost -U postgres inventory || true
soda migrate -e development

# Run Go tests with coverage
log_info "Running Go tests with coverage..."
go test ./... -coverprofile=coverage.out -covermode=atomic

# Generate coverage report
if [ -f coverage.out ]; then
    log_info "Generating coverage report..."
    go tool cover -html=coverage.out -o coverage.html

    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    log_info "Total coverage: $COVERAGE"

    THRESHOLD="70.0%"
    COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')
    THRESHOLD_NUM=$(echo $THRESHOLD | sed 's/%//')

    if (( $(echo "$COVERAGE_NUM < $THRESHOLD_NUM" | bc -l) )); then
        log_warning "Coverage $COVERAGE is below threshold $THRESHOLD"
        # exit 1  # Uncomment to enforce threshold
    else
        log_success "Coverage $COVERAGE meets threshold $THRESHOLD"
    fi
fi

log_success "CI tests completed successfully!"
