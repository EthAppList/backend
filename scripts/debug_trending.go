package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load .env from parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Connect to UPVOTES Redis (not the general Redis)
	upvotesRedisURL := os.Getenv("UPVOTES_REDIS")
	if upvotesRedisURL == "" {
		log.Fatal("UPVOTES_REDIS not set")
	}

	fmt.Printf("Connecting to UPVOTES Redis: %s\n", upvotesRedisURL)

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

	fmt.Println("=== UPVOTES REDIS TRENDING ALGORITHM DEBUG ===")

	// 1. Check current trending projects in Redis
	fmt.Println("\n1. Current trending projects in UPVOTES Redis:")
	trendingResults, err := redisClient.ZRevRangeWithScores(ctx, "trending", 0, 9).Result()
	if err != nil {
		log.Fatal("Failed to get trending projects:", err)
	}

	fmt.Printf("Found %d projects in trending ZSET:\n", len(trendingResults))
	for i, result := range trendingResults {
		projectID := result.Member.(string)
		if len(projectID) > 8 && projectID[:8] == "project:" {
			projectID = projectID[8:]
		}
		fmt.Printf("  %d. Project: %s, Score: %.6f\n", i+1, projectID, result.Score)
	}

	// 2. Check site-wide vote count
	fmt.Println("\n2. Site-wide 24h vote count:")
	siteVotes, err := redisClient.Get(ctx, "site:24h_votes").Result()
	if err == redis.Nil {
		fmt.Println("  No site votes key found (will default to 1)")
		siteVotes = "1"
	} else if err != nil {
		log.Fatal("Failed to get site votes:", err)
	} else {
		fmt.Printf("  24h votes: %s\n", siteVotes)
	}

	// 3. Check individual project data
	fmt.Println("\n3. Individual project vote data:")
	for i, result := range trendingResults {
		if i >= 5 { // Only check top 5
			break
		}

		projectID := result.Member.(string)
		if len(projectID) > 8 && projectID[:8] == "project:" {
			projectID = projectID[8:]
		}

		// Get Redis data
		projectKey := fmt.Sprintf("project:%s", projectID)
		voteCount, err := redisClient.HGet(ctx, projectKey, "up").Result()
		if err == redis.Nil {
			voteCount = "0"
		} else if err != nil {
			log.Printf("Error getting vote count for %s: %v", projectID, err)
			continue
		}

		// Get database data
		var dbVoteCount int64
		var title string
		var lastVoteTime *time.Time
		err = db.QueryRow(`
			SELECT p.title, COALESCE(ps.upvotes_total, 0), ps.last_vote_ts
			FROM products p 
			LEFT JOIN project_stats ps ON p.id = ps.project_id 
			WHERE p.id = $1
		`, projectID).Scan(&title, &dbVoteCount, &lastVoteTime)

		if err != nil {
			log.Printf("Error getting DB data for %s: %v", projectID, err)
			continue
		}

		fmt.Printf("  Project: %s (%s)\n", projectID, title)
		fmt.Printf("    Redis vote count: %s\n", voteCount)
		fmt.Printf("    DB vote count: %d\n", dbVoteCount)
		if lastVoteTime != nil {
			fmt.Printf("    Last vote time: %v\n", *lastVoteTime)
		} else {
			fmt.Printf("    Last vote time: <none>\n")
		}
		fmt.Printf("    Current trending score: %.6f\n", result.Score)

		// Calculate what the score should be
		siteVotesFloat, _ := strconv.ParseFloat(siteVotes, 64)
		voteCountFloat, _ := strconv.ParseFloat(voteCount, 64)

		var timestamp time.Time
		if lastVoteTime != nil {
			timestamp = *lastVoteTime
		} else {
			timestamp = time.Now()
		}

		expectedScore := calculateTrendingScore(voteCountFloat, timestamp, siteVotesFloat)
		fmt.Printf("    Expected score: %.6f\n", expectedScore)

		if math.Abs(result.Score-expectedScore) > 0.000001 {
			fmt.Printf("    ⚠️  SCORE MISMATCH! Difference: %.6f\n", result.Score-expectedScore)
		}
		fmt.Println()
	}

	// 4. Test score calculation with example scenarios
	fmt.Println("\n4. Testing score calculation scenarios:")
	testScenarios := []struct {
		name      string
		upvotes   float64
		timestamp time.Time
		siteVotes float64
	}{
		{"New project with 1 vote", 1, time.Now(), 10},
		{"New project with 2 votes", 2, time.Now(), 10},
		{"Older project with 5 votes", 5, time.Now().Add(-24 * time.Hour), 10},
		{"Very new project with 2 votes", 2, time.Now(), 1},
	}

	for _, scenario := range testScenarios {
		score := calculateTrendingScore(scenario.upvotes, scenario.timestamp, scenario.siteVotes)
		fmt.Printf("  %s: %.6f\n", scenario.name, score)
	}

	// 5. Check for data inconsistencies
	fmt.Println("\n5. Checking for data inconsistencies:")

	// Get all projects from trending ZSET
	allTrending, err := redisClient.ZRevRange(ctx, "trending", 0, -1).Result()
	if err != nil {
		log.Fatal("Failed to get all trending projects:", err)
	}

	fmt.Printf("Total projects in trending ZSET: %d\n", len(allTrending))

	// Check if any projects have 0 votes but are still in trending
	zeroVoteProjects := 0
	for _, member := range allTrending {
		projectID := member
		if len(projectID) > 8 && projectID[:8] == "project:" {
			projectID = projectID[8:]
		}

		projectKey := fmt.Sprintf("project:%s", projectID)
		voteCount, err := redisClient.HGet(ctx, projectKey, "up").Result()
		if err == redis.Nil || voteCount == "0" {
			zeroVoteProjects++
		}
	}

	fmt.Printf("Projects with 0 votes in trending: %d\n", zeroVoteProjects)

	// 6. Simulate a vote to see what happens
	fmt.Println("\n6. Simulating vote processing:")
	if len(trendingResults) > 0 {
		// Take the first project and simulate adding a vote
		projectID := trendingResults[0].Member.(string)
		if len(projectID) > 8 && projectID[:8] == "project:" {
			projectID = projectID[8:]
		}

		fmt.Printf("Simulating vote for project: %s\n", projectID)

		// Get current vote count
		projectKey := fmt.Sprintf("project:%s", projectID)
		currentVotes, err := redisClient.HGet(ctx, projectKey, "up").Result()
		if err == redis.Nil {
			currentVotes = "0"
		}

		currentVotesInt, _ := strconv.Atoi(currentVotes)
		newVotes := currentVotesInt + 1

		fmt.Printf("  Current votes: %s\n", currentVotes)
		fmt.Printf("  After vote: %d\n", newVotes)

		// Calculate new score
		siteVotesFloat, _ := strconv.ParseFloat(siteVotes, 64)
		newScore := calculateTrendingScore(float64(newVotes), time.Now(), siteVotesFloat)
		fmt.Printf("  New score would be: %.6f\n", newScore)
		fmt.Printf("  Current score: %.6f\n", trendingResults[0].Score)

		if newScore > trendingResults[0].Score {
			fmt.Printf("  ✅ Score would increase (good!)\n")
		} else {
			fmt.Printf("  ❌ Score would not increase (potential issue!)\n")
		}
	}

	// 7. Check Redis streams and workers
	fmt.Println("\n7. Checking Redis vote processing infrastructure:")

	// Check if vote stream exists
	streamInfo, err := redisClient.XInfoStream(ctx, "votes").Result()
	if err == redis.Nil {
		fmt.Println("  No votes stream found")
	} else if err != nil {
		fmt.Printf("  Error checking votes stream: %v\n", err)
	} else {
		fmt.Printf("  Votes stream length: %d\n", streamInfo.Length)
		if streamInfo.FirstEntry.ID != "" {
			fmt.Printf("  First entry ID: %s\n", streamInfo.FirstEntry.ID)
		}
		if streamInfo.LastEntry.ID != "" {
			fmt.Printf("  Last entry ID: %s\n", streamInfo.LastEntry.ID)
		}
	}

	// Check consumer groups
	groups, err := redisClient.XInfoGroups(ctx, "votes").Result()
	if err == redis.Nil {
		fmt.Println("  No consumer groups found")
	} else if err != nil {
		fmt.Printf("  Error checking consumer groups: %v\n", err)
	} else {
		fmt.Printf("  Found %d consumer groups:\n", len(groups))
		for _, group := range groups {
			fmt.Printf("    Group: %s, Consumers: %d, Pending: %d\n",
				group.Name, group.Consumers, group.Pending)
		}
	}

	// 8. Check specific projects that should be in trending
	fmt.Println("\n8. Checking specific projects from database:")
	rows, err := db.Query(`
		SELECT ps.project_id, p.title, ps.upvotes_total 
		FROM project_stats ps 
		JOIN products p ON ps.project_id = p.id 
		WHERE ps.upvotes_total > 0 
		ORDER BY ps.upvotes_total DESC 
		LIMIT 5
	`)
	if err != nil {
		log.Printf("Error querying top voted projects: %v", err)
	} else {
		defer rows.Close()
		fmt.Println("Top voted projects from database:")
		for rows.Next() {
			var projectID, title string
			var votes int64
			rows.Scan(&projectID, &title, &votes)

			// Check if this project exists in Redis trending
			projectKey := fmt.Sprintf("project:%s", projectID)
			redisVotes, err := redisClient.HGet(ctx, projectKey, "up").Result()
			if err == redis.Nil {
				redisVotes = "NOT_FOUND"
			}

			trendingScore, err := redisClient.ZScore(ctx, "trending", fmt.Sprintf("project:%s", projectID)).Result()
			trendingExists := err == nil

			fmt.Printf("  %s (%s): DB=%d votes, Redis=%s, Trending=%v",
				projectID, title, votes, redisVotes, trendingExists)
			if trendingExists {
				fmt.Printf(" (score: %.6f)", trendingScore)
			}
			fmt.Println()
		}
	}
}

// calculateTrendingScore implements the same algorithm as the Go code
func calculateTrendingScore(totalUpvotes float64, timestamp time.Time, siteVotesLast24h float64) float64 {
	logScore := math.Log10(math.Max(1, totalUpvotes))
	epoch := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	timeScore := float64(timestamp.Unix()-epoch) / (45000 * math.Sqrt(siteVotesLast24h+1))
	return logScore + timeScore
}
