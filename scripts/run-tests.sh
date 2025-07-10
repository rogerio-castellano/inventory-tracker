#!/bin/bash

set -e

echo "🧪 Running tests..."

# Ensure test database is up to date
echo "🔄 Updating test database..."
soda migrate -e test

# Reset test database (optional)
echo "🔄 Resetting test database..."
soda drop -e test || true
soda create -e test
soda migrate -e test

# Run tests
echo "🏃 Running Go tests..."
GO_ENV=test buffalo test

echo "✅ Tests complete!"
