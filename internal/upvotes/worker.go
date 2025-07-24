package upvotes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// VoteWorker processes vote events from Redis streams
type VoteWorker struct {
	upvotesRedis *UpvotesRedisService
	db           *sql.DB
	consumerID   int
	running      bool
	stopChan     chan struct{}
}

// NewVoteWorker creates a new vote processing worker
func NewVoteWorker(upvotesRedis *UpvotesRedisService, db *sql.DB, consumerID int) *VoteWorker {
	return &VoteWorker{
		upvotesRedis: upvotesRedis,
		db:           db,
		consumerID:   consumerID,
		stopChan:     make(chan struct{}),
	}
}

// Start begins processing vote events
func (w *VoteWorker) Start() error {
	log.Printf("Starting vote worker %d", w.consumerID)

	// Create consumer group if it doesn't exist
	err := w.upvotesRedis.CreateConsumerGroup()
	if err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	w.running = true
	go w.processLoop()

	return nil
}

// Stop gracefully stops the worker
func (w *VoteWorker) Stop() {
	log.Printf("Stopping vote worker %d", w.consumerID)
	w.running = false
	close(w.stopChan)
}

// processLoop is the main processing loop for the worker
func (w *VoteWorker) processLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Vote worker %d panic recovered: %v", w.consumerID, r)
			// Restart the worker after a panic
			if w.running {
				time.Sleep(5 * time.Second)
				go w.processLoop()
			}
		}
	}()

	for w.running {
		select {
		case <-w.stopChan:
			log.Printf("Vote worker %d received stop signal", w.consumerID)
			return
		default:
			err := w.processBatch()
			if err != nil {
				log.Printf("Vote worker %d error processing batch: %v", w.consumerID, err)
				time.Sleep(time.Second) // Brief pause before retrying
			}
		}
	}
}

