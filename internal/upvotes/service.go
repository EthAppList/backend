package upvotes

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/wesjorgensen/EthAppList/backend/internal/models"
)

// UpvotesService provides high-level upvoting functionality
type UpvotesService struct {
	redis         *UpvotesRedisService
	workerManager *WorkerManager
	db            *sql.DB
}

// NewUpvotesService creates a new upvotes service
func NewUpvotesService(redisURL string, db *sql.DB) (*UpvotesService, error) {
	// Initialize Redis service
	redis, err := NewUpvotesRedisService(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create upvotes Redis service: %w", err)
	}

	// Initialize worker manager with database connection
	workerManager := NewWorkerManager(redis, db)

	return &UpvotesService{
		redis:         redis,
		workerManager: workerManager,
		db:            db,
	}, nil
}

// WarmUserCache rebuilds a user's vote cache from database after TTL expiry
func (s *UpvotesService) WarmUserCache(userID string) error {
	// Check if cache exists and has data
	userVotesKey := fmt.Sprintf("user:%s:votes", userID)
	exists, err := s.redis.client.Exists(s.redis.ctx, userVotesKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check cache existence: %w", err)
	}

	if exists > 0 {
		// Cache already exists, no need to warm
		return nil
	}

	log.Printf("Warming vote cache for user %s", userID)

	// Query all products this user has voted for from database
	query := `SELECT product_id FROM votes WHERE user_id = $1`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return fmt.Errorf("failed to query user votes: %w", err)
	}
	defer rows.Close()

	// Collect project IDs
	var projectIDs []interface{}
	for rows.Next() {
		var projectID string
		if err := rows.Scan(&projectID); err != nil {
			log.Printf("Warning: failed to scan vote row for user %s: %v", userID, err)
			continue
		}
		projectIDs = append(projectIDs, projectID)
	}

	if len(projectIDs) == 0 {
		// User has no votes, set empty cache with TTL to avoid repeated database queries
		_, err = s.redis.client.SAdd(s.redis.ctx, userVotesKey, "__empty__").Result()
		if err != nil {
			return fmt.Errorf("failed to create empty cache marker: %w", err)
		}
		s.redis.client.Expire(s.redis.ctx, userVotesKey, 30*24*time.Hour)
		return nil
	}

	// Populate Redis cache with user's votes
	_, err = s.redis.client.SAdd(s.redis.ctx, userVotesKey, projectIDs...).Result()
	if err != nil {
		return fmt.Errorf("failed to populate vote cache: %w", err)
	}

	// Set TTL for memory management (30 days)
	err = s.redis.client.Expire(s.redis.ctx, userVotesKey, 30*24*time.Hour).Err()
	if err != nil {
		log.Printf("Warning: failed to set TTL on vote cache for user %s: %v", userID, err)
	}

	log.Printf("Warmed vote cache for user %s with %d votes", userID, len(projectIDs))
	return nil
}

// Start starts the background workers
func (s *UpvotesService) Start() error {
	log.Printf("Starting upvotes service")

	err := s.workerManager.StartWorkers()
	if err != nil {
		return fmt.Errorf("failed to start upvotes workers: %w", err)
	}

	log.Printf("Upvotes service started successfully")
	return nil
}

// Stop stops the background workers
func (s *UpvotesService) Stop() {
	log.Printf("Stopping upvotes service")
	s.workerManager.StopWorkers()
	s.redis.Close()
	log.Printf("Upvotes service stopped")
}

// SubmitVote submits a vote for processing
func (s *UpvotesService) SubmitVote(userID, productID string) error {
	return s.redis.SubmitVote(userID, productID)
}

// GetProductVoteCount gets the current vote count for a product from Redis
func (s *UpvotesService) GetProductVoteCount(productID string) (int64, error) {
	return s.redis.GetProjectVoteCount(productID)
}

// GetUserVoteStates checks which products a user has voted for (with smart cache warming)
func (s *UpvotesService) GetUserVoteStates(userID string, projectIDs []string) (map[string]bool, error) {
	// First try to warm the cache if needed
	err := s.WarmUserCache(userID)
	if err != nil {
		log.Printf("Warning: failed to warm cache for user %s: %v", userID, err)
		// Continue with the operation, might just be slower
	}

	return s.redis.GetUserVoteState(userID, projectIDs)
}

// GetTrendingProducts gets the top trending products
func (s *UpvotesService) GetTrendingProducts(limit int) ([]string, error) {
	return s.redis.GetTrendingProjects(int64(limit))
}

// InitializeProjectFromDB initializes Redis data from database stats
func (s *UpvotesService) InitializeProjectFromDB(projectID string, voteCount int64, lastVoteTime time.Time) error {
	return s.redis.InitializeProjectFromDB(projectID, voteCount, lastVoteTime)
}

// EnrichProductsWithVoteCounts adds vote counts to products (user vote states fetched separately)
func (s *UpvotesService) EnrichProductsWithVoteCounts(products []*models.Product) error {
	if len(products) == 0 {
		return nil
	}

	// Get vote counts for all products (batch from Redis)
	for _, product := range products {
		count, err := s.redis.GetProjectVoteCount(product.ID)
		if err != nil {
			log.Printf("Warning: failed to get vote count for product %s: %v", product.ID, err)
			count = 0 // Default to 0 on error
		}
		product.UpvoteCount = int(count)
	}

	return nil
}

// GetWorkerStatus returns status information about the workers
func (s *UpvotesService) GetWorkerStatus() map[string]interface{} {
	return map[string]interface{}{
		"worker_count": s.workerManager.GetWorkerCount(),
		"status":       "running",
	}
}
