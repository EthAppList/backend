package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wesjorgensen/EthAppList/backend/internal/models"
)

// RedisService handles all Redis operations for pending changes
type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisService creates a new Redis service instance
func NewRedisService(redisURL string) (*RedisService, error) {
	// Parse Redis URL
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Create Redis client
	client := redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	_, err = client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Printf("Successfully connected to Redis at %s", redisURL)

	return &RedisService{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close closes the Redis connection
func (r *RedisService) Close() error {
	return r.client.Close()
}

// Keys for Redis storage
const (
	PendingProductPrefix = "pending_product:"
	PendingEditPrefix    = "pending_edit:"
	PendingListKey       = "pending_changes_list"
)

// StorePendingProduct stores a new product submission in Redis
func (r *RedisService) StorePendingProduct(pendingProduct *models.PendingProduct) error {
	// Generate ID if not provided
	if pendingProduct.ID == "" {
		pendingProduct.ID = fmt.Sprintf("pending_product_%d", time.Now().UnixNano())
	}

	// Set submitted timestamp
	pendingProduct.SubmittedAt = time.Now()
	pendingProduct.Status = "pending"

	// Serialize to JSON
	data, err := json.Marshal(pendingProduct)
	if err != nil {
		return fmt.Errorf("failed to marshal pending product: %w", err)
	}

	// Store in Redis with expiration (30 days)
	key := PendingProductPrefix + pendingProduct.ID
	err = r.client.Set(r.ctx, key, data, 30*24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to store pending product in Redis: %w", err)
	}

	// Add to pending list for admin dashboard
	err = r.client.SAdd(r.ctx, PendingListKey, key).Err()
	if err != nil {
		return fmt.Errorf("failed to add to pending list: %w", err)
	}

	log.Printf("Stored pending product %s for user %s", pendingProduct.ID, pendingProduct.UserID)
	return nil
}

// StorePendingEdit stores a product edit in Redis
func (r *RedisService) StorePendingEdit(pendingEdit *models.PendingProductEdit) error {
	// Generate ID if not provided
	if pendingEdit.ID == "" {
		pendingEdit.ID = fmt.Sprintf("pending_edit_%s_%d", pendingEdit.ProductID, time.Now().UnixNano())
	}

	// Set submitted timestamp
	pendingEdit.SubmittedAt = time.Now()
	pendingEdit.Status = "pending"

	// Serialize to JSON
	data, err := json.Marshal(pendingEdit)
	if err != nil {
		return fmt.Errorf("failed to marshal pending edit: %w", err)
	}

	// Store in Redis with expiration (30 days)
	key := PendingEditPrefix + pendingEdit.ID
	err = r.client.Set(r.ctx, key, data, 30*24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to store pending edit in Redis: %w", err)
	}

	// Add to pending list for admin dashboard
	err = r.client.SAdd(r.ctx, PendingListKey, key).Err()
	if err != nil {
		return fmt.Errorf("failed to add to pending list: %w", err)
	}

	log.Printf("Stored pending edit %s for product %s by user %s", pendingEdit.ID, pendingEdit.ProductID, pendingEdit.UserID)
	return nil
}

// GetPendingProduct retrieves a pending product by ID
func (r *RedisService) GetPendingProduct(id string) (*models.PendingProduct, error) {
	key := PendingProductPrefix + id
	data, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("pending product not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending product: %w", err)
	}

	var pendingProduct models.PendingProduct
	err = json.Unmarshal([]byte(data), &pendingProduct)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending product: %w", err)
	}

	return &pendingProduct, nil
}

// GetPendingEdit retrieves a pending edit by ID
func (r *RedisService) GetPendingEdit(id string) (*models.PendingProductEdit, error) {
	key := PendingEditPrefix + id
	data, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("pending edit not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending edit: %w", err)
	}

	var pendingEdit models.PendingProductEdit
	err = json.Unmarshal([]byte(data), &pendingEdit)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending edit: %w", err)
	}

	return &pendingEdit, nil
}

