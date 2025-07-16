# Database Migrations

This directory contains database migration files that are automatically executed when the application starts.

## How It Works

The migration system:
1. Scans this directory for `.sql` files on app startup
2. Tracks which migrations have been applied in the `schema_migrations` table
3. Only runs pending migrations (safe to restart the app)
4. Executes migrations in version order

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
- `004_add_user_profiles.sql` - Add user profile features

## Adding New Migrations

1. Create a new `.sql` file with the next version number
2. Write your SQL statements (can include multiple statements)
3. Commit and deploy - the migration will run automatically

### Example Migration File (`004_add_user_profiles.sql`):
```sql
-- Add profile fields to users table
ALTER TABLE users ADD COLUMN bio TEXT;
ALTER TABLE users ADD COLUMN avatar_url TEXT;
ALTER TABLE users ADD COLUMN website TEXT;

-- Create user preferences table
CREATE TABLE user_preferences (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    email_notifications BOOLEAN DEFAULT true,
    dark_mode BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index for better performance
CREATE INDEX idx_user_preferences_user_id ON user_preferences(user_id);
```

## Current Migrations

- `001_init.sql` - Initial database schema with users, products, categories, chains, etc.
- `002_add_product_fields.sql` - Added verification, analytics, and scoring fields to products
- `003_add_product_revisions.sql` - Added revision tracking system for products

## Important Notes

- Migrations are run in a transaction - if any part fails, the entire migration is rolled back
- Never modify existing migration files after they've been deployed
- Always create new migration files for schema changes
- Test your migrations locally before deploying
- The system handles existing schemas gracefully - only new migrations will be applied 