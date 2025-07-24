package upvotes

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// UpvotesRedisService handles high-volume upvotes with Redis streams, counters, and trending
type UpvotesRedisService struct {
	client *redis.Client
	ctx    context.Context
}

// VoteEvent represents a vote event in the stream
type VoteEvent struct {
	UserID    string `json:"u"`
	ProjectID string `json:"p"`
	Timestamp int64  `json:"t"`
}

// NewUpvotesRedisService creates a new upvotes Redis service
func NewUpvotesRedisService(redisURL string) (*UpvotesRedisService, error) {
	// Parse Redis URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse upvotes Redis URL: %w", err)
	}

	// Create Redis client
	client := redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to upvotes Redis: %w", err)
	}

	log.Printf("Successfully connected to Upvotes Redis at %s", redisURL)

	return &UpvotesRedisService{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close closes the Redis connection
func (u *UpvotesRedisService) Close() error {
	return u.client.Close()
}

// Redis key patterns from the architecture
const (
	ProjectHashKey     = "project:%s"      // project:{id} -> HASH with 'up' field
	TrendingZSetKey    = "trending"        // ZSET with score -> project:{id}
	UserVotesSetKey    = "user:%s:votes"   // user:{uid}:votes -> SET of project_ids
	SiteVotesCountKey  = "site:24h_votes"  // STRING with 24h rolling count
	VotesStreamKey     = "votes"           // STREAM for vote events
	VotesConsumerGroup = "vote_processors" // Consumer group name
	VotesConsumerName  = "processor_%d"    // Consumer name pattern
)

// SubmitVote adds a vote to the stream and checks for duplicates
func (u *UpvotesRedisService) SubmitVote(userID, projectID string) error {
	// Check if user already voted for this project
	hasVoted, err := u.client.SIsMember(u.ctx, fmt.Sprintf(UserVotesSetKey, userID), projectID).Result()
	if err != nil {
		return fmt.Errorf("failed to check existing vote: %w", err)
	}

	if hasVoted {
		return fmt.Errorf("user has already voted for this project")
	}

	// Create vote event
	event := VoteEvent{
		UserID:    userID,
		ProjectID: projectID,
		Timestamp: time.Now().UnixMilli(),
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal vote event: %w", err)
	}

	// Add to stream with max length to prevent unbounded growth
	_, err = u.client.XAdd(u.ctx, &redis.XAddArgs{
		Stream: VotesStreamKey,
		MaxLen: 100000, // Keep ~100k recent votes in stream
		Approx: true,   // Use approximate trimming for performance
		Values: map[string]interface{}{
			"d": string(eventJSON),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to add vote to stream: %w", err)
	}

	return nil
}

// ProcessVoteEvent processes a single vote event from the stream
func (u *UpvotesRedisService) ProcessVoteEvent(event VoteEvent) error {
	projectKey := fmt.Sprintf(ProjectHashKey, event.ProjectID)
	userVotesKey := fmt.Sprintf(UserVotesSetKey, event.UserID)

	// Use pipeline for atomic operations
	pipe := u.client.Pipeline()

	// Increment project vote counter
	pipe.HIncrBy(u.ctx, projectKey, "up", 1)

	// Add project to user's voted set
	pipe.SAdd(u.ctx, userVotesKey, event.ProjectID)

	// Set TTL on user votes set (7 days for inactive users)
	pipe.Expire(u.ctx, userVotesKey, 7*24*time.Hour)

	// Increment site-wide 24h vote counter
	pipe.Incr(u.ctx, SiteVotesCountKey)
	pipe.Expire(u.ctx, SiteVotesCountKey, 24*time.Hour)

	// Execute pipeline
	_, err := pipe.Exec(u.ctx)
	if err != nil {
		return fmt.Errorf("failed to process vote event: %w", err)
	}

	// Update trending score (separate from pipeline for error handling)
	err = u.UpdateTrendingScore(event.ProjectID)
	if err != nil {
		log.Printf("Warning: failed to update trending score for project %s: %v", event.ProjectID, err)
		// Don't fail the entire operation for trending score issues
	}

	return nil
}

// UpdateTrendingScore calculates and updates the trending score for a project
func (u *UpvotesRedisService) UpdateTrendingScore(projectID string) error {
	// Get current vote count
	projectKey := fmt.Sprintf(ProjectHashKey, projectID)
	voteCountStr, err := u.client.HGet(u.ctx, projectKey, "up").Result()
	if err == redis.Nil {
		voteCountStr = "0"
	} else if err != nil {
		return fmt.Errorf("failed to get vote count: %w", err)
	}

	voteCount, err := strconv.ParseFloat(voteCountStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse vote count: %w", err)
	}

	// Get 24h site votes for K calculation
	siteVotesStr, err := u.client.Get(u.ctx, SiteVotesCountKey).Result()
	if err == redis.Nil {
		siteVotesStr = "1" // Default to prevent division by zero
	} else if err != nil {
		return fmt.Errorf("failed to get site votes: %w", err)
	}

	siteVotes, err := strconv.ParseFloat(siteVotesStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse site votes: %w", err)
	}

	// Calculate trending score using Reddit-style algorithm
	score := u.CalculateTrendingScore(voteCount, time.Now(), siteVotes)

	// Update trending zset
	err = u.client.ZAdd(u.ctx, TrendingZSetKey, redis.Z{
		Score:  score,
		Member: fmt.Sprintf("project:%s", projectID),
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to update trending score: %w", err)
	}

	return nil
}

// CalculateTrendingScore implements the Reddit-style trending algorithm
func (u *UpvotesRedisService) CalculateTrendingScore(totalUpvotes float64, timestamp time.Time, siteVotesLast24h float64) float64 {
	// Reddit-style scoring: log10(max(1, totalUpvotes)) + (timestamp - epoch) / K
	// K = 45000 × √(siteVotesLast24h + 1)

	logScore := math.Log10(math.Max(1, totalUpvotes))

	// Use Unix timestamp in seconds for time component
	epoch := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	timeScore := float64(timestamp.Unix()-epoch) / (45000 * math.Sqrt(siteVotesLast24h+1))

	return logScore + timeScore
}

// GetProjectVoteCount gets the current vote count for a project
func (u *UpvotesRedisService) GetProjectVoteCount(projectID string) (int64, error) {
	projectKey := fmt.Sprintf(ProjectHashKey, projectID)
	voteCountStr, err := u.client.HGet(u.ctx, projectKey, "up").Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get vote count: %w", err)
	}

	voteCount, err := strconv.ParseInt(voteCountStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse vote count: %w", err)
	}

	return voteCount, nil
}

// GetUserVoteState checks if a user has voted for specific projects
func (u *UpvotesRedisService) GetUserVoteState(userID string, projectIDs []string) (map[string]bool, error) {
	if len(projectIDs) == 0 {
		return make(map[string]bool), nil
	}

	userVotesKey := fmt.Sprintf(UserVotesSetKey, userID)

	// Convert project IDs to interfaces for Redis call
	members := make([]interface{}, len(projectIDs))
	for i, id := range projectIDs {
		members[i] = id
	}

	// Use SMISMEMBER for batch checking (Redis 6.2+)
	results, err := u.client.SMIsMember(u.ctx, userVotesKey, members...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check vote states: %w", err)
	}

	// Build result map (empty marker "__empty__" will naturally return false for real project IDs)
	voteStates := make(map[string]bool)
	for i, projectID := range projectIDs {
		voteStates[projectID] = results[i]
	}

	return voteStates, nil
}

// GetTrendingProjects gets the top trending projects
func (u *UpvotesRedisService) GetTrendingProjects(limit int64) ([]string, error) {
	// Get top projects from trending zset (highest scores first)
	results, err := u.client.ZRevRange(u.ctx, TrendingZSetKey, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get trending projects: %w", err)
	}

	// Extract project IDs (remove "project:" prefix)
	projectIDs := make([]string, len(results))
	for i, result := range results {
		if len(result) > 8 && result[:8] == "project:" {
			projectIDs[i] = result[8:] // Remove "project:" prefix
		} else {
			projectIDs[i] = result // Fallback if format is unexpected
		}
	}

	return projectIDs, nil
}

// InitializeProjectFromDB initializes Redis counters from database stats
func (u *UpvotesRedisService) InitializeProjectFromDB(projectID string, voteCount int64, lastVoteTime time.Time) error {
	projectKey := fmt.Sprintf(ProjectHashKey, projectID)

	// Set vote count
	err := u.client.HSet(u.ctx, projectKey, "up", voteCount).Err()
	if err != nil {
		return fmt.Errorf("failed to initialize project vote count: %w", err)
	}

	// Update trending score
	siteVotesStr, err := u.client.Get(u.ctx, SiteVotesCountKey).Result()
	if err == redis.Nil {
		siteVotesStr = "1"
	} else if err != nil {
		return fmt.Errorf("failed to get site votes: %w", err)
	}

	siteVotes, err := strconv.ParseFloat(siteVotesStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse site votes: %w", err)
	}

	score := u.CalculateTrendingScore(float64(voteCount), lastVoteTime, siteVotes)

	err = u.client.ZAdd(u.ctx, TrendingZSetKey, redis.Z{
		Score:  score,
		Member: fmt.Sprintf("project:%s", projectID),
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to initialize trending score: %w", err)
	}

	return nil
}

// CreateConsumerGroup creates the consumer group for vote processing
func (u *UpvotesRedisService) CreateConsumerGroup() error {
	// Try to create consumer group (ignore error if already exists)
	err := u.client.XGroupCreate(u.ctx, VotesStreamKey, VotesConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}

// GetPendingVoteEvents gets pending vote events for a consumer
func (u *UpvotesRedisService) GetPendingVoteEvents(consumerID string, count int64) ([]redis.XMessage, error) {
	consumerName := fmt.Sprintf(VotesConsumerName, consumerID)

	// Read pending messages first, then new messages
	results, err := u.client.XReadGroup(u.ctx, &redis.XReadGroupArgs{
		Group:    VotesConsumerGroup,
		Consumer: consumerName,
		Streams:  []string{VotesStreamKey, ">"},
		Count:    count,
		Block:    time.Second, // Block for 1 second max
	}).Result()

	if err == redis.Nil {
		return []redis.XMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read vote events: %w", err)
	}

	if len(results) == 0 || len(results[0].Messages) == 0 {
		return []redis.XMessage{}, nil
	}

	return results[0].Messages, nil
}

// AckVoteEvent acknowledges processing of a vote event
func (u *UpvotesRedisService) AckVoteEvent(messageID string) error {
	err := u.client.XAck(u.ctx, VotesStreamKey, VotesConsumerGroup, messageID).Err()
	if err != nil {
		return fmt.Errorf("failed to ack vote event: %w", err)
	}
	return nil
}
