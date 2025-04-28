package repository

import (
	"errors"
)

// This file is kept for backward compatibility
// All database operations are now handled by the PostgreSQL repository
// See postgres_repository.go for the actual implementation

// Repository is deprecated - use PostgresRepository instead
type Repository struct {
	// Intentionally empty
}

// New is deprecated - use NewPostgres instead
func New() (*Repository, error) {
	return nil, errors.New("Supabase repository is no longer supported, please use PostgreSQL repository")
}
