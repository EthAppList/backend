-- Migration: Add URL fields to products table
-- Description: Add website_url, github_url, docs_url, and audit_reports fields (all nullable for backward compatibility)

-- Add new URL fields to products table
ALTER TABLE products 
ADD COLUMN website_url TEXT,
ADD COLUMN github_url TEXT,
ADD COLUMN docs_url TEXT, 
ADD COLUMN audit_reports TEXT[] DEFAULT '{}'; 