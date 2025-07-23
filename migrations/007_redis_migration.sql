-- Migration 007: Transition to Redis-based pending changes system (idempotent)
-- This migration documents the transition from database-based pending_edits to Redis-based workflow

-- Add comment to the pending_edits table to document that it's now legacy
COMMENT ON TABLE pending_edits IS 'Legacy table - pending changes now stored in Redis for git-like workflow. This table may still be used for backwards compatibility.';

-- Add a column to track migration status (idempotent)
ALTER TABLE pending_edits ADD COLUMN IF NOT EXISTS migrated_to_redis BOOLEAN DEFAULT FALSE;

-- Optional: Clear any existing pending edits since they will be handled by Redis now
-- Uncomment the following lines if you want to clear existing pending edits during migration
-- UPDATE pending_edits SET status = 'migrated' WHERE status = 'pending';
-- UPDATE pending_edits SET migrated_to_redis = TRUE WHERE status = 'migrated';

-- Create an index for the new column (idempotent)
CREATE INDEX IF NOT EXISTS idx_pending_edits_migrated_to_redis ON pending_edits(migrated_to_redis);

-- Add migration timestamp
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'pending_edits' 
        AND column_name = 'redis_migration_date'
    ) THEN
        ALTER TABLE pending_edits ADD COLUMN redis_migration_date TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
    END IF;
END $$; 