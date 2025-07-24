-- Migration 009: Fix Stored Functions to Use New project_stats Table (idempotent for Railway redeploys)
-- This migration updates all stored functions to use the new project_stats table instead of the old upvotes table

-- Drop and recreate the functions that were referencing the old upvotes table

-- Fix the get_products_by_category_chain_votes_week function
DROP FUNCTION IF EXISTS get_products_by_category_chain_votes_week(TEXT, TEXT);
CREATE OR REPLACE FUNCTION get_products_by_category_chain_votes_week(category_filter TEXT, chain_filter TEXT)
RETURNS TABLE (
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
    security_score DOUBLE PRECISION,
    ux_score DOUBLE PRECISION,
    decent_score DOUBLE PRECISION,
    vibes_score DOUBLE PRECISION,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    upvote_count BIGINT
) AS $$
DECLARE
    start_date TIMESTAMPTZ := NOW() - INTERVAL '7 days';
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.title,
        p.short_desc,
        p.long_desc,
        p.logo_url,
        p.markdown_content,
        p.submitter_id,
        p.approved,
        p.is_verified,
        p.analytics_list,
        COALESCE(p.security_score, 0.5) as security_score,
        COALESCE(p.ux_score, 0.5) as ux_score,
        COALESCE(p.decent_score, 0.5) as decent_score,
        COALESCE(p.vibes_score, 0.5) as vibes_score,
        p.created_at,
        p.updated_at,
        COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
    FROM products p
    LEFT JOIN project_stats ps ON p.id = ps.project_id
    JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_filter
    JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_filter
    WHERE p.approved = true
    ORDER BY upvote_count DESC, p.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Fix the get_products_by_category_chain_votes_month function
DROP FUNCTION IF EXISTS get_products_by_category_chain_votes_month(TEXT, TEXT);
CREATE OR REPLACE FUNCTION get_products_by_category_chain_votes_month(category_filter TEXT, chain_filter TEXT)
RETURNS TABLE (
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
    security_score DOUBLE PRECISION,
    ux_score DOUBLE PRECISION,
    decent_score DOUBLE PRECISION,
    vibes_score DOUBLE PRECISION,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    upvote_count BIGINT
) AS $$
DECLARE
    start_date TIMESTAMPTZ := NOW() - INTERVAL '30 days';
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.title,
        p.short_desc,
        p.long_desc,
        p.logo_url,
        p.markdown_content,
        p.submitter_id,
        p.approved,
        p.is_verified,
        p.analytics_list,
        COALESCE(p.security_score, 0.5) as security_score,
        COALESCE(p.ux_score, 0.5) as ux_score,
        COALESCE(p.decent_score, 0.5) as decent_score,
        COALESCE(p.vibes_score, 0.5) as vibes_score,
        p.created_at,
        p.updated_at,
        COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
    FROM products p
    LEFT JOIN project_stats ps ON p.id = ps.project_id
    JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_filter
    JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_filter
    WHERE p.approved = true
    ORDER BY upvote_count DESC, p.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Fix the get_products_by_category_chain_votes_year function
DROP FUNCTION IF EXISTS get_products_by_category_chain_votes_year(TEXT, TEXT);
CREATE OR REPLACE FUNCTION get_products_by_category_chain_votes_year(category_filter TEXT, chain_filter TEXT)
RETURNS TABLE (
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
    security_score DOUBLE PRECISION,
    ux_score DOUBLE PRECISION,
    decent_score DOUBLE PRECISION,
    vibes_score DOUBLE PRECISION,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    upvote_count BIGINT
) AS $$
DECLARE
    start_date TIMESTAMPTZ := NOW() - INTERVAL '365 days';
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.title,
        p.short_desc,
        p.long_desc,
        p.logo_url,
        p.markdown_content,
        p.submitter_id,
        p.approved,
        p.is_verified,
        p.analytics_list,
        COALESCE(p.security_score, 0.5) as security_score,
        COALESCE(p.ux_score, 0.5) as ux_score,
        COALESCE(p.decent_score, 0.5) as decent_score,
        COALESCE(p.vibes_score, 0.5) as vibes_score,
        p.created_at,
        p.updated_at,
        COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
    FROM products p
    LEFT JOIN project_stats ps ON p.id = ps.project_id
    JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_filter
    JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_filter
    WHERE p.approved = true
    ORDER BY upvote_count DESC, p.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Fix the get_products_by_category_chain_votes_all function
