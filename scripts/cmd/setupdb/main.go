package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/nedpals/supabase-go"
	"github.com/wesjorgensen/EthAppList/backend/scripts/pkg/utils"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Get Supabase credentials
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		log.Fatalf("Supabase credentials are required. Please set SUPABASE_URL and SUPABASE_KEY environment variables.")
	}

	// Initialize Supabase client
	client := supabase.CreateClient(supabaseURL, supabaseKey)
	if client == nil {
		log.Fatalf("Failed to create Supabase client")
	}

	// Find the project root to locate the migrations directory
	projectRoot, err := utils.FindProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}

	// Read the SQL migration file
	sqlBytes, err := ioutil.ReadFile(filepath.Join(projectRoot, "migrations", "init.sql"))
	if err != nil {
		log.Fatalf("Failed to read migration file: %v", err)
	}

	sqlQuery := string(sqlBytes)

	// Supabase Go client doesn't have a direct SQL execution method
	// You would typically use the Supabase REST API or Postgres connection
	// For this example, we'll log the instructions
	fmt.Println("===== Database Setup =====")
	fmt.Println("To set up your Supabase database:")
	fmt.Println("1. Log in to your Supabase dashboard")
	fmt.Println("2. Go to SQL Editor")
	fmt.Println("3. Create a new query")
	fmt.Println("4. Paste the following SQL and run it:")
	fmt.Println("\n" + sqlQuery)
	fmt.Println("\nAlternatively, you can use the Supabase CLI or REST API to execute this SQL.")
	fmt.Println("=============================")
}
