package models

import (
	"encoding/json"
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
	ID                    string    `json:"id" db:"id"`
	Title                 string    `json:"title" db:"title"`
	ShortDesc             string    `json:"short_desc" db:"short_desc"`
	LongDesc              string    `json:"long_desc" db:"long_desc"`
	LogoURL               string    `json:"logo_url" db:"logo_url"`
	WebsiteURL            *string   `json:"website_url,omitempty" db:"website_url"`
	GitHubURL             *string   `json:"github_url,omitempty" db:"github_url"`
	DocsURL               *string   `json:"docs_url,omitempty" db:"docs_url"`
	AuditReports          []string  `json:"audit_reports,omitempty" db:"audit_reports"`
	MarkdownContent       string    `json:"markdown_content" db:"markdown_content"`
	SubmitterID           string    `json:"submitter_id" db:"submitter_id"`
	Approved              bool      `json:"approved" db:"approved"`
	IsVerified            bool      `json:"is_verified" db:"is_verified"`
	AnalyticsList         []string  `json:"analytics_list" db:"analytics_list"`
	OverallScore          float64   `json:"overall_score" db:"overall_score"`
	SecurityScore         float64   `json:"security_score" db:"security_score"`
	UXScore               float64   `json:"ux_score" db:"ux_score"`
	VibesScore            float64   `json:"vibes_score" db:"vibes_score"`
	CurrentRevisionNumber int       `json:"current_revision_number" db:"current_revision_number"`
	LastEditorID          *string   `json:"last_editor_id" db:"last_editor_id"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`

	// Relationships
	Categories  []Category `json:"categories" db:"-"`
	Chains      []Chain    `json:"chains" db:"-"`
	UpvoteCount int        `json:"upvote_count,omitempty" db:"-"`
	Submitter   *User      `json:"submitter,omitempty" db:"-"`
	LastEditor  *User      `json:"last_editor,omitempty" db:"-"`
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

// ProductRevision represents a complete snapshot of a product at a specific point in time
type ProductRevision struct {
	ID             string          `json:"id" db:"id"`
	ProductID      string          `json:"product_id" db:"product_id"`
	RevisionNumber int             `json:"revision_number" db:"revision_number"`
	EditorID       *string         `json:"editor_id" db:"editor_id"`
	EditSummary    *string         `json:"edit_summary" db:"edit_summary"`
	DiffData       json.RawMessage `json:"diff_data" db:"diff_data"`
	ProductData    json.RawMessage `json:"product_data" db:"product_data"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`

	// Relationships
	Editor       *User                `json:"editor,omitempty" db:"-"`
	FieldChanges []ProductFieldChange `json:"field_changes,omitempty" db:"-"`
	Product      *Product             `json:"product,omitempty" db:"-"`
}

// ProductFieldChange represents a single field change in a product revision
type ProductFieldChange struct {
	ID         string  `json:"id" db:"id"`
	RevisionID string  `json:"revision_id" db:"revision_id"`
	FieldName  string  `json:"field_name" db:"field_name"`
	OldValue   *string `json:"old_value" db:"old_value"`
	NewValue   *string `json:"new_value" db:"new_value"`
	ChangeType string  `json:"change_type" db:"change_type"` // 'added', 'modified', 'removed'
}

// ProductDiff represents the differences between two product revisions
type ProductDiff struct {
	FromRevision int                  `json:"from_revision"`
	ToRevision   int                  `json:"to_revision"`
	Changes      []ProductFieldChange `json:"changes"`
	Summary      string               `json:"summary"`
}

// RevisionSummary represents a summary of changes for display in history lists
type RevisionSummary struct {
	RevisionNumber int       `json:"revision_number"`
	EditSummary    *string   `json:"edit_summary"`
	EditorID       *string   `json:"editor_id"`
	Editor         *User     `json:"editor,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ChangeCount    int       `json:"change_count"`
	MajorChange    bool      `json:"major_change"`
}

// Redis-based pending changes models for git-like workflow

// PendingProduct represents a new product submission stored in Redis
type PendingProduct struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Product     Product   `json:"product"`
	SubmittedAt time.Time `json:"submitted_at"`
	Status      string    `json:"status"` // "pending", "approved", "rejected"
}

// PendingProductEdit represents an edit to an existing product stored in Redis
type PendingProductEdit struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ProductID    string    `json:"product_id"`
	OriginalData Product   `json:"original_data"` // Snapshot of product before edit
	UpdatedData  Product   `json:"updated_data"`  // Proposed changes
	EditSummary  string    `json:"edit_summary"`
	SubmittedAt  time.Time `json:"submitted_at"`
	Status       string    `json:"status"` // "pending", "approved", "rejected"
}

// PendingChangesList represents a list of all pending changes for admin review
type PendingChangesList struct {
	PendingProducts []PendingProduct     `json:"pending_products"`
	PendingEdits    []PendingProductEdit `json:"pending_edits"`
	TotalCount      int                  `json:"total_count"`
}
