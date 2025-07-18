package repository

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// IDEMPOTENT MIGRATION SYSTEM
//
// This system runs ALL migrations every time the application starts.
// All migration files are designed to be idempotent (safe to run multiple times).
//
// Key principles:
// - Use CREATE TABLE IF NOT EXISTS
// - Use ALTER TABLE ... ADD COLUMN IF NOT EXISTS
// - Use CREATE INDEX IF NOT EXISTS
// - Use INSERT ... ON CONFLICT DO NOTHING
// - Use CREATE OR REPLACE for functions
// - Use DROP ... IF EXISTS before CREATE for policies/triggers
//
// Benefits:
// - Easy to modify existing migrations during development
// - No need to track migration state
// - Simpler deployment process
// - Self-healing (fixes any missing tables/columns)
//
// Trade-offs:
// - Slightly slower startup (runs all migrations each time)
// - Requires careful design to ensure true idempotency

// Migration represents a database migration
type Migration struct {
	Version  int
	Name     string
	Filename string
	SQL      string
}

// RunMigrations executes all database migrations every time (idempotent approach)
func (r *PostgresRepository) RunMigrations() error {
	log.Println("Starting database migrations (running all migrations)...")

	// Get list of available migrations from directory
	migrations, err := r.loadMigrationsFromDirectory()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	if len(migrations) == 0 {
		log.Println("No migration files found")
		return nil
	}

	log.Printf("Found %d migration files", len(migrations))

	// Execute all migrations (they are designed to be idempotent)
	for _, migration := range migrations {
		log.Printf("Running migration: %s (version %d)", migration.Name, migration.Version)
		err := r.executeMigration(migration)
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
		}
		log.Printf("Successfully executed migration: %s", migration.Name)
	}

	log.Printf("Successfully executed all %d migrations", len(migrations))
	return nil
}

// createMigrationsTable creates the migrations tracking table
func (r *PostgresRepository) createMigrationsTable() error {
	// First create the basic table
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.Exec(query)
	if err != nil {
		return err
	}

	// Add filename column if it doesn't exist
	alterQuery := `
		ALTER TABLE schema_migrations 
		ADD COLUMN IF NOT EXISTS filename TEXT
	`
	_, err = r.db.Exec(alterQuery)
	return err
}

// loadMigrationsFromDirectory scans for migration files and loads them
func (r *PostgresRepository) loadMigrationsFromDirectory() ([]Migration, error) {
	var migrations []Migration

	// Try multiple paths for Railway deployment
	migrationPaths := []string{
		"migrations",              // Local development
		"/root/migrations",        // Docker container
		"./migrations",            // Current directory
		"/app/migrations",         // Alternative container path
		"/opt/railway/migrations", // Railway specific path
	}

	var foundPath string
	for _, basePath := range migrationPaths {
		files, err := ioutil.ReadDir(basePath)
		if err != nil {
			log.Printf("Migration path not found: %s", basePath)
			continue // Try next path
		}

		foundPath = basePath
		log.Printf("Found migration directory: %s", basePath)

		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".sql") {
				continue
			}

			// Extract version from filename (e.g., "001_init.sql" -> version 1)
			version, name := r.parseFilename(file.Name())
			if version == 0 {
				log.Printf("Skipping file with invalid version: %s", file.Name())
				continue
			}

			// Read file content
			content, err := ioutil.ReadFile(filepath.Join(basePath, file.Name()))
			if err != nil {
				log.Printf("Failed to read migration file %s: %v", file.Name(), err)
				continue
			}

			migration := Migration{
				Version:  version,
				Name:     name,
				Filename: file.Name(),
				SQL:      string(content),
			}
			migrations = append(migrations, migration)
			log.Printf("Loaded migration: %s (version %d)", file.Name(), version)
		}

		// If we found files in this path, don't try other paths
		if len(migrations) > 0 {
			break
		}
	}

	if foundPath == "" {
		return nil, fmt.Errorf("no migration directory found in any of the expected paths: %v", migrationPaths)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	log.Printf("Loaded %d migrations from %s", len(migrations), foundPath)
	return migrations, nil
}

// parseFilename extracts version and name from migration filename
// Expected format: "001_init.sql", "002_add_product_fields.sql", etc.
func (r *PostgresRepository) parseFilename(filename string) (int, string) {
	// Remove .sql extension
	nameWithoutExt := strings.TrimSuffix(filename, ".sql")

	// Match pattern like "001_init" or "002_add_product_fields"
	re := regexp.MustCompile(`^(\d+)_(.+)$`)
	matches := re.FindStringSubmatch(nameWithoutExt)

	if len(matches) != 3 {
		return 0, ""
	}

	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, ""
	}

	return version, matches[2]
}

// getAppliedMigrations returns a list of applied migration versions
func (r *PostgresRepository) getAppliedMigrations() ([]int, error) {
	query := "SELECT version FROM schema_migrations ORDER BY version"
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		err := rows.Scan(&version)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, nil
}

// filterPendingMigrations returns migrations that haven't been applied yet
func (r *PostgresRepository) filterPendingMigrations(allMigrations []Migration, appliedVersions []int) []Migration {
	appliedMap := make(map[int]bool)
	for _, version := range appliedVersions {
		appliedMap[version] = true
	}

	var pending []Migration
	for _, migration := range allMigrations {
		if !appliedMap[migration.Version] {
			pending = append(pending, migration)
		}
	}

	return pending
}

// executeMigration runs a single migration (idempotent)
func (r *PostgresRepository) executeMigration(migration Migration) error {
	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Parse SQL into individual statements
	statements := r.parseSQL(migration.SQL)

	// Execute each statement
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		log.Printf("Executing statement %d/%d from %s", i+1, len(statements), migration.Filename)
		_, err = tx.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute statement %d in migration %s: %w\nStatement: %s", i+1, migration.Name, err, stmt)
		}
	}

	// Commit transaction (no need to record migration since all migrations run every time)
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// parseSQL properly parses SQL text into individual executable statements
// Simplified approach that handles dollar-quoted functions correctly
func (r *PostgresRepository) parseSQL(sql string) []string {
	var statements []string
	var currentStatement strings.Builder
	var inDollarQuote bool

	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and pure comment lines
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "--") {
			continue
		}

		// Simple dollar quote detection
		if strings.Contains(line, "$$") {
			// Count number of $$ in the line
			count := strings.Count(line, "$$")
			if count%2 == 1 {
				// Odd number means we're entering or leaving dollar quotes
				inDollarQuote = !inDollarQuote
			}
		}

		// Add line to current statement
		currentStatement.WriteString(line + "\n")

		// Check if statement ends (only when not in dollar quotes)
		if !inDollarQuote && strings.HasSuffix(trimmedLine, ";") {
			stmt := strings.TrimSpace(currentStatement.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStatement.Reset()
		}
	}

	// Add any remaining statement
	if currentStatement.Len() > 0 {
		stmt := strings.TrimSpace(currentStatement.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}
