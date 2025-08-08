-- Database initialization script for Auction Service
-- This script will be automatically executed when the PostgreSQL container starts

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create database if it doesn't exist (PostgreSQL creates it automatically via POSTGRES_DB env var)
-- But we can add any additional setup here

-- Grant necessary permissions
GRANT ALL PRIVILEGES ON DATABASE auction_service TO postgres;

-- Set timezone
SET timezone = 'UTC';

-- Log successful initialization
DO $$
BEGIN
    RAISE NOTICE 'Auction Service database initialized successfully';
END $$; 