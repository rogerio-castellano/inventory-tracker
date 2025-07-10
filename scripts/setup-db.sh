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

log_info "Setting up databases..."

# Wait for PostgreSQL to be ready
MAX_ATTEMPTS=30
ATTEMPT=0

while ! pg_isready -h postgres -p 5432 -U postgres -q; do
    ATTEMPT=$((ATTEMPT + 1))
    if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
        log_error "PostgreSQL is not ready after $MAX_ATTEMPTS attempts"
        exit 1
    fi
    log_info "Waiting for PostgreSQL to be ready... (attempt $ATTEMPT/$MAX_ATTEMPTS)"
    sleep 2
done

log_success "PostgreSQL is ready!"

# Function to run soda commands with error handling
run_soda_command() {
    local env=$1
    local command=$2
    local description=$3
    
    log_info "$description"
    
    if soda $command -e $env; then
        log_success "$description completed successfully"
        return 0
    else
        local exit_code=$?
        if [ $exit_code -eq 1 ] && [ "$command" = "create" ]; then
            log_warning "Database might already exist for environment: $env"
            return 0
        else
            log_error "$description failed with exit code: $exit_code"
            return $exit_code
        fi
    fi
}

# Create test database
run_soda_command "test" "create" "Creating test database"

# Check if migrations directory exists
if [ ! -d "migrations" ]; then
    log_warning "No migrations directory found. Creating empty migrations directory..."
    mkdir -p migrations
fi

# Run migrations for development
run_soda_command "development" "migrate" "Running migrations for development environment"

# Run migrations for test
run_soda_command "test" "migrate" "Running migrations for test environment"

# Verify database status
log_info "Checking database status..."
if soda schema -e development > /dev/null 2>&1; then
    log_success "Development database is accessible"
else
    log_error "Development database is not accessible"
fi

if soda schema -e test > /dev/null 2>&1; then
    log_success "Test database is accessible"
else
    log_error "Test database is not accessible"
fi

log_success "Database setup complete!"
