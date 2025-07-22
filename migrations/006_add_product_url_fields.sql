-- Migration: Add URL fields to products table
-- Description: Add website_url, github_url, docs_url, and audit_reports fields (all nullable for backward compatibility)

-- Add new URL fields to products table (idempotent)
ALTER TABLE products 
ADD COLUMN IF NOT EXISTS website_url TEXT,
ADD COLUMN IF NOT EXISTS github_url TEXT,
ADD COLUMN IF NOT EXISTS docs_url TEXT, 
ADD COLUMN IF NOT EXISTS audit_reports TEXT[] DEFAULT '{}'; 