package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID            string    `json:"id" db:"id"`
	WalletAddress string    `json:"wallet_address" db:"wallet_address"`
	TwitterHandle string    `json:"twitter_handle,omitempty" db:"twitter_handle"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`

	// Internal metrics
	SubmittedProducts int `json:"submitted_products,omitempty" db:"-"`
	Upvotes           int `json:"upvotes,omitempty" db:"-"`
}

// Product represents a crypto product
type Product struct {
	ID              string    `json:"id" db:"id"`
	Title           string    `json:"title" db:"title"`
	ShortDesc       string    `json:"short_desc" db:"short_desc"`
	LongDesc        string    `json:"long_desc" db:"long_desc"`
	LogoURL         string    `json:"logo_url" db:"logo_url"`
	MarkdownContent string    `json:"markdown_content" db:"markdown_content"`
	SubmitterID     string    `json:"submitter_id" db:"submitter_id"`
	Approved        bool      `json:"approved" db:"approved"`
	IsVerified      bool      `json:"is_verified" db:"is_verified"`
	AnalyticsList   []string  `json:"analytics_list" db:"analytics_list"`
	SecurityScore   float64   `json:"security_score" db:"security_score"`
	UXScore         float64   `json:"ux_score" db:"ux_score"`
	DecentScore     float64   `json:"decent_score" db:"decent_score"`
	VibesScore      float64   `json:"vibes_score" db:"vibes_score"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`

	// Relationships
	Categories  []Category `json:"categories,omitempty" db:"-"`
	Chains      []Chain    `json:"chains,omitempty" db:"-"`
	UpvoteCount int        `json:"upvote_count,omitempty" db:"-"`
	Submitter   *User      `json:"submitter,omitempty" db:"-"`
}

// Category represents a product category
type Category struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Relationships
	ProductCount int `json:"product_count,omitempty" db:"-"`
}

// Chain represents a blockchain network
type Chain struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Icon      string    `json:"icon" db:"icon"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Upvote represents a user's upvote on a product
type Upvote struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	ProductID string    `json:"product_id" db:"product_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// PendingEdit represents a pending edit to a product or category
type PendingEdit struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	EntityType  string    `json:"entity_type" db:"entity_type"` // "product" or "category"
	EntityID    string    `json:"entity_id" db:"entity_id"`
	ChangeType  string    `json:"change_type" db:"change_type"` // "create", "update"
	ChangeData  string    `json:"change_data" db:"change_data"` // JSON string with the changes
	Status      string    `json:"status" db:"status"`           // "pending", "approved", "rejected"
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	ProcessedAt time.Time `json:"processed_at,omitempty" db:"processed_at"`
}

// ProductFilter holds criteria for filtering products
type ProductFilter struct {
	CategoryID  string `json:"category_id"`
	ChainID     string `json:"chain_id"`
	SearchQuery string `json:"search_query"`
	SortBy      string `json:"sort_by"` // "new", "top_day", "top_week", "top_month", "top_year", "top_all"
	Page        int    `json:"page"`
	PerPage     int    `json:"per_page"`
}

// AppStats represents application statistics
type AppStats struct {
	TotalProducts   int `json:"total_products"`
	TotalUsers      int `json:"total_users"`
	TotalCategories int `json:"total_categories"`
	TotalUpvotes    int `json:"total_upvotes"`
}
