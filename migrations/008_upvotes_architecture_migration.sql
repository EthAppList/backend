-- Migration 008: New Scalable Upvotes Architecture (idempotent for Railway redeploys)
-- This migration implements the new high-volume upvotes system with Redis + Postgres

-- Drop old upvotes table completely since no data needs to be preserved (idempotent)
DROP TABLE IF EXISTS upvotes CASCADE;

-- Create new votes table for immutable vote log (idempotent)
CREATE TABLE IF NOT EXISTS votes (
    id TEXT PRIMARY KEY DEFAULT CAST(EXTRACT(EPOCH FROM NOW()) * 1000000 + EXTRACT(MICROSECONDS FROM NOW()) AS TEXT),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, product_id)
);

-- Create project_stats table for rolled-up vote counts (idempotent)
CREATE TABLE IF NOT EXISTS project_stats (
    project_id TEXT PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    upvotes_total BIGINT NOT NULL DEFAULT 0,
    last_vote_ts TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for performance (idempotent)
CREATE INDEX IF NOT EXISTS idx_votes_product_id ON votes(product_id);
CREATE INDEX IF NOT EXISTS idx_votes_user_id ON votes(user_id);
CREATE INDEX IF NOT EXISTS idx_votes_created_at ON votes(created_at);
CREATE INDEX IF NOT EXISTS idx_project_stats_last_vote_ts ON project_stats(last_vote_ts);
CREATE INDEX IF NOT EXISTS idx_project_stats_upvotes_total ON project_stats(upvotes_total);

-- Initialize project_stats for all approved products with 0 votes (idempotent)
-- This ensures every approved product has a stats row, even with no votes
INSERT INTO project_stats (project_id, upvotes_total, last_vote_ts)
SELECT 
    p.id,
    0,
    p.created_at
FROM products p
WHERE p.approved = true 
  AND NOT EXISTS (SELECT 1 FROM project_stats ps WHERE ps.project_id = p.id)
ON CONFLICT (project_id) DO NOTHING;

-- Note: Redis keys and worker processes are managed by the application code
-- The new system provides:
-- 1. Immutable vote log in 'votes' table
-- 2. Rolled-up stats in 'project_stats' table  
-- 3. Real-time counters and trending in Redis
-- 4. Per-user vote state tracking in Redis with smart cache warming
-- 5. High-volume stream processing for votes 