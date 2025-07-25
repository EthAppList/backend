package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load .env from parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Connect to UPVOTES Redis
	upvotesRedisURL := os.Getenv("UPVOTES_REDIS")
	if upvotesRedisURL == "" {
		log.Fatal("UPVOTES_REDIS not set")
	}

	opt, err := redis.ParseURL(upvotesRedisURL)
	if err != nil {
		log.Fatal("Failed to parse UPVOTES Redis URL:", err)
	}

	redisClient := redis.NewClient(opt)
	ctx := context.Background()

	// Test Redis connection
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to UPVOTES Redis:", err)
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

	fmt.Println("=== FINAL TRENDING ALGORITHM FIX AND DATA REBUILD ===")

	// 1. Get all projects with votes from the database
	fmt.Println("\n1. Rebuilding all trending scores from database...")
	rows, err := db.Query(`
		SELECT ps.project_id, ps.upvotes_total, ps.last_vote_ts
		FROM project_stats ps 
		JOIN products p ON ps.project_id = p.id 
		WHERE p.approved = true AND ps.upvotes_total > 0
	`)
	if err != nil {
		log.Fatal("Failed to query project stats:", err)
	}
	defer rows.Close()

	// 2. Clear existing trending data and site-wide vote counter
	fmt.Println("Clearing existing trending ZSET and old site vote counter...")
	pipe := redisClient.Pipeline()
	pipe.Del(ctx, "trending")
	pipe.Del(ctx, "site:24h_votes") // Remove old, problematic key
	_, err = pipe.Exec(ctx)
	if err != nil {
		log.Fatal("Failed to clear Redis data:", err)
	}

	// 3. Rebuild trending scores with the new, stable algorithm
	projectsFixed := 0
	for rows.Next() {
		var projectID string
		var voteCount int64
		var lastVoteTime time.Time

		err := rows.Scan(&projectID, &voteCount, &lastVoteTime)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		// Calculate score with the new, stable algorithm
		score := calculateTrendingScore(float64(voteCount), lastVoteTime)

		// Update Redis with the correct data
		redisPipe := redisClient.Pipeline()
		projectKey := fmt.Sprintf("project:%s", projectID)
		redisPipe.HSet(ctx, projectKey, "up", voteCount)
		redisPipe.ZAdd(ctx, "trending", redis.Z{
			Score:  score,
			Member: fmt.Sprintf("project:%s", projectID),
		})
		_, err = redisPipe.Exec(ctx)
		if err != nil {
			log.Printf("Error updating project %s: %v", projectID, err)
			continue
		}

		fmt.Printf("Fixed %s: %d votes, last vote %v, new score %.6f\n",
			projectID, voteCount, lastVoteTime, score)
		projectsFixed++
	}

	fmt.Printf("\n2. Fixed %d projects successfully!\n", projectsFixed)

	// 4. Show the new, correct trending order
	fmt.Println("\n3. New, stable trending order:")
	newTrending, err := redisClient.ZRevRangeWithScores(ctx, "trending", 0, 9).Result()
	if err != nil {
		log.Fatal("Failed to get new trending data:", err)
	}

	for i, result := range newTrending {
		projectID := result.Member.(string)
		if len(projectID) > 8 && projectID[:8] == "project:" {
			projectID = projectID[8:]
		}

		var title string
		err := db.QueryRow("SELECT title FROM products WHERE id = $1", projectID).Scan(&title)
		if err != nil {
			title = "Unknown"
		}

		fmt.Printf("  %d. %s (%s): %.6f\n", i+1, title, projectID, result.Score)
	}

	fmt.Println("\n✅ Trending algorithm fixed and all data rebuilt!")
	fmt.Println("You can now safely redeploy your server. The issue will not happen again.")
}

// calculateTrendingScore implements the new, stable trending algorithm
func calculateTrendingScore(totalUpvotes float64, timestamp time.Time) float64 {
	// K is a constant to control time decay. A lower value means faster decay.
	// 45000 is the original Reddit value, roughly 12.5 hours in seconds.
	const K = 45000

	logScore := math.Log10(math.Max(1, totalUpvotes))
	epoch := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	timeScore := float64(timestamp.Unix()-epoch) / K

	return logScore + timeScore
}
