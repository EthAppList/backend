-- Migration: Add product score submissions tracking
-- Description: Add table to track individual score submissions and calculate averages

-- Create table to track individual score submissions (idempotent)
CREATE TABLE IF NOT EXISTS product_score_submissions (
    id TEXT PRIMARY KEY DEFAULT CAST(EXTRACT(EPOCH FROM NOW()) * 1000000 + EXTRACT(MICROSECONDS FROM NOW()) + RANDOM() * 1000 AS TEXT),
    product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    overall_score DECIMAL(3,2) NOT NULL CHECK (overall_score >= 0 AND overall_score <= 1),
    security_score DECIMAL(3,2) NOT NULL CHECK (security_score >= 0 AND security_score <= 1),
    ux_score DECIMAL(3,2) NOT NULL CHECK (ux_score >= 0 AND ux_score <= 1),
    vibes_score DECIMAL(3,2) NOT NULL CHECK (vibes_score >= 0 AND vibes_score <= 1),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- One score submission per user per product
    UNIQUE(product_id, user_id)
);

-- Create indexes for performance (idempotent)
CREATE INDEX IF NOT EXISTS idx_score_submissions_product_id ON product_score_submissions(product_id);
CREATE INDEX IF NOT EXISTS idx_score_submissions_user_id ON product_score_submissions(user_id);
CREATE INDEX IF NOT EXISTS idx_score_submissions_created_at ON product_score_submissions(created_at);

-- Add fields to products table to track score aggregations (idempotent)
ALTER TABLE products ADD COLUMN IF NOT EXISTS score_count INTEGER DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS overall_score_sum DECIMAL(10,2) DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS security_score_sum DECIMAL(10,2) DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS ux_score_sum DECIMAL(10,2) DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS vibes_score_sum DECIMAL(10,2) DEFAULT 0;

-- Function to recalculate average scores for a product (idempotent)
CREATE OR REPLACE FUNCTION recalculate_product_scores(target_product_id TEXT)
RETURNS VOID AS $$
DECLARE
    score_data RECORD;
BEGIN
    -- Get aggregated scores for the product
    SELECT 
        COUNT(*) as submission_count,
        COALESCE(SUM(overall_score), 0) as overall_sum,
        COALESCE(SUM(security_score), 0) as security_sum,
        COALESCE(SUM(ux_score), 0) as ux_sum,
        COALESCE(SUM(vibes_score), 0) as vibes_sum
    INTO score_data
    FROM product_score_submissions 
    WHERE product_id = target_product_id;
    
    -- Update the products table with aggregated data
    UPDATE products SET
        score_count = score_data.submission_count,
        overall_score_sum = score_data.overall_sum,
        security_score_sum = score_data.security_sum,
        ux_score_sum = score_data.ux_sum,
        vibes_score_sum = score_data.vibes_sum,
        -- Calculate averages (fallback to 0.5 if no scores)
        overall_score = CASE 
            WHEN score_data.submission_count > 0 THEN score_data.overall_sum / score_data.submission_count
            ELSE 0.5 
        END,
        security_score = CASE 
            WHEN score_data.submission_count > 0 THEN score_data.security_sum / score_data.submission_count
            ELSE 0.5 
        END,
        ux_score = CASE 
            WHEN score_data.submission_count > 0 THEN score_data.ux_sum / score_data.submission_count
            ELSE 0.5 
        END,
        vibes_score = CASE 
            WHEN score_data.submission_count > 0 THEN score_data.vibes_sum / score_data.submission_count
            ELSE 0.5 
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = target_product_id;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically recalculate scores when submissions change (idempotent)
CREATE OR REPLACE FUNCTION trigger_recalculate_scores()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM recalculate_product_scores(OLD.product_id);
        RETURN OLD;
    ELSE
        PERFORM recalculate_product_scores(NEW.product_id);
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Drop and recreate trigger to ensure it's up to date
DROP TRIGGER IF EXISTS trigger_score_submissions_change ON product_score_submissions;
CREATE TRIGGER trigger_score_submissions_change
    AFTER INSERT OR UPDATE OR DELETE ON product_score_submissions
    FOR EACH ROW EXECUTE FUNCTION trigger_recalculate_scores(); 