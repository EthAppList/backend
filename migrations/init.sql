-- Enable the required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    wallet_address TEXT UNIQUE NOT NULL,
    twitter_handle TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create categories table
CREATE TABLE IF NOT EXISTS categories (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create chains table
CREATE TABLE IF NOT EXISTS chains (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    icon TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create products table
CREATE TABLE IF NOT EXISTS products (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    short_desc TEXT NOT NULL,
    long_desc TEXT,
    logo_url TEXT,
    markdown_content TEXT,
    submitter_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    approved BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create upvotes table
CREATE TABLE IF NOT EXISTS upvotes (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, product_id)
);

-- Create product_categories junction table
CREATE TABLE IF NOT EXISTS product_categories (
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    category_id TEXT REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, category_id)
);

-- Create product_chains junction table
CREATE TABLE IF NOT EXISTS product_chains (
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    chain_id TEXT REFERENCES chains(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, chain_id)
);

-- Create pending_edits table for moderation
CREATE TABLE IF NOT EXISTS pending_edits (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    entity_type TEXT NOT NULL, -- 'product' or 'category'
    entity_id TEXT NOT NULL,
    change_type TEXT NOT NULL, -- 'create', 'update'
    change_data JSONB NOT NULL, -- JSON data with the changes
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'approved', 'rejected'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_products_approved ON products(approved);
CREATE INDEX IF NOT EXISTS idx_products_created_at ON products(created_at);
CREATE INDEX IF NOT EXISTS idx_upvotes_product_id ON upvotes(product_id);
CREATE INDEX IF NOT EXISTS idx_pending_edits_status ON pending_edits(status);
CREATE INDEX IF NOT EXISTS idx_product_categories_product_id ON product_categories(product_id);
CREATE INDEX IF NOT EXISTS idx_product_categories_category_id ON product_categories(category_id);
CREATE INDEX IF NOT EXISTS idx_product_chains_product_id ON product_chains(product_id);
CREATE INDEX IF NOT EXISTS idx_product_chains_chain_id ON product_chains(chain_id);

-- Create text search configuration for product search
CREATE INDEX IF NOT EXISTS idx_products_title_trgm ON products USING GIN (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_products_short_desc_trgm ON products USING GIN (short_desc gin_trgm_ops);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers to update the updated_at column automatically
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_categories_updated_at BEFORE UPDATE ON categories FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_chains_updated_at BEFORE UPDATE ON chains FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert some default chains
INSERT INTO chains (id, name, icon) VALUES 
('1', 'Ethereum', 'https://cryptologos.cc/logos/ethereum-eth-logo.png'),
('2', 'Polygon', 'https://cryptologos.cc/logos/polygon-matic-logo.png'),
('3', 'Solana', 'https://cryptologos.cc/logos/solana-sol-logo.png'),
('4', 'Binance Smart Chain', 'https://cryptologos.cc/logos/bnb-bnb-logo.png'),
('5', 'Arbitrum', 'https://cryptologos.cc/logos/arbitrum-arb-logo.png'),
('6', 'Optimism', 'https://cryptologos.cc/logos/optimism-ethereum-op-logo.png'),
('7', 'Avalanche', 'https://cryptologos.cc/logos/avalanche-avax-logo.png')
ON CONFLICT (id) DO NOTHING;

-- Insert some example categories
INSERT INTO categories (id, name, description) VALUES 
('1', 'DeFi', 'Decentralized Finance applications'),
('2', 'NFT', 'NFT marketplaces and tools'),
('3', 'GameFi', 'Blockchain games and gaming platforms'),
('4', 'Infrastructure', 'Blockchain infrastructure and developer tools'),
('5', 'Social', 'Social platforms built on blockchain'),
('6', 'DAO', 'Decentralized Autonomous Organizations and governance'),
('7', 'Privacy', 'Privacy-focused applications')
ON CONFLICT (id) DO NOTHING;

-- Create row level security policies
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE chains ENABLE ROW LEVEL SECURITY;
ALTER TABLE upvotes ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_chains ENABLE ROW LEVEL SECURITY;
ALTER TABLE pending_edits ENABLE ROW LEVEL SECURITY;

-- Create policies (customize these according to your actual auth setup)
CREATE POLICY "Anyone can read approved products" ON products
    FOR SELECT USING (approved = true);

CREATE POLICY "Anyone can read categories" ON categories
    FOR SELECT USING (true);

CREATE POLICY "Anyone can read chains" ON chains
    FOR SELECT USING (true);

-- Function to get top products by upvotes in a date range
CREATE OR REPLACE FUNCTION get_top_products_by_period(
    period TEXT, -- 'day', 'week', 'month', 'year', 'all'
    category_id TEXT DEFAULT NULL,
    chain_id TEXT DEFAULT NULL,
    limit_count INTEGER DEFAULT 10,
    offset_count INTEGER DEFAULT 0
) RETURNS TABLE (
    id TEXT,
    title TEXT,
    short_desc TEXT,
    long_desc TEXT,
    logo_url TEXT,
    markdown_content TEXT,
    submitter_id TEXT,
    approved BOOLEAN,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    upvote_count BIGINT
) AS $$
DECLARE
    start_date TIMESTAMP WITH TIME ZONE;
BEGIN
    -- Set start date based on period
    CASE period
        WHEN 'day' THEN start_date := CURRENT_TIMESTAMP - INTERVAL '1 day';
        WHEN 'week' THEN start_date := CURRENT_TIMESTAMP - INTERVAL '7 days';
        WHEN 'month' THEN start_date := CURRENT_TIMESTAMP - INTERVAL '30 days';
        WHEN 'year' THEN start_date := CURRENT_TIMESTAMP - INTERVAL '365 days';
        ELSE start_date := '1970-01-01'::TIMESTAMP; -- 'all' time
    END CASE;
    
    -- Handle both chain and category filters
    IF category_id IS NOT NULL AND chain_id IS NOT NULL THEN
        RETURN QUERY
        SELECT p.*, COUNT(u.id)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN upvotes u ON p.id = u.product_id AND u.created_at >= start_date
        JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_id
        JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_id
        WHERE p.approved = true
        GROUP BY p.id
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    -- Handle only category filter
    ELSIF category_id IS NOT NULL THEN
        RETURN QUERY
        SELECT p.*, COUNT(u.id)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN upvotes u ON p.id = u.product_id AND u.created_at >= start_date
        JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_id
        WHERE p.approved = true
        GROUP BY p.id
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    -- Handle only chain filter
    ELSIF chain_id IS NOT NULL THEN
        RETURN QUERY
        SELECT p.*, COUNT(u.id)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN upvotes u ON p.id = u.product_id AND u.created_at >= start_date
        JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_id
        WHERE p.approved = true
        GROUP BY p.id
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    -- No filters
    ELSE
        RETURN QUERY
        SELECT p.*, COUNT(u.id)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN upvotes u ON p.id = u.product_id AND u.created_at >= start_date
        WHERE p.approved = true
        GROUP BY p.id
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    END IF;
END;
$$ LANGUAGE plpgsql; 