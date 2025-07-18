# Database Migrations (Idempotent System)

This directory contains database migration files that are automatically executed **every time** the application starts. All migrations are designed to be **idempotent** (safe to run multiple times).

## How It Works

The migration system:
1. Scans this directory for `.sql` files on app startup
2. Runs **ALL** migrations in version order every time
3. Each migration is designed to be safe to run repeatedly
4. No migration state tracking - all migrations run each startup

## Idempotent Design Principles

All migrations must follow these patterns to be safe to run multiple times:

### Tables and Columns
```sql
-- ✅ Good - Safe to run multiple times
CREATE TABLE IF NOT EXISTS users (...);
ALTER TABLE products ADD COLUMN IF NOT EXISTS new_field TEXT;

-- ❌ Bad - Will fail on second run
CREATE TABLE users (...);
ALTER TABLE products ADD COLUMN new_field TEXT;
```

### Indexes
```sql
-- ✅ Good
CREATE INDEX IF NOT EXISTS idx_products_title ON products(title);

-- ❌ Bad  
CREATE INDEX idx_products_title ON products(title);
```

### Functions and Policies
```sql
-- ✅ Good - Replaces existing
CREATE OR REPLACE FUNCTION my_function() ...;
DROP POLICY IF EXISTS "policy_name" ON table_name;
CREATE POLICY "policy_name" ON table_name ...;

-- ❌ Bad - Will fail if exists
CREATE FUNCTION my_function() ...;
CREATE POLICY "policy_name" ON table_name ...;
```

### Data Insertion
```sql
-- ✅ Good - Won't create duplicates
INSERT INTO categories (id, name) VALUES ('1', 'DeFi')
ON CONFLICT (id) DO NOTHING;

-- ❌ Bad - Will fail if data exists
INSERT INTO categories (id, name) VALUES ('1', 'DeFi');
```

## Naming Convention

Migration files must follow this naming pattern:
```
XXX_description.sql
```

Where:
- `XXX` is a 3-digit version number (001, 002, 003, etc.)
- `description` is a brief description of what the migration does

### Examples:
- `001_init.sql` - Initial database schema
- `002_add_product_fields.sql` - Add new fields to products table
- `003_add_product_revisions.sql` - Add revision tracking
- `004_fix_revision_tables.sql` - Fix table issues

## Benefits of This Approach

✅ **Easy Updates**: Modify existing migrations during development  
✅ **Self-Healing**: Automatically fixes missing tables/columns  
✅ **Simple Deployment**: No migration state to track  
✅ **Reliable**: Consistent database state on every startup  

## Trade-offs

⚠️ **Slower Startup**: Runs all migrations each time (usually quick)  
⚠️ **Design Overhead**: Requires careful idempotent design  

## Adding New Migrations

1. Create a new `.sql` file with the next version number
2. Ensure all operations are idempotent using the patterns above
3. Test locally by running the app multiple times
4. Commit and deploy - the migration will run automatically

### Example Migration File (`005_add_user_profiles.sql`):
```sql
-- Add profile fields to users table (idempotent)
ALTER TABLE users ADD COLUMN IF NOT EXISTS bio TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS website TEXT;

-- Create user preferences table (idempotent)
CREATE TABLE IF NOT EXISTS user_preferences (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    email_notifications BOOLEAN DEFAULT true,
    dark_mode BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index for better performance (idempotent)
CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id ON user_preferences(user_id);

-- Insert default preferences for existing users (idempotent)
INSERT INTO user_preferences (id, user_id)
SELECT 
    'pref_' || u.id,
    u.id
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM user_preferences up WHERE up.user_id = u.id
)
ON CONFLICT DO NOTHING;
```

## Current Migrations

- `001_init.sql` - Initial database schema with users, products, categories, chains, etc.
- `002_add_product_fields.sql` - Added verification, analytics, and scoring fields to products
- `003_add_product_revisions.sql` - Added revision tracking system for products
- `004_fix_revision_tables.sql` - Fixed revision table ID generation for Railway deployment

## Testing Migrations

To test that your migration is truly idempotent:

1. Run the application locally
2. Check that your changes are applied
3. Run the application again (without changing code)
4. Verify no errors occur and changes are still there
5. Repeat step 3-4 several times to be sure

If any step fails, your migration is not idempotent and needs to be fixed. 