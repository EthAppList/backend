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

	// Get list of available migrations from directory
	migrations, err := r.loadMigrationsFromDirectory()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	if len(migrations) == 0 {
		log.Println("No migration files found")
		return nil
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
			filename TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.Exec(query)
	return err
}

// loadMigrationsFromDirectory scans for migration files and loads them
func (r *PostgresRepository) loadMigrationsFromDirectory() ([]Migration, error) {
	var migrations []Migration

	// Try both local and Docker paths
	migrationPaths := []string{"migrations", "/root/migrations"}

	for _, basePath := range migrationPaths {
		files, err := ioutil.ReadDir(basePath)
		if err != nil {
			continue // Try next path
		}

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
		}

		// If we found files in this path, don't try other paths
		if len(migrations) > 0 {
			break
		}
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

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

	// Record migration as applied
	_, err = tx.Exec(
		"INSERT INTO schema_migrations (version, name, filename) VALUES ($1, $2, $3)",
		migration.Version, migration.Name, migration.Filename,
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

// parseSQL properly parses SQL text into individual executable statements
// Handles dollar-quoted strings, functions, and complex SQL structures
func (r *PostgresRepository) parseSQL(sql string) []string {
	var statements []string
	var currentStatement strings.Builder
	var inDollarQuote bool
	var dollarQuoteTag string
	var parenthesesDepth int

	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and pure comment lines
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "--") {
			continue
		}

		// Handle dollar quoting for functions
		if strings.Contains(line, "$$") {
			parts := strings.Split(line, "$$")
			for i, part := range parts {
				if i%2 == 1 { // We're inside dollar quotes
					if !inDollarQuote {
						// Starting dollar quote
						dollarQuoteTag = part
						inDollarQuote = true
					} else if part == dollarQuoteTag {
						// Ending dollar quote with matching tag
						inDollarQuote = false
						dollarQuoteTag = ""
					}
				}
			}
		}

		// Track parentheses depth for complex statements
		if !inDollarQuote {
			parenthesesDepth += strings.Count(line, "(") - strings.Count(line, ")")
		}

		// Add line to current statement
		currentStatement.WriteString(line + "\n")

		// Check if statement ends
		shouldEndStatement := false
		if !inDollarQuote && parenthesesDepth <= 0 {
			// Look for statement terminators
			if strings.HasSuffix(trimmedLine, ";") {
				// Special handling for function definitions and complex statements
				currentStmt := currentStatement.String()
				if !r.isIncompleteStatement(currentStmt) {
					shouldEndStatement = true
				}
			}
		}

		if shouldEndStatement {
			stmt := strings.TrimSpace(currentStatement.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStatement.Reset()
			parenthesesDepth = 0
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

// isIncompleteStatement checks if a statement appears to be incomplete
func (r *PostgresRepository) isIncompleteStatement(stmt string) bool {
	stmt = strings.TrimSpace(stmt)

	// Check for incomplete CREATE statements
	if strings.HasPrefix(strings.ToUpper(stmt), "CREATE") {
		// If it's a CREATE statement, make sure it's not just the beginning
		lines := strings.Split(stmt, "\n")
		if len(lines) < 3 {
			return true
		}
	}

	// Check for unmatched dollar quotes
	dollarCount := strings.Count(stmt, "$$")
	if dollarCount%2 != 0 {
		return true
	}

	return false
}
