-- Enable the required extensions (idempotent)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Create users table (idempotent)
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    wallet_address TEXT UNIQUE NOT NULL,
    twitter_handle TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create categories table (idempotent)
CREATE TABLE IF NOT EXISTS categories (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create chains table (idempotent)
CREATE TABLE IF NOT EXISTS chains (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    icon TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create products table (idempotent)
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

-- Create upvotes table (idempotent)
CREATE TABLE IF NOT EXISTS upvotes (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, product_id)
);

-- Create product_categories junction table (idempotent)
CREATE TABLE IF NOT EXISTS product_categories (
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    category_id TEXT REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, category_id)
);

-- Create product_chains junction table (idempotent)
CREATE TABLE IF NOT EXISTS product_chains (
    product_id TEXT REFERENCES products(id) ON DELETE CASCADE,
    chain_id TEXT REFERENCES chains(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, chain_id)
);

-- Create pending_edits table for moderation (idempotent)
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

-- Create indexes for performance (idempotent)
CREATE INDEX IF NOT EXISTS idx_products_approved ON products(approved);
CREATE INDEX IF NOT EXISTS idx_products_created_at ON products(created_at);
CREATE INDEX IF NOT EXISTS idx_upvotes_product_id ON upvotes(product_id);
CREATE INDEX IF NOT EXISTS idx_pending_edits_status ON pending_edits(status);
CREATE INDEX IF NOT EXISTS idx_product_categories_product_id ON product_categories(product_id);
CREATE INDEX IF NOT EXISTS idx_product_categories_category_id ON product_categories(category_id);
CREATE INDEX IF NOT EXISTS idx_product_chains_product_id ON product_chains(product_id);
CREATE INDEX IF NOT EXISTS idx_product_chains_chain_id ON product_chains(chain_id);

-- Create text search configuration for product search (idempotent)
CREATE INDEX IF NOT EXISTS idx_products_title_trgm ON products USING GIN (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_products_short_desc_trgm ON products USING GIN (short_desc gin_trgm_ops);

-- Create function to update updated_at timestamp (idempotent)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;



-- Create triggers to update the updated_at column automatically (idempotent)
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
DROP TRIGGER IF EXISTS update_products_updated_at ON products;
CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
DROP TRIGGER IF EXISTS update_categories_updated_at ON categories;
CREATE TRIGGER update_categories_updated_at BEFORE UPDATE ON categories FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
DROP TRIGGER IF EXISTS update_chains_updated_at ON chains;
CREATE TRIGGER update_chains_updated_at BEFORE UPDATE ON chains FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert some default chains using name as ID (idempotent)
-- Use ON CONFLICT on name to handle existing chains with different IDs
INSERT INTO chains (id, name, icon) VALUES 
('Ethereum', 'Ethereum', 'https://cryptologos.cc/logos/ethereum-eth-logo.png'),
('Polygon', 'Polygon', 'https://cryptologos.cc/logos/polygon-matic-logo.png'),
('Solana', 'Solana', 'https://cryptologos.cc/logos/solana-sol-logo.png'),
('Binance Smart Chain', 'Binance Smart Chain', 'https://cryptologos.cc/logos/bnb-bnb-logo.png'),
('Arbitrum', 'Arbitrum', 'https://cryptologos.cc/logos/arbitrum-arb-logo.png'),
('Optimism', 'Optimism', 'https://cryptologos.cc/logos/optimism-ethereum-op-logo.png'),
('Avalanche', 'Avalanche', 'https://cryptologos.cc/logos/avalanche-avax-logo.png')
ON CONFLICT (name) DO UPDATE SET 
    id = EXCLUDED.id,
    icon = EXCLUDED.icon;

-- Insert some example categories using name as ID (idempotent)
-- Use ON CONFLICT on name to handle existing categories with different IDs
INSERT INTO categories (id, name, description) VALUES 
('DeFi', 'DeFi', 'Decentralized Finance applications'),
('NFT', 'NFT', 'NFT marketplaces and tools'),
('GameFi', 'GameFi', 'Blockchain games and gaming platforms'),
('Infrastructure', 'Infrastructure', 'Blockchain infrastructure and developer tools'),
('Social', 'Social', 'Social platforms built on blockchain'),
('DAO', 'DAO', 'Decentralized Autonomous Organizations and governance'),
('Privacy', 'Privacy', 'Privacy-focused applications')
ON CONFLICT (name) DO UPDATE SET 
    id = EXCLUDED.id,
    description = EXCLUDED.description;

-- Create row level security policies (idempotent)
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE chains ENABLE ROW LEVEL SECURITY;
ALTER TABLE upvotes ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_chains ENABLE ROW LEVEL SECURITY;
ALTER TABLE pending_edits ENABLE ROW LEVEL SECURITY;

-- Create policies (idempotent - will replace existing)
DROP POLICY IF EXISTS "Anyone can read approved products" ON products;
CREATE POLICY "Anyone can read approved products" ON products
    FOR SELECT USING (approved = true);

DROP POLICY IF EXISTS "Anyone can read categories" ON categories;
CREATE POLICY "Anyone can read categories" ON categories
    FOR SELECT USING (true);

DROP POLICY IF EXISTS "Anyone can read chains" ON chains;
CREATE POLICY "Anyone can read chains" ON chains
    FOR SELECT USING (true);

-- Note: The get_top_products_by_period function is defined in migration 002
-- after the new product fields are added to ensure proper column matching 