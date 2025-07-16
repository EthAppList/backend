-- Add new fields to products table
ALTER TABLE products ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE products ADD COLUMN IF NOT EXISTS analytics_list TEXT[] DEFAULT '{}';
ALTER TABLE products ADD COLUMN IF NOT EXISTS security_score DECIMAL(3,2) DEFAULT 0.50;
ALTER TABLE products ADD COLUMN IF NOT EXISTS ux_score DECIMAL(3,2) DEFAULT 0.50;
ALTER TABLE products ADD COLUMN IF NOT EXISTS decent_score DECIMAL(3,2) DEFAULT 0.50;
ALTER TABLE products ADD COLUMN IF NOT EXISTS vibes_score DECIMAL(3,2) DEFAULT 0.50;

-- Update get_top_products_by_period function to include new fields
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
    is_verified BOOLEAN,
    analytics_list TEXT[],
    security_score DECIMAL(3,2),
    ux_score DECIMAL(3,2),
    decent_score DECIMAL(3,2),
    vibes_score DECIMAL(3,2),
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