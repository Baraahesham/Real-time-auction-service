-- Auction Service Database Schema
-- PostgreSQL schema for the auction service

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Auctions table
CREATE TABLE IF NOT EXISTS auctions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    creator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    starting_price DECIMAL(10,2) NOT NULL CHECK (starting_price > 0),
    current_price DECIMAL(10,2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'ended', 'cancelled')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Bids table
CREATE TABLE IF NOT EXISTS bids (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id UUID NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(10,2) NOT NULL CHECK (amount > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_auctions_item_id ON auctions(item_id);
CREATE INDEX IF NOT EXISTS idx_auctions_creator_id ON auctions(creator_id);
CREATE INDEX IF NOT EXISTS idx_auctions_status ON auctions(status);
CREATE INDEX IF NOT EXISTS idx_auctions_start_time ON auctions(start_time);
CREATE INDEX IF NOT EXISTS idx_auctions_end_time ON auctions(end_time);

-- NEW: Composite index for the new GetActiveByItemID query (most important for performance)
CREATE INDEX IF NOT EXISTS idx_auctions_item_status ON auctions(item_id, status) WHERE status = 'active';

-- NEW: Composite index for auction listing with status filter
CREATE INDEX IF NOT EXISTS idx_auctions_status_created ON auctions(status, created_at DESC);

-- NEW: Index for auction expiration queries (scheduler)
CREATE INDEX IF NOT EXISTS idx_auctions_status_end_time ON auctions(status, end_time) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_bids_auction_id ON bids(auction_id);
CREATE INDEX IF NOT EXISTS idx_bids_user_id ON bids(user_id);
CREATE INDEX IF NOT EXISTS idx_bids_status ON bids(status);
CREATE INDEX IF NOT EXISTS idx_bids_amount ON bids(amount);
CREATE INDEX IF NOT EXISTS idx_bids_created_at ON bids(created_at);

-- Composite index for getting highest bid efficiently
CREATE INDEX IF NOT EXISTS idx_bids_auction_amount ON bids(auction_id, amount DESC);

-- NEW: Composite index for GetHighestBid query (most important for bid performance)
CREATE INDEX IF NOT EXISTS idx_bids_auction_status_amount ON bids(auction_id, status, amount DESC) WHERE status = 'accepted';

-- NEW: Index for bid listing by auction
CREATE INDEX IF NOT EXISTS idx_bids_auction_created ON bids(auction_id, created_at DESC);

-- NEW: Index for user activity queries
CREATE INDEX IF NOT EXISTS idx_bids_user_created ON bids(user_id, created_at DESC);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to automatically update updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_items_updated_at BEFORE UPDATE ON items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_auctions_updated_at BEFORE UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_bids_updated_at BEFORE UPDATE ON bids
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Sample data for testing
INSERT INTO users (id, name) VALUES 
    ('550e8400-e29b-41d4-a716-446655440001', 'John'),
    ('550e8400-e29b-41d4-a716-446655440002', 'Alice'),
    ('550e8400-e29b-41d4-a716-446655440003', 'Bob')
ON CONFLICT (id) DO NOTHING;

INSERT INTO items (id, name, description) VALUES 
    ('660e8400-e29b-41d4-a716-446655440001', 'Vintage Watch', 'A beautiful vintage watch'),
    ('660e8400-e29b-41d4-a716-446655440002', 'Art Painting', 'Original oil painting by a famous artist'),
    ('660e8400-e29b-41d4-a716-446655440003', 'Antique Vase', 'Ming dynasty antique vase')
ON CONFLICT (id) DO NOTHING; 