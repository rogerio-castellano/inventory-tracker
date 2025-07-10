#!/bin/bash

set -e

echo "🔄 Setting up local CI simulation..."

# Start PostgreSQL in background (if not running)
if ! pg_isready -h localhost -p 5432 -U postgres -q 2>/dev/null; then
    echo "Starting PostgreSQL..."
    # Adjust this command based on your local setup
    # brew services start postgresql  # macOS
    # sudo systemctl start postgresql  # Linux
    # pg_ctl -D /usr/local/var/postgres start  # Manual start
fi

# Install dependencies if not present
if ! command -v buffalo &> /dev/null; then
    echo "Installing Buffalo CLI..."
    go install github.com/gobuffalo/buffalo/cmd/buffalo@latest
fi

if ! command -v soda &> /dev/null; then
    echo "Installing Soda CLI..."
    go install github.com/gobuffalo/pop/v6/soda@latest
fi

# Set environment variables
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/app_development?sslmode=disable"
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/app_test?sslmode=disable"
export PGPASSWORD="postgres"
export GO_ENV="test"

# Run the same setup as CI
echo "Running database setup..."
./scripts/setup-db-ci.sh

echo "Running tests..."
./scripts/run-tests-ci.sh

echo "✅ Local CI simulation complete!"