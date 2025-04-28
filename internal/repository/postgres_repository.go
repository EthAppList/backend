package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/wesjorgensen/EthAppList/backend/internal/config"
	"github.com/wesjorgensen/EthAppList/backend/internal/models"
)

// PostgresRepository handles all database interactions using direct PostgreSQL connection
type PostgresRepository struct {
	db  *sql.DB
	cfg *config.Config
}

// NewPostgres creates a new PostgreSQL repository instance
func NewPostgres(cfg *config.Config) (*PostgresRepository, error) {
	// Create connection string from config
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	// Connect to the database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{
		db:  db,
		cfg: cfg,
	}, nil
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// CreateUser creates a new user in the database
func (r *PostgresRepository) CreateUser(user *models.User) error {
	if user.ID == "" {
		user.ID = generateID()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	query := `
		INSERT INTO users (id, wallet_address, twitter_handle, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, wallet_address, twitter_handle, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		user.ID,
		user.WalletAddress,
		user.TwitterHandle,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(
		&user.ID,
		&user.WalletAddress,
		&user.TwitterHandle,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByWallet gets a user by their wallet address
func (r *PostgresRepository) GetUserByWallet(walletAddress string) (*models.User, error) {
	query := `
		SELECT id, wallet_address, twitter_handle, created_at, updated_at
		FROM users
		WHERE wallet_address = $1
	`

	user := &models.User{}
	err := r.db.QueryRow(query, walletAddress).Scan(
		&user.ID,
		&user.WalletAddress,
		&user.TwitterHandle,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// CreateProduct creates a new product in the database
func (r *PostgresRepository) CreateProduct(product *models.Product) error {
	if product.ID == "" {
		product.ID = generateID()
	}
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()

	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert product
	query := `
		INSERT INTO products (id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, approved, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, approved, created_at, updated_at
	`

	err = tx.QueryRow(
		query,
		product.ID,
		product.Title,
		product.ShortDesc,
		product.LongDesc,
		product.LogoURL,
		product.MarkdownContent,
		product.SubmitterID,
		product.Approved,
		product.CreatedAt,
		product.UpdatedAt,
	).Scan(
		&product.ID,
		&product.Title,
		&product.ShortDesc,
		&product.LongDesc,
		&product.LogoURL,
		&product.MarkdownContent,
		&product.SubmitterID,
		&product.Approved,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}

	// Insert category relationships
	if len(product.Categories) > 0 {
		for _, category := range product.Categories {
			_, err = tx.Exec(
				"INSERT INTO product_categories (product_id, category_id) VALUES ($1, $2)",
				product.ID, category.ID,
			)
			if err != nil {
				return fmt.Errorf("failed to link product to category: %w", err)
			}
		}
	}

	// Insert chain relationships
	if len(product.Chains) > 0 {
		for _, chain := range product.Chains {
			_, err = tx.Exec(
				"INSERT INTO product_chains (product_id, chain_id) VALUES ($1, $2)",
				product.ID, chain.ID,
			)
			if err != nil {
				return fmt.Errorf("failed to link product to chain: %w", err)
			}
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetProductByID gets a product by its ID
func (r *PostgresRepository) GetProductByID(id string) (*models.Product, error) {
	query := `
		SELECT id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, approved, created_at, updated_at
		FROM products
		WHERE id = $1
	`

	product := &models.Product{}
	err := r.db.QueryRow(query, id).Scan(
		&product.ID,
		&product.Title,
		&product.ShortDesc,
		&product.LongDesc,
		&product.LogoURL,
		&product.MarkdownContent,
		&product.SubmitterID,
		&product.Approved,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("product not found")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	// Load categories
	if err = r.loadProductCategories(product); err != nil {
		return nil, err
	}

	// Load chains
	if err = r.loadProductChains(product); err != nil {
		return nil, err
	}

	// Get upvote count
	upvoteCount, err := r.getProductUpvoteCount(product.ID)
	if err != nil {
		return nil, err
	}
	product.UpvoteCount = upvoteCount

	return product, nil
}

// GetProducts gets a list of products with optional filters
func (r *PostgresRepository) GetProducts(
	categoryID, chainID, searchTerm, sortOption string,
	page, perPage int,
) ([]*models.Product, int, error) {
	// Base query for counting total
	countQuery := `
		SELECT COUNT(DISTINCT p.id) 
		FROM products p
	`

	// Base query for fetching products
	query := `
		SELECT DISTINCT p.id, p.title, p.short_desc, p.long_desc, p.logo_url, 
               p.markdown_content, p.submitter_id, p.approved, p.created_at, p.updated_at
		FROM products p
	`

	// Build where clause and arguments
	whereClause := "WHERE p.approved = true"
	args := []interface{}{}
	argIndex := 1

	// Add category filter if provided
	if categoryID != "" {
		countQuery += " JOIN product_categories pc ON p.id = pc.product_id"
		query += " JOIN product_categories pc ON p.id = pc.product_id"
		whereClause += fmt.Sprintf(" AND pc.category_id = $%d", argIndex)
		args = append(args, categoryID)
		argIndex++
	}

	// Add chain filter if provided
	if chainID != "" {
		countQuery += " JOIN product_chains pch ON p.id = pch.product_id"
		query += " JOIN product_chains pch ON p.id = pch.product_id"
		whereClause += fmt.Sprintf(" AND pch.chain_id = $%d", argIndex)
		args = append(args, chainID)
		argIndex++
	}

	// Add search filter if provided
	if searchTerm != "" {
		searchPattern := "%" + searchTerm + "%"
		whereClause += fmt.Sprintf(" AND (p.title ILIKE $%d OR p.short_desc ILIKE $%d)", argIndex, argIndex)
		args = append(args, searchPattern)
		argIndex++
	}

	// Add where clause to queries
	countQuery += " " + whereClause
	query += " " + whereClause

	// Add sorting
	switch sortOption {
	case "new":
		query += " ORDER BY p.created_at DESC"
	case "top_day", "top_week", "top_month", "top_year", "top_all":
		// For simplicity, we'll implement a basic version here
		// In a production app, you might use window functions or more complex queries
		query += " LEFT JOIN upvotes u ON p.id = u.product_id"

		// Define time window based on sort option
		var timeWindow string
		switch sortOption {
		case "top_day":
			timeWindow = "1 day"
		case "top_week":
			timeWindow = "1 week"
		case "top_month":
			timeWindow = "1 month"
		case "top_year":
			timeWindow = "1 year"
		case "top_all":
			timeWindow = "100 years" // practically all time
		}

		if sortOption != "top_all" {
			query += fmt.Sprintf(" WHERE u.created_at > NOW() - INTERVAL '%s'", timeWindow)
		}

		query += " GROUP BY p.id ORDER BY COUNT(u.id) DESC"
	default:
		query += " ORDER BY p.created_at DESC" // Default to newest
	}

	// Add pagination
	offset := (page - 1) * perPage
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", perPage, offset)

	// Get total count
	var total int
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// Execute query
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get products: %w", err)
	}
	defer rows.Close()

	// Process results
	products := []*models.Product{}
	for rows.Next() {
		product := &models.Product{}
		err := rows.Scan(
			&product.ID,
			&product.Title,
			&product.ShortDesc,
			&product.LongDesc,
			&product.LogoURL,
			&product.MarkdownContent,
			&product.SubmitterID,
			&product.Approved,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}

		// Load categories and chains
		if err = r.loadProductCategories(product); err != nil {
			return nil, 0, err
		}
		if err = r.loadProductChains(product); err != nil {
			return nil, 0, err
		}

		// Get upvote count
		upvoteCount, err := r.getProductUpvoteCount(product.ID)
		if err != nil {
			return nil, 0, err
		}
		product.UpvoteCount = upvoteCount

		products = append(products, product)
	}

	return products, total, nil
}

// Helper functions

func (r *PostgresRepository) loadProductCategories(product *models.Product) error {
	query := `
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at
		FROM categories c
		JOIN product_categories pc ON c.id = pc.category_id
		WHERE pc.product_id = $1
	`

	rows, err := r.db.Query(query, product.ID)
	if err != nil {
		return fmt.Errorf("failed to get product categories: %w", err)
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var category models.Category
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.Description,
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}

	product.Categories = categories
	return nil
}

func (r *PostgresRepository) loadProductChains(product *models.Product) error {
	query := `
		SELECT c.id, c.name, c.icon, c.created_at, c.updated_at
		FROM chains c
		JOIN product_chains pc ON c.id = pc.chain_id
		WHERE pc.product_id = $1
	`

	rows, err := r.db.Query(query, product.ID)
	if err != nil {
		return fmt.Errorf("failed to get product chains: %w", err)
	}
	defer rows.Close()

	chains := []models.Chain{}
	for rows.Next() {
		var chain models.Chain
		err := rows.Scan(
			&chain.ID,
			&chain.Name,
			&chain.Icon,
			&chain.CreatedAt,
			&chain.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to scan chain: %w", err)
		}
		chains = append(chains, chain)
	}

	product.Chains = chains
	return nil
}

func (r *PostgresRepository) getProductUpvoteCount(productID string) (int, error) {
	query := `SELECT COUNT(*) FROM upvotes WHERE product_id = $1`
	var count int
	err := r.db.QueryRow(query, productID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count upvotes: %w", err)
	}
	return count, nil
}

// Add the rest of the repository methods as needed
// (UpvoteProduct, GetCategories, CreateCategory, etc.)

// GetCategories returns all categories
func (r *PostgresRepository) GetCategories() ([]models.Category, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM categories ORDER BY name ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var category models.Category
		err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.Description,
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		// Get product count
		var count int
		countQuery := `SELECT COUNT(*) FROM product_categories WHERE category_id = $1`
		err = r.db.QueryRow(countQuery, category.ID).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to count products for category: %w", err)
		}
		category.ProductCount = count

		categories = append(categories, category)
	}

	return categories, nil
}

// CreateCategory creates a new category
func (r *PostgresRepository) CreateCategory(category *models.Category) error {
	if category.ID == "" {
		category.ID = generateID()
	}
	category.CreatedAt = time.Now()
	category.UpdatedAt = time.Now()

	query := `
		INSERT INTO categories (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, description, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		category.ID,
		category.Name,
		category.Description,
		category.CreatedAt,
		category.UpdatedAt,
	).Scan(
		&category.ID,
		&category.Name,
		&category.Description,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	return nil
}

// UpvoteProduct adds an upvote to a product
func (r *PostgresRepository) UpvoteProduct(userID, productID string) error {
	// Check if user already upvoted this product
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM upvotes WHERE user_id = $1 AND product_id = $2",
		userID, productID,
	).Scan(&count)

	if err != nil {
		return fmt.Errorf("failed to check existing upvote: %w", err)
	}

	if count > 0 {
		return errors.New("already upvoted")
	}

	// Add upvote
	upvoteID := generateID()
	_, err = r.db.Exec(
		"INSERT INTO upvotes (id, user_id, product_id, created_at) VALUES ($1, $2, $3, $4)",
		upvoteID, userID, productID, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to add upvote: %w", err)
	}

	return nil
}

// GetPendingEdits returns all pending edits
func (r *PostgresRepository) GetPendingEdits() ([]models.PendingEdit, error) {
	query := `
		SELECT id, user_id, entity_type, entity_id, change_type, change_data, status, created_at, processed_at
		FROM pending_edits
		WHERE status = 'pending'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending edits: %w", err)
	}
	defer rows.Close()

	pendingEdits := []models.PendingEdit{}
	for rows.Next() {
		var edit models.PendingEdit
		var processedAt sql.NullTime

		err := rows.Scan(
			&edit.ID,
			&edit.UserID,
			&edit.EntityType,
			&edit.EntityID,
			&edit.ChangeType,
			&edit.ChangeData,
			&edit.Status,
			&edit.CreatedAt,
			&processedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan pending edit: %w", err)
		}

		// Handle null processed_at
		if processedAt.Valid {
			edit.ProcessedAt = processedAt.Time
		}

		pendingEdits = append(pendingEdits, edit)
	}

	return pendingEdits, nil
}

// ApproveEdit approves a pending edit
func (r *PostgresRepository) ApproveEdit(editID string) error {
	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Get the pending edit details
	var edit models.PendingEdit
	var processedAt sql.NullTime

	err = tx.QueryRow(`
		SELECT id, user_id, entity_type, entity_id, change_type, change_data, status, created_at, processed_at
		FROM pending_edits
		WHERE id = $1
	`, editID).Scan(
		&edit.ID,
		&edit.UserID,
		&edit.EntityType,
		&edit.EntityID,
		&edit.ChangeType,
		&edit.ChangeData,
		&edit.Status,
		&edit.CreatedAt,
		&processedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to get edit details: %w", err)
	}

	// Handle null processed_at
	if processedAt.Valid {
		edit.ProcessedAt = processedAt.Time
	}

	// Check if edit is already processed
	if edit.Status != "pending" {
		return fmt.Errorf("edit is already %s", edit.Status)
	}

	// Apply the edit based on entity type and change type
	switch edit.EntityType {
	case "product":
		if edit.ChangeType == "create" {
			// Apply the product creation
			var product models.Product
			err = json.Unmarshal([]byte(edit.ChangeData), &product)
			if err != nil {
				return fmt.Errorf("failed to unmarshal product data: %w", err)
			}

			// Set approved flag
			product.Approved = true

			// If entity ID exists, use it; otherwise generate a new one
			if edit.EntityID != "" {
				product.ID = edit.EntityID
			}

			// Insert the product using the main logic
			// (simplified for this implementation)
			_, err = tx.Exec(`
				INSERT INTO products (id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, approved, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`,
				product.ID,
				product.Title,
				product.ShortDesc,
				product.LongDesc,
				product.LogoURL,
				product.MarkdownContent,
				product.SubmitterID,
				product.Approved,
				time.Now(),
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to insert approved product: %w", err)
			}
		} else if edit.ChangeType == "update" {
			// Parse the product data from the change data
			var product models.Product
			err = json.Unmarshal([]byte(edit.ChangeData), &product)
			if err != nil {
				return fmt.Errorf("failed to unmarshal product data: %w", err)
			}

			// Update the product
			_, err = tx.Exec(`
				UPDATE products
				SET title = $1, short_desc = $2, long_desc = $3, logo_url = $4, markdown_content = $5, updated_at = $6, approved = true
				WHERE id = $7
			`,
				product.Title,
				product.ShortDesc,
				product.LongDesc,
				product.LogoURL,
				product.MarkdownContent,
				time.Now(),
				edit.EntityID,
			)

			if err != nil {
				return fmt.Errorf("failed to update product: %w", err)
			}
		}
	case "category":
		// Similar logic for category changes
		if edit.ChangeType == "create" || edit.ChangeType == "update" {
			var category models.Category
			err = json.Unmarshal([]byte(edit.ChangeData), &category)
			if err != nil {
				return fmt.Errorf("failed to unmarshal category data: %w", err)
			}

			if edit.ChangeType == "create" {
				// Categories are auto-approved, so they should already be in the database
				// This is just a safety check
				var count int
				err = tx.QueryRow("SELECT COUNT(*) FROM categories WHERE id = $1", category.ID).Scan(&count)
				if err != nil {
					return fmt.Errorf("failed to check if category exists: %w", err)
				}

				if count == 0 {
					_, err = tx.Exec(`
						INSERT INTO categories (id, name, description, created_at, updated_at)
						VALUES ($1, $2, $3, $4, $5)
					`,
						category.ID,
						category.Name,
						category.Description,
						time.Now(),
						time.Now(),
					)

					if err != nil {
						return fmt.Errorf("failed to create category: %w", err)
					}
				}
			} else {
				// Update the category
				_, err = tx.Exec(`
					UPDATE categories
					SET name = $1, description = $2, updated_at = $3
					WHERE id = $4
				`,
					category.Name,
					category.Description,
					time.Now(),
					category.ID,
				)

				if err != nil {
					return fmt.Errorf("failed to update category: %w", err)
				}
			}
		}
	default:
		return fmt.Errorf("unsupported entity type: %s", edit.EntityType)
	}

	// Update the pending edit status
	_, err = tx.Exec(`
		UPDATE pending_edits
		SET status = 'approved', processed_at = $1
		WHERE id = $2
	`, time.Now(), editID)

	if err != nil {
		return fmt.Errorf("failed to update edit status: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RejectEdit rejects a pending edit
func (r *PostgresRepository) RejectEdit(editID string) error {
	_, err := r.db.Exec(`
		UPDATE pending_edits
		SET status = 'rejected', processed_at = $1
		WHERE id = $2 AND status = 'pending'
	`, time.Now(), editID)

	if err != nil {
		return fmt.Errorf("failed to reject edit: %w", err)
	}

	return nil
}
