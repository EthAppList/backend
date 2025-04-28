package dockersetup

import (
	"log"

	"github.com/wesjorgensen/EthAppList/backend/scripts/db"
)

// Initialize sets up the database for Docker deployment
func Initialize() error {
	log.Println("Starting Docker database initialization...")
	if err := db.SetupPostgres(); err != nil {
		log.Printf("Failed to set up database: %v", err)
		return err
	}
	log.Println("Database setup completed successfully")
	return nil
}

// RunSetup provides a callable entry point for Docker setup
func RunSetup() {
	if err := Initialize(); err != nil {
		log.Fatalf("Docker database setup failed: %v", err)
	}
}
