package main

import (
	"fmt"

	"github.com/nedpals/supabase-go"
)

func main() {
	// Create a client
	client := supabase.CreateClient("https://your-project.supabase.co", "your-api-key")

	// Print the client type to see available methods
	fmt.Printf("Client type: %T\n", client)

	// Check if DB field exists and its type
	fmt.Printf("DB type: %T\n", client.DB)

	// Test query building
	query := client.DB.From("users")
	fmt.Printf("Query type: %T\n", query)

	// Test select method
	selectQuery := query.Select("*")
	fmt.Printf("Select query type: %T\n", selectQuery)

	// Check available methods on the query
	fmt.Println("Methods that should be available:")
	fmt.Println("- Select(columns string) - Used for selecting columns")
	fmt.Println("- Eq(column string, value interface{}) - Used for equality filters")
	fmt.Println("- Order(column string, ascending bool) - Used for sorting")
	fmt.Println("- Range(from, to int) - Used for pagination")
	fmt.Println("- Execute(result interface{}) - Used to execute the query")
}
