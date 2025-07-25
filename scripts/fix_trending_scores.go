package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"math"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load .env from parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
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

	fmt.Println("=== FIXING TRENDING SCORES ===")

	// 1. Show current state
	fmt.Println("\n1. Current trending state:")
	currentTrending, err := redisClient.ZRevRangeWithScores(ctx, "trending", 0, -1).Result()
	if err != nil {
		log.Fatal("Failed to get current trending:", err)
	}
	fmt.Printf("Found %d projects in trending ZSET\n", len(currentTrending))

	// 2. Get site votes for calculation
	siteVotesStr, err := redisClient.Get(ctx, "site:24h_votes").Result()
	if err == redis.Nil {
		siteVotesStr = "1"
		fmt.Println("No site votes found, using default value of 1")
	} else if err != nil {
		log.Fatal("Failed to get site votes:", err)
	}
	siteVotes, _ := strconv.ParseFloat(siteVotesStr, 64)
	fmt.Printf("Using site votes: %.0f\n", siteVotes)

	// 3. Get all projects with votes from database
	fmt.Println("\n2. Rebuilding trending scores from database...")
	rows, err := db.Query(`
		SELECT ps.project_id, ps.upvotes_total, ps.last_vote_ts
		FROM project_stats ps 
		JOIN products p ON ps.project_id = p.id 
		WHERE p.approved = true AND ps.upvotes_total > 0
		ORDER BY ps.upvotes_total DESC, ps.last_vote_ts DESC
	`)
	if err != nil {
		log.Fatal("Failed to query project stats:", err)
	}
	defer rows.Close()

	// 4. Clear existing trending data
	fmt.Println("Clearing existing trending data...")
	err = redisClient.Del(ctx, "trending").Err()
	if err != nil {
		log.Fatal("Failed to clear trending data:", err)
	}

	// 5. Rebuild trending scores with correct timestamps
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

		// Calculate correct trending score using actual vote timestamp
		score := calculateTrendingScore(float64(voteCount), lastVoteTime, siteVotes)

		// Update Redis with correct data
		pipe := redisClient.Pipeline()

		// Set vote count in Redis
		projectKey := fmt.Sprintf("project:%s", projectID)
		pipe.HSet(ctx, projectKey, "up", voteCount)

		// Add to trending with correct score
		pipe.ZAdd(ctx, "trending", redis.Z{
			Score:  score,
			Member: fmt.Sprintf("project:%s", projectID),
		})

		_, err = pipe.Exec(ctx)
		if err != nil {
			log.Printf("Error updating project %s: %v", projectID, err)
			continue
		}

		fmt.Printf("Fixed %s: %d votes, last vote %v, score %.6f\n",
			projectID, voteCount, lastVoteTime, score)
		projectsFixed++
	}

	fmt.Printf("\n3. Fixed %d projects successfully!\n", projectsFixed)

	// 6. Show new trending order
	fmt.Println("\n4. New trending order:")
	newTrending, err := redisClient.ZRevRangeWithScores(ctx, "trending", 0, 9).Result()
	if err != nil {
		log.Fatal("Failed to get new trending:", err)
	}

	for i, result := range newTrending {
		projectID := result.Member.(string)
		if len(projectID) > 8 && projectID[:8] == "project:" {
			projectID = projectID[8:]
		}

		// Get project title from database
		var title string
		err := db.QueryRow("SELECT title FROM products WHERE id = $1", projectID).Scan(&title)
		if err != nil {
			title = "Unknown"
		}

		fmt.Printf("  %d. %s (%s): %.6f\n", i+1, projectID, title, result.Score)
	}

	fmt.Println("\n5. Trending scores have been fixed! ✅")
	fmt.Println("The algorithm now correctly uses vote timestamps instead of processing time.")
}

// calculateTrendingScore implements the same algorithm as the Go code
func calculateTrendingScore(totalUpvotes float64, timestamp time.Time, siteVotesLast24h float64) float64 {
	logScore := math.Log10(math.Max(1, totalUpvotes))
	epoch := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	timeScore := float64(timestamp.Unix()-epoch) / (45000 * math.Sqrt(siteVotesLast24h+1))
	return logScore + timeScore
}
