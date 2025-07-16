package repository

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

// Migration represents a database migration
type Migration struct {
	Version  int
	Name     string
	Filename string
	SQL      string
}

// RunMigrations executes all pending database migrations
func (r *PostgresRepository) RunMigrations() error {
	log.Println("Starting database migrations...")

	// Create migrations table if it doesn't exist
	err := r.createMigrationsTable()
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of available migrations
	migrations, err := r.loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Get applied migrations
	appliedMigrations, err := r.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Filter out already applied migrations
	pendingMigrations := r.filterPendingMigrations(migrations, appliedMigrations)

	if len(pendingMigrations) == 0 {
		log.Println("No pending migrations")
		return nil
	}

	// Execute pending migrations
	for _, migration := range pendingMigrations {
		log.Printf("Running migration: %s", migration.Name)
		err := r.executeMigration(migration)
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
		}
		log.Printf("Successfully applied migration: %s", migration.Name)
	}

	log.Printf("Successfully applied %d migrations", len(pendingMigrations))
	return nil
}

// createMigrationsTable creates the migrations tracking table
func (r *PostgresRepository) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.Exec(query)
	return err
}

// loadMigrations loads all migration files from the migrations directory
func (r *PostgresRepository) loadMigrations() ([]Migration, error) {
	var migrations []Migration

	// Define migration files in order
	migrationFiles := []struct {
		version  int
		name     string
		filename string
	}{
		{1, "init", "init.sql"},
		{2, "add_product_fields", "add_product_fields.sql"},
		{3, "add_product_revisions", "add_product_revisions.sql"},
	}

	for _, mf := range migrationFiles {
		var content []byte
		var err error

		// Try to read from local migrations directory first
		content, err = ioutil.ReadFile(filepath.Join("migrations", mf.filename))
		if err != nil {
			// Try Docker path if local path fails
			content, err = ioutil.ReadFile(filepath.Join("/root/migrations", mf.filename))
			if err != nil {
				// If file doesn't exist in either location, skip it (for optional migrations)
				log.Printf("Migration file %s not found in local or Docker path, skipping", mf.filename)
				continue
			}
		}

		migration := Migration{
			Version:  mf.version,
			Name:     mf.name,
			Filename: mf.filename,
			SQL:      string(content),
		}
		migrations = append(migrations, migration)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
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

// executeMigration runs a single migration
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

	// Execute migration SQL
	// Split SQL by semicolons to handle multiple statements
	statements := r.splitSQL(migration.SQL)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err = tx.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to execute SQL statement: %w", err)
		}
	}

	// Record migration as applied
	_, err = tx.Exec(
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2)",
		migration.Version, migration.Name,
	)
	if err != nil {
		return err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// splitSQL splits SQL text into individual statements
func (r *PostgresRepository) splitSQL(sql string) []string {
	var statements []string
	var currentStatement strings.Builder
	var inFunction bool
	var dollarQuoteTag string

	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip comments and empty lines
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "--") {
			continue
		}

		// Check for dollar quoting (for functions and complex strings)
		if strings.Contains(trimmedLine, "$$") {
			// Extract dollar quote tag
			parts := strings.Split(trimmedLine, "$$")
			if len(parts) >= 2 {
				if dollarQuoteTag == "" {
					// Starting dollar quote
					dollarQuoteTag = parts[0] + "$$" + parts[1] + "$$"
					inFunction = true
				} else if strings.Contains(line, dollarQuoteTag) {
					// Ending dollar quote
					dollarQuoteTag = ""
					inFunction = false
				}
			}
		}

		// Add line to current statement
		currentStatement.WriteString(line + "\n")

		// Check if statement ends (semicolon at end of line, not in function)
		if !inFunction && strings.HasSuffix(trimmedLine, ";") {
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
