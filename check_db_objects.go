package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/wesjorgensen/EthAppList/backend/internal/config"
	"github.com/wesjorgensen/EthAppList/backend/internal/repository"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Initialize configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}

	// Initialize PostgreSQL repository
	log.Println("Connecting to PostgreSQL...")
	pgRepo, err := repository.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL repository: %v", err)
	}
	defer pgRepo.Close()

	db := pgRepo.GetDB()

	// Check for views that reference upvotes
	fmt.Println("Checking for views that reference 'upvotes'...")
	rows, err := db.Query(`
		SELECT table_name, view_definition 
		FROM information_schema.views 
		WHERE view_definition LIKE '%upvotes%' 
		AND table_schema = 'public'
	`)
	if err != nil {
		log.Printf("Failed to check views: %v", err)
	} else {
		defer rows.Close()
		viewCount := 0
		for rows.Next() {
			var name, definition string
			if err := rows.Scan(&name, &definition); err != nil {
				log.Printf("Error scanning view: %v", err)
				continue
			}
			fmt.Printf("View '%s' contains 'upvotes'\n", name)
			viewCount++
		}
		if viewCount == 0 {
			fmt.Println("No views found that reference 'upvotes'")
		}
	}

	// Check for triggers that reference upvotes
	fmt.Println("Checking for triggers that reference 'upvotes'...")
	rows, err = db.Query(`
		SELECT trigger_name, action_statement 
		FROM information_schema.triggers 
		WHERE action_statement LIKE '%upvotes%' 
		AND trigger_schema = 'public'
	`)
	if err != nil {
		log.Printf("Failed to check triggers: %v", err)
	} else {
		defer rows.Close()
		triggerCount := 0
		for rows.Next() {
			var name, statement string
			if err := rows.Scan(&name, &statement); err != nil {
				log.Printf("Error scanning trigger: %v", err)
				continue
			}
			fmt.Printf("Trigger '%s' contains 'upvotes'\n", name)
			triggerCount++
		}
		if triggerCount == 0 {
			fmt.Println("No triggers found that reference 'upvotes'")
		}
	}

	// Check for any table that still exists named upvotes
	fmt.Println("Checking if upvotes table still exists...")
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = 'upvotes'
		)
	`).Scan(&exists)
	if err != nil {
		log.Printf("Error checking table existence: %v", err)
	} else {
		fmt.Printf("Table 'upvotes' exists: %v\n", exists)
	}

	// Try to directly query what the getProductUpvoteCount function would do
	fmt.Println("Testing direct project_stats query...")
	var count int
	err = db.QueryRow("SELECT upvotes_total FROM project_stats WHERE project_id = 'test123'").Scan(&count)
	if err != nil {
		fmt.Printf("Expected error for nonexistent product: %v\n", err)
	} else {
		fmt.Printf("Unexpected result: %d\n", count)
	}

	// Check what the specific error would be if we try to query upvotes
	fmt.Println("Testing query to non-existent upvotes table...")
	err = db.QueryRow("SELECT COUNT(*) FROM upvotes").Scan(&count)
	if err != nil {
		fmt.Printf("Error (as expected): %v\n", err)
	} else {
		fmt.Printf("Unexpected success: %d\n", count)
	}
}
