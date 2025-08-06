-- Create test database
SELECT 'CREATE DATABASE inventory'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'inventory')\gexec

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE inventory TO postgres;

-- Optional: Create additional users or schemas here
-- CREATE USER inventory_user WITH PASSWORD 'inventory_password';
-- GRANT ALL PRIVILEGES ON DATABASE inventory TO inventory_user;
