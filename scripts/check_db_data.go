package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load .env from parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Connect to Postgres
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	fmt.Println("=== DATABASE VOTE DATA CHECK ===")

	// 1. Check votes table
	fmt.Println("\n1. Votes table data:")
	var voteCount int
	err = db.QueryRow("SELECT COUNT(*) FROM votes").Scan(&voteCount)
	if err != nil {
		log.Printf("Error checking votes table: %v", err)
	} else {
		fmt.Printf("Total votes in database: %d\n", voteCount)
	}

	if voteCount > 0 {
		fmt.Println("\nRecent votes:")
		rows, err := db.Query(`
			SELECT v.user_id, v.product_id, p.title, v.created_at 
			FROM votes v 
			JOIN products p ON v.product_id = p.id 
			ORDER BY v.created_at DESC 
			LIMIT 10
		`)
		if err != nil {
			log.Printf("Error getting recent votes: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var userID, productID, title string
				var createdAt string
				rows.Scan(&userID, &productID, &title, &createdAt)
				fmt.Printf("  %s -> %s (%s) at %s\n", userID, productID, title, createdAt)
			}
		}
	}

	// 2. Check project_stats table
	fmt.Println("\n2. Project stats data:")
	var statsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM project_stats").Scan(&statsCount)
	if err != nil {
		log.Printf("Error checking project_stats table: %v", err)
	} else {
		fmt.Printf("Total project stats: %d\n", statsCount)
	}

	if statsCount > 0 {
		fmt.Println("\nTop voted projects:")
		rows, err := db.Query(`
			SELECT ps.project_id, p.title, ps.upvotes_total, ps.last_vote_ts 
			FROM project_stats ps 
			JOIN products p ON ps.project_id = p.id 
			WHERE ps.upvotes_total > 0
			ORDER BY ps.upvotes_total DESC, ps.last_vote_ts DESC 
			LIMIT 10
		`)
		if err != nil {
			log.Printf("Error getting project stats: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var projectID, title string
				var upvotes int64
				var lastVote string
				rows.Scan(&projectID, &title, &upvotes, &lastVote)
				fmt.Printf("  %s (%s): %d votes, last: %s\n", projectID, title, upvotes, lastVote)
			}
		}
	}

	// 3. Check approved products
	fmt.Println("\n3. Approved products:")
	var approvedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM products WHERE approved = true").Scan(&approvedCount)
	if err != nil {
		log.Printf("Error checking approved products: %v", err)
	} else {
		fmt.Printf("Total approved products: %d\n", approvedCount)
	}

	// 4. Check if any approved products are missing from project_stats
	fmt.Println("\n4. Data consistency check:")
	var missingStats int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM products p 
		WHERE p.approved = true 
		AND NOT EXISTS (SELECT 1 FROM project_stats ps WHERE ps.project_id = p.id)
	`).Scan(&missingStats)
	if err != nil {
		log.Printf("Error checking missing stats: %v", err)
	} else {
		fmt.Printf("Approved products missing from project_stats: %d\n", missingStats)
		if missingStats > 0 {
			fmt.Println("  ⚠️  Some approved products are missing project_stats entries!")
		}
	}
}