DROP FUNCTION IF EXISTS get_products_by_category_chain_votes_all(TEXT, TEXT);
CREATE OR REPLACE FUNCTION get_products_by_category_chain_votes_all(category_filter TEXT, chain_filter TEXT)
RETURNS TABLE (
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
    security_score DOUBLE PRECISION,
    ux_score DOUBLE PRECISION,
    decent_score DOUBLE PRECISION,
    vibes_score DOUBLE PRECISION,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    upvote_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        p.id,
        p.title,
        p.short_desc,
        p.long_desc,
        p.logo_url,
        p.markdown_content,
        p.submitter_id,
        p.approved,
        p.is_verified,
        p.analytics_list,
        COALESCE(p.security_score, 0.5) as security_score,
        COALESCE(p.ux_score, 0.5) as ux_score,
        COALESCE(p.decent_score, 0.5) as decent_score,
        COALESCE(p.vibes_score, 0.5) as vibes_score,
        p.created_at,
        p.updated_at,
        COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
    FROM products p
    LEFT JOIN project_stats ps ON p.id = ps.project_id
    JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_filter
    JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_filter
    WHERE p.approved = true
    ORDER BY upvote_count DESC, p.created_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Fix the get_top_products_by_period function that was causing the main error
DROP FUNCTION IF EXISTS get_top_products_by_period(TEXT, TEXT, TEXT, INTEGER, INTEGER);
CREATE OR REPLACE FUNCTION get_top_products_by_period(
    period TEXT, -- 'day', 'week', 'month', 'year', 'all'
    category_filter TEXT DEFAULT NULL,
    chain_filter TEXT DEFAULT NULL,
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
    IF category_filter IS NOT NULL AND chain_filter IS NOT NULL THEN
        RETURN QUERY
        SELECT 
            p.id,
            p.title,
            p.short_desc,
            COALESCE(p.long_desc, '') as long_desc,
            COALESCE(p.logo_url, '') as logo_url,
            COALESCE(p.markdown_content, '') as markdown_content,
            p.submitter_id,
            p.approved,
            COALESCE(p.is_verified, false) as is_verified,
            COALESCE(p.analytics_list, '{}') as analytics_list,
            COALESCE(p.security_score, 0.5) as security_score,
            COALESCE(p.ux_score, 0.5) as ux_score,
            COALESCE(p.decent_score, 0.5) as decent_score,
            COALESCE(p.vibes_score, 0.5) as vibes_score,
            p.created_at,
            p.updated_at,
            COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN project_stats ps ON p.id = ps.project_id
        JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_filter
        JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_filter
        WHERE p.approved = true
        GROUP BY p.id, p.title, p.short_desc, p.long_desc, p.logo_url, p.markdown_content, 
                 p.submitter_id, p.approved, p.is_verified, p.analytics_list, 
                 p.security_score, p.ux_score, p.decent_score, p.vibes_score, 
                 p.created_at, p.updated_at, ps.upvotes_total
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    -- Handle only category filter
    ELSIF category_filter IS NOT NULL THEN
        RETURN QUERY
        SELECT 
            p.id,
            p.title,
            p.short_desc,
            COALESCE(p.long_desc, '') as long_desc,
            COALESCE(p.logo_url, '') as logo_url,
            COALESCE(p.markdown_content, '') as markdown_content,
            p.submitter_id,
            p.approved,
            COALESCE(p.is_verified, false) as is_verified,
            COALESCE(p.analytics_list, '{}') as analytics_list,
            COALESCE(p.security_score, 0.5) as security_score,
            COALESCE(p.ux_score, 0.5) as ux_score,
            COALESCE(p.decent_score, 0.5) as decent_score,
            COALESCE(p.vibes_score, 0.5) as vibes_score,
            p.created_at,
            p.updated_at,
            COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN project_stats ps ON p.id = ps.project_id
        JOIN product_categories pc ON p.id = pc.product_id AND pc.category_id = category_filter
        WHERE p.approved = true
        GROUP BY p.id, p.title, p.short_desc, p.long_desc, p.logo_url, p.markdown_content, 
                 p.submitter_id, p.approved, p.is_verified, p.analytics_list, 
                 p.security_score, p.ux_score, p.decent_score, p.vibes_score, 
                 p.created_at, p.updated_at, ps.upvotes_total
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    -- Handle only chain filter
    ELSIF chain_filter IS NOT NULL THEN
        RETURN QUERY
        SELECT 
            p.id,
            p.title,
            p.short_desc,
            COALESCE(p.long_desc, '') as long_desc,
            COALESCE(p.logo_url, '') as logo_url,
            COALESCE(p.markdown_content, '') as markdown_content,
            p.submitter_id,
            p.approved,
            COALESCE(p.is_verified, false) as is_verified,
            COALESCE(p.analytics_list, '{}') as analytics_list,
            COALESCE(p.security_score, 0.5) as security_score,
            COALESCE(p.ux_score, 0.5) as ux_score,
            COALESCE(p.decent_score, 0.5) as decent_score,
            COALESCE(p.vibes_score, 0.5) as vibes_score,
            p.created_at,
            p.updated_at,
            COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN project_stats ps ON p.id = ps.project_id
        JOIN product_chains ch ON p.id = ch.product_id AND ch.chain_id = chain_filter
        WHERE p.approved = true
        GROUP BY p.id, p.title, p.short_desc, p.long_desc, p.logo_url, p.markdown_content, 
                 p.submitter_id, p.approved, p.is_verified, p.analytics_list, 
                 p.security_score, p.ux_score, p.decent_score, p.vibes_score, 
                 p.created_at, p.updated_at, ps.upvotes_total
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    -- Handle no filters
    ELSE
        RETURN QUERY
        SELECT 
            p.id,
            p.title,
            p.short_desc,
            COALESCE(p.long_desc, '') as long_desc,
            COALESCE(p.logo_url, '') as logo_url,
            COALESCE(p.markdown_content, '') as markdown_content,
            p.submitter_id,
            p.approved,
            COALESCE(p.is_verified, false) as is_verified,
            COALESCE(p.analytics_list, '{}') as analytics_list,
            COALESCE(p.security_score, 0.5) as security_score,
            COALESCE(p.ux_score, 0.5) as ux_score,
            COALESCE(p.decent_score, 0.5) as decent_score,
            COALESCE(p.vibes_score, 0.5) as vibes_score,
            p.created_at,
            p.updated_at,
            COALESCE(ps.upvotes_total, 0)::BIGINT AS upvote_count
        FROM products p
        LEFT JOIN project_stats ps ON p.id = ps.project_id
        WHERE p.approved = true
        GROUP BY p.id, p.title, p.short_desc, p.long_desc, p.logo_url, p.markdown_content, 
                 p.submitter_id, p.approved, p.is_verified, p.analytics_list, 
                 p.security_score, p.ux_score, p.decent_score, p.vibes_score, 
                 p.created_at, p.updated_at, ps.upvotes_total
        ORDER BY upvote_count DESC, p.created_at DESC
        LIMIT limit_count
        OFFSET offset_count;
    END IF;
END;
$$ LANGUAGE plpgsql; 