// processBatch processes a batch of vote events
func (w *VoteWorker) processBatch() error {
	// Get pending vote events
	messages, err := w.upvotesRedis.GetPendingVoteEvents(strconv.Itoa(w.consumerID), 10) // Process up to 10 at a time
	if err != nil {
		return fmt.Errorf("failed to get pending vote events: %w", err)
	}

	if len(messages) == 0 {
		// No messages, brief sleep to avoid busy waiting
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	log.Printf("Vote worker %d processing %d messages", w.consumerID, len(messages))

	// Process each message
	for _, message := range messages {
		err := w.processMessage(message)
		if err != nil {
			log.Printf("Vote worker %d failed to process message %s: %v", w.consumerID, message.ID, err)
			// Continue with other messages, don't ack this one
			continue
		}

		// Acknowledge successful processing
		err = w.upvotesRedis.AckVoteEvent(message.ID)
		if err != nil {
			log.Printf("Vote worker %d failed to ack message %s: %v", w.consumerID, message.ID, err)
		}
	}

	return nil
}

// processMessage processes a single vote event message
func (w *VoteWorker) processMessage(message redis.XMessage) error {
	// Extract event data
	eventData, ok := message.Values["d"].(string)
	if !ok {
		return fmt.Errorf("missing event data in message")
	}

	// Parse vote event
	var event VoteEvent
	err := json.Unmarshal([]byte(eventData), &event)
	if err != nil {
		return fmt.Errorf("failed to unmarshal vote event: %w", err)
	}

	// Validate event
	if event.UserID == "" || event.ProjectID == "" {
		return fmt.Errorf("invalid vote event: missing user or project ID")
	}

	// Process the vote event
	err = w.ProcessVoteEvent(event)
	if err != nil {
		return fmt.Errorf("failed to process vote event: %w", err)
	}

	log.Printf("Vote worker %d processed vote: user=%s, project=%s", w.consumerID, event.UserID, event.ProjectID)
	return nil
}

// ProcessVoteEvent processes a single vote event from the stream
func (w *VoteWorker) ProcessVoteEvent(event VoteEvent) error {
	// First, write to database for immutable log
	err := w.writeVoteToDatabase(event)
	if err != nil {
		return fmt.Errorf("failed to write vote to database: %w", err)
	}

	// Then update Redis counters and user state
	err = w.upvotesRedis.ProcessVoteEvent(event)
	if err != nil {
		log.Printf("Warning: Redis counter update failed for vote %s->%s: %v", event.UserID, event.ProjectID, err)
		// Don't fail the entire operation since database write succeeded
	}

	return nil
}

// writeVoteToDatabase writes the vote to the immutable database log
func (w *VoteWorker) writeVoteToDatabase(event VoteEvent) error {
	// Check if vote already exists (idempotent)
	var count int
	err := w.db.QueryRow(
		"SELECT COUNT(*) FROM votes WHERE user_id = $1 AND product_id = $2",
		event.UserID, event.ProjectID,
	).Scan(&count)

	if err != nil {
		return fmt.Errorf("failed to check existing vote: %w", err)
	}

	if count > 0 {
		// Vote already exists, this is idempotent processing
		return nil
	}

	// Generate vote ID
	voteID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Insert vote into database
	_, err = w.db.Exec(
		"INSERT INTO votes (id, user_id, product_id, created_at) VALUES ($1, $2, $3, $4)",
		voteID, event.UserID, event.ProjectID, time.UnixMilli(event.Timestamp),
	)

	if err != nil {
		return fmt.Errorf("failed to insert vote: %w", err)
	}

	// Update project_stats for fast lookups
	_, err = w.db.Exec(`
		INSERT INTO project_stats (project_id, upvotes_total, last_vote_ts)
		VALUES ($1, 1, $2)
		ON CONFLICT (project_id) DO UPDATE SET
			upvotes_total = project_stats.upvotes_total + 1,
			last_vote_ts = $2`,
		event.ProjectID, time.UnixMilli(event.Timestamp),
	)

	if err != nil {
		log.Printf("Warning: failed to update project_stats for product %s: %v", event.ProjectID, err)
		// Don't fail the entire operation for stats update issues
	}

	return nil
}

// WorkerManager manages multiple vote workers
type WorkerManager struct {
	workers      []*VoteWorker
	upvotesRedis *UpvotesRedisService
	db           *sql.DB
	workerCount  int
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(upvotesRedis *UpvotesRedisService, db *sql.DB) *WorkerManager {
	// Default to 2 workers, can be overridden by environment variable
	workerCount := 2
	if envWorkers := os.Getenv("VOTE_WORKERS"); envWorkers != "" {
		if count, err := strconv.Atoi(envWorkers); err == nil && count > 0 {
			workerCount = count
		}
	}

	return &WorkerManager{
		upvotesRedis: upvotesRedis,
		db:           db,
		workerCount:  workerCount,
		workers:      make([]*VoteWorker, 0, workerCount),
	}
}

// StartWorkers starts all vote processing workers
func (wm *WorkerManager) StartWorkers() error {
	log.Printf("Starting %d vote workers", wm.workerCount)

	for i := 0; i < wm.workerCount; i++ {
		worker := NewVoteWorker(wm.upvotesRedis, wm.db, i)
		err := worker.Start()
		if err != nil {
			// Stop any workers that were already started
			wm.StopWorkers()
			return fmt.Errorf("failed to start worker %d: %w", i, err)
		}
		wm.workers = append(wm.workers, worker)
	}

	log.Printf("Successfully started %d vote workers", len(wm.workers))
	return nil
}

// StopWorkers stops all vote processing workers
func (wm *WorkerManager) StopWorkers() {
	log.Printf("Stopping %d vote workers", len(wm.workers))

	for i, worker := range wm.workers {
		log.Printf("Stopping worker %d", i)
		worker.Stop()
	}

	wm.workers = wm.workers[:0] // Clear the slice
	log.Printf("All vote workers stopped")
}

// GetWorkerCount returns the number of active workers
func (wm *WorkerManager) GetWorkerCount() int {
	return len(wm.workers)
}
