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

log_info "Setting up databases for CI..."

# Wait for PostgreSQL to be ready
MAX_ATTEMPTS=30
ATTEMPT=0

while ! pg_isready -h localhost -p 5432 -U postgres -q; do
    ATTEMPT=$((ATTEMPT + 1))
    if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
        log_error "PostgreSQL is not ready after $MAX_ATTEMPTS attempts"
        exit 1
    fi
    log_info "Waiting for PostgreSQL to be ready... (attempt $ATTEMPT/$MAX_ATTEMPTS)"
    sleep 2
done

log_success "PostgreSQL is ready!"

# Create CI-specific database.yml
log_info "Creating CI database configuration..."
cat > database.yml << EOF
inventory:
  dialect: postgres
  database: inventory
  host: localhost
  port: 5432
  user: postgres
  password: example
  pool: 5
  idle_pool: 2
  max_conn_lifetime: 3600
  sslmode: disable
EOF

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

# Create database
log_info "Creating inventory database..."
createdb -h localhost -U postgres inventory || log_warning "Inventory database might already exist"

# Check if migrations directory exists
if [ ! -d "migrations" ]; then
    log_warning "No migrations directory found. Creating empty migrations directory..."
    mkdir -p migrations
fi

# Run migrations
run_soda_command "inventory" "migrate" "Running migrations for inventory environment"

# Run migrations for test
# run_soda_command "test" "migrate" "Running migrations for test environment"

# Verify database status
log_info "Checking database status..."
if soda schema -e inventory > /dev/null 2>&1; then
    log_success "Inventory database is accessible"
else
    log_error "Inventory database is not accessible"
fi

# if soda schema -e test > /dev/null 2>&1; then
#     log_success "Test database is accessible"
# else
#     log_error "Test database is not accessible"
# fi

log_success "CI Database setup complete!"