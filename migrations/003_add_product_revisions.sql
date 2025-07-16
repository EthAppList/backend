-- Product Revision System Migration
-- This migration adds support for tracking complete revision history of products

-- Add revision tracking fields to main products table
ALTER TABLE products ADD COLUMN current_revision_number INTEGER DEFAULT 1;
ALTER TABLE products ADD COLUMN last_editor_id TEXT REFERENCES users(id);

-- Store complete snapshots of each revision
CREATE TABLE product_revisions (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    revision_number INTEGER NOT NULL,
    editor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    edit_summary TEXT,
    diff_data JSONB, -- JSON object showing what fields changed
    product_data JSONB NOT NULL, -- Complete product state at this revision
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(product_id, revision_number)
);

-- Track field-level changes for efficient diffing
CREATE TABLE product_field_changes (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
    revision_id TEXT NOT NULL REFERENCES product_revisions(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    old_value TEXT,
    new_value TEXT,
    change_type TEXT NOT NULL CHECK (change_type IN ('added', 'modified', 'removed'))
);

-- Indexes for performance
CREATE INDEX idx_product_revisions_product_id ON product_revisions(product_id);
CREATE INDEX idx_product_revisions_created_at ON product_revisions(created_at);
CREATE INDEX idx_product_revisions_editor_id ON product_revisions(editor_id);
CREATE INDEX idx_product_revisions_product_revision ON product_revisions(product_id, revision_number);
CREATE INDEX idx_field_changes_revision_id ON product_field_changes(revision_id);
CREATE INDEX idx_field_changes_field_name ON product_field_changes(field_name);

-- Initialize revision tracking for existing products
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
        'long_desc', long_desc,
        'logo_url', logo_url,
        'markdown_content', markdown_content,
        'submitter_id', submitter_id,
        'approved', approved,
        'is_verified', is_verified,
        'analytics_list', analytics_list,
        'security_score', security_score,
        'ux_score', ux_score,
        'decent_score', decent_score,
        'vibes_score', vibes_score,
        'created_at', created_at,
        'updated_at', updated_at
    ) as product_data
FROM products;

-- Update products table with current revision numbers
UPDATE products SET current_revision_number = 1; 