// GetAllPendingChanges retrieves all pending changes for admin review
func (r *RedisService) GetAllPendingChanges() (*models.PendingChangesList, error) {
	// Get all pending keys
	keys, err := r.client.SMembers(r.ctx, PendingListKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pending list: %w", err)
	}

	pendingProducts := []models.PendingProduct{}
	pendingEdits := []models.PendingProductEdit{}

	for _, key := range keys {
		// Check if key exists (might have expired)
		exists, err := r.client.Exists(r.ctx, key).Result()
		if err != nil {
			log.Printf("Error checking key existence: %v", err)
			continue
		}
		if exists == 0 {
			// Remove from set if expired
			r.client.SRem(r.ctx, PendingListKey, key)
			continue
		}

		data, err := r.client.Get(r.ctx, key).Result()
		if err != nil {
			log.Printf("Error getting data for key %s: %v", key, err)
			continue
		}

		if strings.HasPrefix(key, PendingProductPrefix) {
			var pendingProduct models.PendingProduct
			if err := json.Unmarshal([]byte(data), &pendingProduct); err == nil {
				if pendingProduct.Status == "pending" {
					pendingProducts = append(pendingProducts, pendingProduct)
				}
			}
		} else if strings.HasPrefix(key, PendingEditPrefix) {
			var pendingEdit models.PendingProductEdit
			if err := json.Unmarshal([]byte(data), &pendingEdit); err == nil {
				if pendingEdit.Status == "pending" {
					pendingEdits = append(pendingEdits, pendingEdit)
				}
			}
		}
	}

	return &models.PendingChangesList{
		PendingProducts: pendingProducts,
		PendingEdits:    pendingEdits,
		TotalCount:      len(pendingProducts) + len(pendingEdits),
	}, nil
}

// ApprovePendingProduct marks a pending product as approved and returns it for database storage
func (r *RedisService) ApprovePendingProduct(id string) (*models.PendingProduct, error) {
	pendingProduct, err := r.GetPendingProduct(id)
	if err != nil {
		return nil, err
	}

	pendingProduct.Status = "approved"

	// Update in Redis
	data, err := json.Marshal(pendingProduct)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal approved product: %w", err)
	}

	key := PendingProductPrefix + id
	err = r.client.Set(r.ctx, key, data, time.Hour).Err() // Short expiry after approval
	if err != nil {
		return nil, fmt.Errorf("failed to update approved product: %w", err)
	}

	// Remove from pending list
	err = r.client.SRem(r.ctx, PendingListKey, key).Err()
	if err != nil {
		log.Printf("Failed to remove from pending list: %v", err)
	}

	return pendingProduct, nil
}

// ApprovePendingEdit marks a pending edit as approved and returns it for database merging
func (r *RedisService) ApprovePendingEdit(id string) (*models.PendingProductEdit, error) {
	pendingEdit, err := r.GetPendingEdit(id)
	if err != nil {
		return nil, err
	}

	pendingEdit.Status = "approved"

	// Update in Redis
	data, err := json.Marshal(pendingEdit)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal approved edit: %w", err)
	}

	key := PendingEditPrefix + id
	err = r.client.Set(r.ctx, key, data, time.Hour).Err() // Short expiry after approval
	if err != nil {
		return nil, fmt.Errorf("failed to update approved edit: %w", err)
	}

	// Remove from pending list
	err = r.client.SRem(r.ctx, PendingListKey, key).Err()
	if err != nil {
		log.Printf("Failed to remove from pending list: %v", err)
	}

	return pendingEdit, nil
}

// RejectPendingChange marks a pending change as rejected and removes it
func (r *RedisService) RejectPendingChange(id string) error {
	// Try both product and edit prefixes
	productKey := PendingProductPrefix + id
	editKey := PendingEditPrefix + id

	// Check which key exists
	productExists, _ := r.client.Exists(r.ctx, productKey).Result()
	editExists, _ := r.client.Exists(r.ctx, editKey).Result()

	var key string
	if productExists > 0 {
		key = productKey
	} else if editExists > 0 {
		key = editKey
	} else {
		return fmt.Errorf("pending change not found")
	}

	// Remove from Redis
	err := r.client.Del(r.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete pending change: %w", err)
	}

	// Remove from pending list
	err = r.client.SRem(r.ctx, PendingListKey, key).Err()
	if err != nil {
		log.Printf("Failed to remove from pending list: %v", err)
	}

	log.Printf("Rejected and removed pending change %s", id)
	return nil
}

// CleanupProcessedChanges removes approved/rejected changes older than 1 hour
func (r *RedisService) CleanupProcessedChanges() error {
	// This could be run as a periodic cleanup job
	// For now, we rely on TTL expiration
	return nil
}
