-- Fix Revision Tables Migration (idempotent)
-- This migration ensures revision tables use proper ID generation for Railway deployment

-- Only proceed if we need to fix the tables (check if gen_random_uuid is being used)
DO $$
DECLARE
    needs_fix BOOLEAN := FALSE;
    table_exists BOOLEAN := FALSE;
BEGIN
    -- Check if product_revisions table exists
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables 
        WHERE table_name = 'product_revisions' AND table_schema = 'public'
    ) INTO table_exists;
    
    -- If table exists, check if it's using problematic ID generation
    IF table_exists THEN
        -- Check if the default uses gen_random_uuid (which might not work in Railway)
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'product_revisions' 
            AND column_name = 'id'
            AND column_default LIKE '%gen_random_uuid%'
        ) INTO needs_fix;
    END IF;
    
    -- If we need to fix or table doesn't exist, recreate it
    IF needs_fix OR NOT table_exists THEN
        -- Drop existing revision tables if they exist (with CASCADE to handle foreign keys)
        DROP TABLE IF EXISTS product_field_changes CASCADE;
        DROP TABLE IF EXISTS product_revisions CASCADE;
        
        -- Recreate product_revisions table with timestamp-based ID generation
        CREATE TABLE product_revisions (
            id TEXT PRIMARY KEY DEFAULT CAST(EXTRACT(EPOCH FROM NOW()) * 1000000 + EXTRACT(MICROSECONDS FROM NOW()) AS TEXT),
            product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
            revision_number INTEGER NOT NULL,
            editor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
            edit_summary TEXT,
            diff_data JSONB, -- JSON object showing what fields changed
            product_data JSONB NOT NULL, -- Complete product state at this revision
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            
            UNIQUE(product_id, revision_number)
        );
        
        -- Recreate product_field_changes table with timestamp-based ID generation
        CREATE TABLE product_field_changes (
            id TEXT PRIMARY KEY DEFAULT CAST(EXTRACT(EPOCH FROM NOW()) * 1000000 + EXTRACT(MICROSECONDS FROM NOW()) + RANDOM() * 1000 AS TEXT),
            revision_id TEXT NOT NULL REFERENCES product_revisions(id) ON DELETE CASCADE,
            field_name TEXT NOT NULL,
            old_value TEXT,
            new_value TEXT,
            change_type TEXT NOT NULL CHECK (change_type IN ('added', 'modified', 'removed'))
        );
        
        -- Recreate indexes for performance
        CREATE INDEX idx_product_revisions_product_id ON product_revisions(product_id);
        CREATE INDEX idx_product_revisions_created_at ON product_revisions(created_at);
        CREATE INDEX idx_product_revisions_editor_id ON product_revisions(editor_id);
        CREATE INDEX idx_product_revisions_product_revision ON product_revisions(product_id, revision_number);
        CREATE INDEX idx_field_changes_revision_id ON product_field_changes(revision_id);
        CREATE INDEX idx_field_changes_field_name ON product_field_changes(field_name);
    END IF;
END $$;

-- Ensure products table has the revision fields (idempotent)
ALTER TABLE products ADD COLUMN IF NOT EXISTS current_revision_number INTEGER DEFAULT 1;
ALTER TABLE products ADD COLUMN IF NOT EXISTS last_editor_id TEXT REFERENCES users(id);

-- Initialize revision tracking for existing products (idempotent)
-- This will only insert if the revision doesn't already exist
INSERT INTO product_revisions (product_id, revision_number, editor_id, edit_summary, product_data)
SELECT 
    id as product_id,
    1 as revision_number,
    submitter_id as editor_id,
    'Initial product version' as edit_summary,
    jsonb_build_object(
        'id', id,
        'title', title,
        'short_desc', short_desc,
        'long_desc', COALESCE(long_desc, ''),
        'logo_url', COALESCE(logo_url, ''),
        'markdown_content', COALESCE(markdown_content, ''),
        'submitter_id', submitter_id,
        'approved', approved,
        'is_verified', COALESCE(is_verified, false),
        'analytics_list', COALESCE(analytics_list, '{}'),
        'security_score', COALESCE(security_score, 0.5),
        'ux_score', COALESCE(ux_score, 0.5),
        'decent_score', COALESCE(decent_score, 0.5),
        'vibes_score', COALESCE(vibes_score, 0.5),
        'created_at', created_at,
        'updated_at', updated_at
    ) as product_data
FROM products
WHERE approved = true
  AND EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'product_revisions')
  AND NOT EXISTS (
      SELECT 1 FROM product_revisions pr 
      WHERE pr.product_id = products.id AND pr.revision_number = 1
  )
ON CONFLICT (product_id, revision_number) DO NOTHING;

-- Update products table with current revision numbers (idempotent)
UPDATE products 
SET current_revision_number = 1 
WHERE current_revision_number IS NULL; 