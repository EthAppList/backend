-- Migration 007: Remove legacy pending_edits table and transition to Redis-based workflow (idempotent)
-- This migration removes the old database-based pending_edits system in favor of Redis

-- Drop indexes related to pending_edits (idempotent)
DROP INDEX IF EXISTS idx_pending_edits_status;

-- Drop the pending_edits table completely since we're moving to Redis (idempotent)
DROP TABLE IF EXISTS pending_edits;

-- Note: All pending changes are now managed via Redis for the git-like workflow
-- The new system provides:
-- 1. Better performance with Redis caching
-- 2. Git-like workflow where live products stay visible during review
-- 3. Cleaner separation between pending changes and live data
-- 4. Automatic cleanup of processed changes 