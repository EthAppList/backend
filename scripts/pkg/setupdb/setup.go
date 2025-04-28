package setupdb

import (
	"log"

	"github.com/wesjorgensen/EthAppList/backend/scripts/db"
)

// Initialize sets up the database
func Initialize() error {
	if err := db.SetupPostgres(); err != nil {
		log.Printf("Failed to set up database: %v", err)
		return err
	}
	return nil
}

// RunSetup provides a callable entry point for the setup
func RunSetup() {
	if err := Initialize(); err != nil {
		log.Fatalf("Database setup failed: %v", err)
	}
	log.Println("Database setup completed successfully")
}
