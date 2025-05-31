package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
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
	product.CurrentRevisionNumber = 1

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
		INSERT INTO products (
			id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, 
			approved, is_verified, analytics_list, security_score, ux_score, decent_score, vibes_score,
			current_revision_number, last_editor_id, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
		RETURNING id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, 
			approved, is_verified, analytics_list, security_score, ux_score, decent_score, vibes_score,
			current_revision_number, last_editor_id, created_at, updated_at
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
		product.IsVerified,
		pq.Array(product.AnalyticsList),
		product.SecurityScore,
		product.UXScore,
		product.DecentScore,
		product.VibesScore,
		product.CurrentRevisionNumber,
		product.SubmitterID, // last_editor_id is initially the submitter
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
		&product.IsVerified,
		pq.Array(&product.AnalyticsList),
		&product.SecurityScore,
		&product.UXScore,
		&product.DecentScore,
		&product.VibesScore,
		&product.CurrentRevisionNumber,
		&product.LastEditorID,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}

	// Create initial revision
	editSummary := "Initial product version"
	err = r.createProductRevisionTx(tx, product.ID, 1, &product.SubmitterID, &editSummary, nil, product)
	if err != nil {
		return fmt.Errorf("failed to create initial revision: %w", err)
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
		SELECT id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, 
		       approved, is_verified, analytics_list, security_score, ux_score, decent_score, vibes_score,
		       current_revision_number, last_editor_id, created_at, updated_at
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
		&product.IsVerified,
		pq.Array(&product.AnalyticsList),
		&product.SecurityScore,
		&product.UXScore,
		&product.DecentScore,
		&product.VibesScore,
		&product.CurrentRevisionNumber,
		&product.LastEditorID,
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
               p.markdown_content, p.submitter_id, p.approved, p.is_verified, 
               p.analytics_list, p.security_score, p.ux_score, p.decent_score, p.vibes_score,
               p.current_revision_number, p.last_editor_id, p.created_at, p.updated_at
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
			&product.IsVerified,
			pq.Array(&product.AnalyticsList),
			&product.SecurityScore,
			&product.UXScore,
			&product.DecentScore,
			&product.VibesScore,
			&product.CurrentRevisionNumber,
			&product.LastEditorID,
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

			// Set approved flag and revision tracking
			product.Approved = true
			product.CurrentRevisionNumber = 1

			// If entity ID exists, use it; otherwise generate a new one
			if edit.EntityID != "" {
				product.ID = edit.EntityID
			}

			// Insert the product using the main logic
			_, err = tx.Exec(`
				INSERT INTO products (
					id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, 
					approved, is_verified, analytics_list, security_score, ux_score, decent_score, vibes_score,
					current_revision_number, last_editor_id, created_at, updated_at
				)
				VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
				)
			`,
				product.ID,
				product.Title,
				product.ShortDesc,
				product.LongDesc,
				product.LogoURL,
				product.MarkdownContent,
				product.SubmitterID,
				product.Approved,
				product.IsVerified,
				pq.Array(product.AnalyticsList),
				product.SecurityScore,
				product.UXScore,
				product.DecentScore,
				product.VibesScore,
				product.CurrentRevisionNumber,
				product.SubmitterID, // last_editor_id is initially the submitter
				time.Now(),
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to insert approved product: %w", err)
			}

			// Create initial revision
			editSummary := "Initial product version (approved edit)"
			err = r.createProductRevisionTx(tx, product.ID, 1, &product.SubmitterID, &editSummary, nil, &product)
			if err != nil {
				return fmt.Errorf("failed to create initial revision: %w", err)
			}

		} else if edit.ChangeType == "update" {
			// Get current product state for diff calculation
			var currentProduct models.Product
			err = tx.QueryRow(`
				SELECT id, title, short_desc, long_desc, logo_url, markdown_content, submitter_id, 
					   approved, is_verified, analytics_list, security_score, ux_score, decent_score, vibes_score,
					   current_revision_number, last_editor_id, created_at, updated_at
				FROM products WHERE id = $1
			`, edit.EntityID).Scan(
				&currentProduct.ID,
				&currentProduct.Title,
				&currentProduct.ShortDesc,
				&currentProduct.LongDesc,
				&currentProduct.LogoURL,
				&currentProduct.MarkdownContent,
				&currentProduct.SubmitterID,
				&currentProduct.Approved,
				&currentProduct.IsVerified,
				pq.Array(&currentProduct.AnalyticsList),
				&currentProduct.SecurityScore,
				&currentProduct.UXScore,
				&currentProduct.DecentScore,
				&currentProduct.VibesScore,
				&currentProduct.CurrentRevisionNumber,
				&currentProduct.LastEditorID,
				&currentProduct.CreatedAt,
				&currentProduct.UpdatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to get current product: %w", err)
			}

			// Parse the new product data from the change data
			var newProduct models.Product
			err = json.Unmarshal([]byte(edit.ChangeData), &newProduct)
			if err != nil {
				return fmt.Errorf("failed to unmarshal product data: %w", err)
			}

			// Ensure we keep the product ID and mark as approved
			newProduct.ID = edit.EntityID
			newProduct.Approved = true
			newProduct.CurrentRevisionNumber = currentProduct.CurrentRevisionNumber + 1
			newProduct.LastEditorID = &edit.UserID
			newProduct.UpdatedAt = time.Now()

			// Calculate differences for the revision
			changes := r.calculateProductDifferences(&currentProduct, &newProduct)

			// Create edit summary from pending edit or generate default
			editSummary := "Product update (approved edit)"
			if len(changes) > 0 {
				changedFields := make([]string, len(changes))
				for i, change := range changes {
					changedFields[i] = change.FieldName
				}
				editSummary = fmt.Sprintf("Updated %s", strings.Join(changedFields, ", "))
			}

			// Create new revision with transaction
			err = r.createProductRevisionTx(tx, edit.EntityID, newProduct.CurrentRevisionNumber, &edit.UserID, &editSummary, &changes, &newProduct)
			if err != nil {
				return fmt.Errorf("failed to create revision: %w", err)
			}

			// Update the product with new data
			_, err = tx.Exec(`
				UPDATE products
				SET title = $1, short_desc = $2, long_desc = $3, logo_url = $4, markdown_content = $5, 
					is_verified = $6, analytics_list = $7, security_score = $8, ux_score = $9, 
					decent_score = $10, vibes_score = $11, current_revision_number = $12, 
					last_editor_id = $13, updated_at = $14, approved = true
				WHERE id = $15
			`,
				newProduct.Title,
				newProduct.ShortDesc,
				newProduct.LongDesc,
				newProduct.LogoURL,
				newProduct.MarkdownContent,
				newProduct.IsVerified,
				pq.Array(newProduct.AnalyticsList),
				newProduct.SecurityScore,
				newProduct.UXScore,
				newProduct.DecentScore,
				newProduct.VibesScore,
				newProduct.CurrentRevisionNumber,
				edit.UserID,
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

// generateID generates a unique ID for database entities
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// DeleteAllProducts deletes all products from the database
// WARNING: This is a destructive operation and should only be used for testing
func (r *PostgresRepository) DeleteAllProducts() error {
	// Start a transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Delete from related tables first to maintain referential integrity
	// Delete product_categories join table entries
	_, err = tx.Exec("DELETE FROM product_categories")
	if err != nil {
		return fmt.Errorf("failed to delete product categories: %w", err)
	}

	// Delete product_chains join table entries
	_, err = tx.Exec("DELETE FROM product_chains")
	if err != nil {
		return fmt.Errorf("failed to delete product chains: %w", err)
	}

	// Delete upvotes related to products
	_, err = tx.Exec("DELETE FROM upvotes")
	if err != nil {
		return fmt.Errorf("failed to delete upvotes: %w", err)
	}

	// Delete pending edits related to products
	_, err = tx.Exec("DELETE FROM pending_edits WHERE entity_type = 'product'")
	if err != nil {
		return fmt.Errorf("failed to delete pending product edits: %w", err)
	}

	// Finally, delete all products
	_, err = tx.Exec("DELETE FROM products")
	if err != nil {
		return fmt.Errorf("failed to delete products: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CreateProductRevision creates a new revision of a product
func (r *PostgresRepository) CreateProductRevision(productID string, editorID *string, editSummary *string, changes []models.ProductFieldChange, newProductData *models.Product) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Get current revision number
	var currentRevision int
	err = tx.QueryRow("SELECT current_revision_number FROM products WHERE id = $1", productID).Scan(&currentRevision)
	if err != nil {
		return fmt.Errorf("failed to get current revision: %w", err)
	}

	newRevision := currentRevision + 1

	// Create the revision
	err = r.createProductRevisionTx(tx, productID, newRevision, editorID, editSummary, &changes, newProductData)
	if err != nil {
		return fmt.Errorf("failed to create revision: %w", err)
	}

	// Update the product with new data and revision number
	_, err = tx.Exec(`
		UPDATE products
		SET title = $1, short_desc = $2, long_desc = $3, logo_url = $4, markdown_content = $5, 
			is_verified = $6, analytics_list = $7, security_score = $8, ux_score = $9, 
			decent_score = $10, vibes_score = $11, current_revision_number = $12, 
			last_editor_id = $13, updated_at = $14
		WHERE id = $15
	`,
		newProductData.Title,
		newProductData.ShortDesc,
		newProductData.LongDesc,
		newProductData.LogoURL,
		newProductData.MarkdownContent,
		newProductData.IsVerified,
		pq.Array(newProductData.AnalyticsList),
		newProductData.SecurityScore,
		newProductData.UXScore,
		newProductData.DecentScore,
		newProductData.VibesScore,
		newRevision,
		editorID,
		time.Now(),
		productID,
	)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// createProductRevisionTx creates a product revision within a transaction
func (r *PostgresRepository) createProductRevisionTx(tx *sql.Tx, productID string, revisionNumber int, editorID *string, editSummary *string, changes *[]models.ProductFieldChange, productData *models.Product) error {
	// Serialize product data to JSON
	productJSON, err := json.Marshal(productData)
	if err != nil {
		return fmt.Errorf("failed to marshal product data: %w", err)
	}

	// Create diff data if changes are provided
	var diffData []byte
	if changes != nil {
		diffData, err = json.Marshal(changes)
		if err != nil {
			return fmt.Errorf("failed to marshal diff data: %w", err)
		}
	}

	// Insert revision
	revisionID := generateID()
	_, err = tx.Exec(`
		INSERT INTO product_revisions (id, product_id, revision_number, editor_id, edit_summary, diff_data, product_data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		revisionID, productID, revisionNumber, editorID, editSummary, diffData, productJSON, time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to insert revision: %w", err)
	}

	// Insert field changes if provided
	if changes != nil {
		for _, change := range *changes {
			_, err = tx.Exec(`
				INSERT INTO product_field_changes (id, revision_id, field_name, old_value, new_value, change_type)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
				generateID(), revisionID, change.FieldName, change.OldValue, change.NewValue, change.ChangeType,
			)
			if err != nil {
				return fmt.Errorf("failed to insert field change: %w", err)
			}
		}
	}

	return nil
}

// GetProductRevisions returns the revision history for a product
func (r *PostgresRepository) GetProductRevisions(productID string, page, perPage int) ([]models.RevisionSummary, int, error) {
	offset := (page - 1) * perPage

	// Get total count
	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM product_revisions WHERE product_id = $1", productID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get revision count: %w", err)
	}

	// Get revisions with editor info
	query := `
		SELECT pr.revision_number, pr.edit_summary, pr.editor_id, pr.created_at,
			   u.wallet_address, u.twitter_handle,
			   COALESCE((SELECT COUNT(*) FROM product_field_changes pfc WHERE pfc.revision_id = pr.id), 0) as change_count
		FROM product_revisions pr
		LEFT JOIN users u ON pr.editor_id = u.id
		WHERE pr.product_id = $1
		ORDER BY pr.revision_number DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(query, productID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get revisions: %w", err)
	}
	defer rows.Close()

	var revisions []models.RevisionSummary
	for rows.Next() {
		var rev models.RevisionSummary
		var walletAddress, twitterHandle sql.NullString

		err := rows.Scan(
			&rev.RevisionNumber,
			&rev.EditSummary,
			&rev.EditorID,
			&rev.CreatedAt,
			&walletAddress,
			&twitterHandle,
			&rev.ChangeCount,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan revision: %w", err)
		}

		// Add editor info if available
		if walletAddress.Valid {
			rev.Editor = &models.User{
				ID:            *rev.EditorID,
				WalletAddress: walletAddress.String,
				TwitterHandle: twitterHandle.String,
			}
		}

		// Determine if this is a major change (more than 2 field changes)
		rev.MajorChange = rev.ChangeCount > 2

		revisions = append(revisions, rev)
	}

	return revisions, total, nil
}

// GetProductRevision returns a specific revision of a product
func (r *PostgresRepository) GetProductRevision(productID string, revisionNumber int) (*models.ProductRevision, error) {
	query := `
		SELECT pr.id, pr.product_id, pr.revision_number, pr.editor_id, pr.edit_summary, 
			   pr.diff_data, pr.product_data, pr.created_at,
			   u.wallet_address, u.twitter_handle
		FROM product_revisions pr
		LEFT JOIN users u ON pr.editor_id = u.id
		WHERE pr.product_id = $1 AND pr.revision_number = $2
	`

	var revision models.ProductRevision
	var walletAddress, twitterHandle sql.NullString

	err := r.db.QueryRow(query, productID, revisionNumber).Scan(
		&revision.ID,
		&revision.ProductID,
		&revision.RevisionNumber,
		&revision.EditorID,
		&revision.EditSummary,
		&revision.DiffData,
		&revision.ProductData,
		&revision.CreatedAt,
		&walletAddress,
		&twitterHandle,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("revision not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	// Add editor info if available
	if walletAddress.Valid {
		revision.Editor = &models.User{
			ID:            *revision.EditorID,
			WalletAddress: walletAddress.String,
			TwitterHandle: twitterHandle.String,
		}
	}

	// Load field changes
	fieldChanges, err := r.getRevisionFieldChanges(revision.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load field changes: %w", err)
	}
	revision.FieldChanges = fieldChanges

	return &revision, nil
}

// getRevisionFieldChanges loads field changes for a revision
func (r *PostgresRepository) getRevisionFieldChanges(revisionID string) ([]models.ProductFieldChange, error) {
	query := `
		SELECT id, revision_id, field_name, old_value, new_value, change_type
		FROM product_field_changes
		WHERE revision_id = $1
		ORDER BY field_name
	`

	rows, err := r.db.Query(query, revisionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query field changes: %w", err)
	}
	defer rows.Close()

	var changes []models.ProductFieldChange
	for rows.Next() {
		var change models.ProductFieldChange
		err := rows.Scan(
			&change.ID,
			&change.RevisionID,
			&change.FieldName,
			&change.OldValue,
			&change.NewValue,
			&change.ChangeType,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan field change: %w", err)
		}
		changes = append(changes, change)
	}

	return changes, nil
}

// CompareProductRevisions compares two revisions and returns the differences
func (r *PostgresRepository) CompareProductRevisions(productID string, fromRevision, toRevision int) (*models.ProductDiff, error) {
	// Get both revisions
	fromRev, err := r.GetProductRevision(productID, fromRevision)
	if err != nil {
		return nil, fmt.Errorf("failed to get from revision: %w", err)
	}

	toRev, err := r.GetProductRevision(productID, toRevision)
	if err != nil {
		return nil, fmt.Errorf("failed to get to revision: %w", err)
	}

	// Parse product data from both revisions
	var fromProduct, toProduct models.Product
	err = json.Unmarshal(fromRev.ProductData, &fromProduct)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal from product: %w", err)
	}

	err = json.Unmarshal(toRev.ProductData, &toProduct)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to product: %w", err)
	}

	// Calculate differences
	changes := r.calculateProductDifferences(&fromProduct, &toProduct)

	diff := &models.ProductDiff{
		FromRevision: fromRevision,
		ToRevision:   toRevision,
		Changes:      changes,
		Summary:      fmt.Sprintf("%d field(s) changed", len(changes)),
	}

	return diff, nil
}

// calculateProductDifferences compares two products and returns the differences
func (r *PostgresRepository) calculateProductDifferences(from, to *models.Product) []models.ProductFieldChange {
	var changes []models.ProductFieldChange

	// Helper function to add change if values differ
	addChange := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			changeType := "modified"
			if oldVal == "" {
				changeType = "added"
			} else if newVal == "" {
				changeType = "removed"
			}

			changes = append(changes, models.ProductFieldChange{
				FieldName:  field,
				OldValue:   &oldVal,
				NewValue:   &newVal,
				ChangeType: changeType,
			})
		}
	}

	// Compare all fields
	addChange("title", from.Title, to.Title)
	addChange("short_desc", from.ShortDesc, to.ShortDesc)
	addChange("long_desc", from.LongDesc, to.LongDesc)
	addChange("logo_url", from.LogoURL, to.LogoURL)
	addChange("markdown_content", from.MarkdownContent, to.MarkdownContent)
	addChange("security_score", fmt.Sprintf("%.2f", from.SecurityScore), fmt.Sprintf("%.2f", to.SecurityScore))
	addChange("ux_score", fmt.Sprintf("%.2f", from.UXScore), fmt.Sprintf("%.2f", to.UXScore))
	addChange("decent_score", fmt.Sprintf("%.2f", from.DecentScore), fmt.Sprintf("%.2f", to.DecentScore))
	addChange("vibes_score", fmt.Sprintf("%.2f", from.VibesScore), fmt.Sprintf("%.2f", to.VibesScore))

	// Compare boolean fields
	addChange("is_verified", fmt.Sprintf("%t", from.IsVerified), fmt.Sprintf("%t", to.IsVerified))
	addChange("approved", fmt.Sprintf("%t", from.Approved), fmt.Sprintf("%t", to.Approved))

	// Compare arrays (simplified comparison)
	fromAnalytics, _ := json.Marshal(from.AnalyticsList)
	toAnalytics, _ := json.Marshal(to.AnalyticsList)
	addChange("analytics_list", string(fromAnalytics), string(toAnalytics))

	return changes
}

// RevertProductToRevision reverts a product to a specific revision
func (r *PostgresRepository) RevertProductToRevision(productID string, revisionNumber int, editorID *string, reason string) error {
	// Get the target revision
	targetRevision, err := r.GetProductRevision(productID, revisionNumber)
	if err != nil {
		return fmt.Errorf("failed to get target revision: %w", err)
	}

	// Parse the product data from the target revision
	var targetProduct models.Product
	err = json.Unmarshal(targetRevision.ProductData, &targetProduct)
	if err != nil {
		return fmt.Errorf("failed to unmarshal target product: %w", err)
	}

	// Get current product state
	currentProduct, err := r.GetProductByID(productID)
	if err != nil {
		return fmt.Errorf("failed to get current product: %w", err)
	}

	// Calculate changes (from current to target)
	changes := r.calculateProductDifferences(currentProduct, &targetProduct)

	// Create edit summary
	editSummary := fmt.Sprintf("Reverted to revision %d: %s", revisionNumber, reason)

	// Create new revision with reverted data
	err = r.CreateProductRevision(productID, editorID, &editSummary, changes, &targetProduct)
	if err != nil {
		return fmt.Errorf("failed to create revert revision: %w", err)
	}

	return nil
}

// GetRecentEdits returns recent product edits across all products
func (r *PostgresRepository) GetRecentEdits(limit int) ([]models.RevisionSummary, error) {
	query := `
		SELECT pr.product_id, pr.revision_number, pr.edit_summary, pr.editor_id, pr.created_at,
			   u.wallet_address, u.twitter_handle, p.title,
			   COALESCE((SELECT COUNT(*) FROM product_field_changes pfc WHERE pfc.revision_id = pr.id), 0) as change_count
		FROM product_revisions pr
		LEFT JOIN users u ON pr.editor_id = u.id
		LEFT JOIN products p ON pr.product_id = p.id
		ORDER BY pr.created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent edits: %w", err)
	}
	defer rows.Close()

	var edits []models.RevisionSummary
	for rows.Next() {
		var edit models.RevisionSummary
		var walletAddress, twitterHandle, productTitle sql.NullString
		var productID string

		err := rows.Scan(
			&productID,
			&edit.RevisionNumber,
			&edit.EditSummary,
			&edit.EditorID,
			&edit.CreatedAt,
			&walletAddress,
			&twitterHandle,
			&productTitle,
			&edit.ChangeCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recent edit: %w", err)
		}

		// Add editor info if available
		if walletAddress.Valid {
			edit.Editor = &models.User{
				ID:            *edit.EditorID,
				WalletAddress: walletAddress.String,
				TwitterHandle: twitterHandle.String,
			}
		}

		edit.MajorChange = edit.ChangeCount > 2
		edits = append(edits, edit)
	}

	return edits, nil
}

// UpdateProduct updates an existing product in the database
func (r *PostgresRepository) UpdateProduct(product *models.Product) error {
	// Update the product record
	query := `
		UPDATE products 
		SET title = $2, short_desc = $3, long_desc = $4, logo_url = $5, 
		    markdown_content = $6, approved = $7, is_verified = $8, 
		    analytics_list = $9, security_score = $10, ux_score = $11, 
		    decent_score = $12, vibes_score = $13, current_revision_number = $14,
		    last_editor_id = $15, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(
		query,
		product.ID,
		product.Title,
		product.ShortDesc,
		product.LongDesc,
		product.LogoURL,
		product.MarkdownContent,
		product.Approved,
		product.IsVerified,
		pq.Array(product.AnalyticsList),
		product.SecurityScore,
		product.UXScore,
		product.DecentScore,
		product.VibesScore,
		product.CurrentRevisionNumber,
		product.LastEditorID,
	)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	return nil
}
