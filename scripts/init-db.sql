-- Create test database
SELECT 'CREATE DATABASE inventory_test'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'inventory_test')\gexec

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE inventory_development TO postgres;
GRANT ALL PRIVILEGES ON DATABASE inventory_test TO postgres;

-- Optional: Create additional users or schemas here
-- CREATE USER inventory_user WITH PASSWORD 'inventory_password';
-- GRANT ALL PRIVILEGES ON DATABASE inventory_development TO inventory_user;
-- GRANT ALL PRIVILEGES ON DATABASE inventory_test TO inventory_user;
