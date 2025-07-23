package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

	repo := &PostgresRepository{
		db:  db,
		cfg: cfg,
	}

	// Run database migrations automatically
	if err := repo.RunMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	return repo, nil
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
			id, title, short_desc, long_desc, logo_url, website_url, github_url, docs_url, audit_reports,
			markdown_content, submitter_id, approved, is_verified, analytics_list, overall_score, 
			security_score, ux_score, vibes_score, current_revision_number, last_editor_id, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		RETURNING id, title, short_desc, long_desc, logo_url, website_url, github_url, docs_url, audit_reports,
			markdown_content, submitter_id, approved, is_verified, analytics_list, overall_score, 
			security_score, ux_score, vibes_score, current_revision_number, last_editor_id, created_at, updated_at
	`

	err = tx.QueryRow(
		query,
		product.ID,
		product.Title,
		product.ShortDesc,
		product.LongDesc,
		product.LogoURL,
		product.WebsiteURL,
		product.GitHubURL,
		product.DocsURL,
		pq.Array(product.AuditReports),
		product.MarkdownContent,
		product.SubmitterID,
		product.Approved,
		product.IsVerified,
		pq.Array(product.AnalyticsList),
		product.OverallScore,
		product.SecurityScore,
		product.UXScore,
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
		&product.WebsiteURL,
		&product.GitHubURL,
		&product.DocsURL,
		pq.Array(&product.AuditReports),
		&product.MarkdownContent,
		&product.SubmitterID,
		&product.Approved,
		&product.IsVerified,
		pq.Array(&product.AnalyticsList),
		&product.OverallScore,
		&product.SecurityScore,
		&product.UXScore,
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
	log.Printf("CreateProduct: Processing %d categories for product %s", len(product.Categories), product.ID)
	if len(product.Categories) > 0 {
		for i, category := range product.Categories {
			log.Printf("CreateProduct: Processing category %d: '%s'", i+1, category.Name)

			// Skip empty or invalid category names
			if strings.TrimSpace(category.Name) == "" {
				log.Printf("CreateProduct: Skipping empty category name at index %d", i+1)
				continue
			}

			// Ensure category exists (create if new) and get its ID
			categoryID, err := r.ensureCategoryExists(tx, category.Name)
			if err != nil {
				log.Printf("CreateProduct: FAILED to ensure category '%s' exists: %v", category.Name, err)
				return fmt.Errorf("failed to ensure category exists: %w", err)
			}
			log.Printf("CreateProduct: Category '%s' resolved to ID: %s", category.Name, categoryID)

			_, err = tx.Exec(
				"INSERT INTO product_categories (product_id, category_id) VALUES ($1, $2)",
				product.ID, categoryID,
			)
			if err != nil {
				log.Printf("CreateProduct: FAILED to link product %s to category %s: %v", product.ID, categoryID, err)
				return fmt.Errorf("failed to link product to category: %w", err)
			}
			log.Printf("CreateProduct: Successfully linked product %s to category %s", product.ID, categoryID)
		}
	} else {
		log.Printf("CreateProduct: No categories to process for product %s", product.ID)
	}

	// Insert chain relationships
	log.Printf("CreateProduct: Processing %d chains for product %s", len(product.Chains), product.ID)
	if len(product.Chains) > 0 {
		for i, chain := range product.Chains {
			log.Printf("CreateProduct: Processing chain %d: '%s'", i+1, chain.Name)

			// Skip empty or invalid chain names
			if strings.TrimSpace(chain.Name) == "" {
				log.Printf("CreateProduct: Skipping empty chain name at index %d", i+1)
				continue
			}

			// Ensure chain exists (create if new) and get its ID
			chainID, err := r.ensureChainExists(tx, chain.Name)
			if err != nil {
				log.Printf("CreateProduct: FAILED to ensure chain '%s' exists: %v", chain.Name, err)
				return fmt.Errorf("failed to ensure chain exists: %w", err)
			}
			log.Printf("CreateProduct: Chain '%s' resolved to ID: %s", chain.Name, chainID)

			_, err = tx.Exec(
				"INSERT INTO product_chains (product_id, chain_id) VALUES ($1, $2)",
				product.ID, chainID,
			)
			if err != nil {
				log.Printf("CreateProduct: FAILED to link product %s to chain %s: %v", product.ID, chainID, err)
				return fmt.Errorf("failed to link product to chain: %w", err)
			}
			log.Printf("CreateProduct: Successfully linked product %s to chain %s", product.ID, chainID)
		}
	} else {
		log.Printf("CreateProduct: No chains to process for product %s", product.ID)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetProductByID gets an APPROVED product by its ID (public API)
func (r *PostgresRepository) GetProductByID(id string) (*models.Product, error) {
	query := `
		SELECT id, title, short_desc, long_desc, logo_url, website_url, github_url, docs_url, audit_reports,
		       markdown_content, submitter_id, approved, is_verified, analytics_list, overall_score, 
		       security_score, ux_score, vibes_score, current_revision_number, last_editor_id, created_at, updated_at
		FROM products
		WHERE id = $1 AND approved = true
	`

	product := &models.Product{}
	err := r.db.QueryRow(query, id).Scan(
		&product.ID,
		&product.Title,
		&product.ShortDesc,
		&product.LongDesc,
		&product.LogoURL,
		&product.WebsiteURL,
		&product.GitHubURL,
		&product.DocsURL,
		pq.Array(&product.AuditReports),
		&product.MarkdownContent,
		&product.SubmitterID,
		&product.Approved,
		&product.IsVerified,
		pq.Array(&product.AnalyticsList),
		&product.OverallScore,
		&product.SecurityScore,
		&product.UXScore,
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

	// Load categories and chains for approved products
	if err = r.loadProductCategories(product); err != nil {
		return nil, err
	}

	if err = r.loadProductChains(product); err != nil {
		return nil, err
	}

	// Load submitter
	if err = r.loadProductSubmitter(product); err != nil {
		return nil, err
	}

	// Load last editor
	if err = r.loadProductLastEditor(product); err != nil {
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

// GetProductByIDAdmin gets ANY product by its ID (for internal use - not for API endpoints)
func (r *PostgresRepository) GetProductByIDAdmin(id string) (*models.Product, error) {
	query := `
		SELECT id, title, short_desc, long_desc, logo_url, website_url, github_url, docs_url, audit_reports,
		       markdown_content, submitter_id, approved, is_verified, analytics_list, overall_score, 
		       security_score, ux_score, vibes_score, current_revision_number, last_editor_id, created_at, updated_at
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
		&product.WebsiteURL,
		&product.GitHubURL,
		&product.DocsURL,
		pq.Array(&product.AuditReports),
		&product.MarkdownContent,
		&product.SubmitterID,
		&product.Approved,
		&product.IsVerified,
		pq.Array(&product.AnalyticsList),
		&product.OverallScore,
		&product.SecurityScore,
		&product.UXScore,
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

	// Always load categories and chains for admin view
	if err = r.loadProductCategories(product); err != nil {
		return nil, err
	}

	if err = r.loadProductChains(product); err != nil {
		return nil, err
	}

	// Load submitter
	if err = r.loadProductSubmitter(product); err != nil {
		return nil, err
	}

	// Load last editor
	if err = r.loadProductLastEditor(product); err != nil {
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
		SELECT p.id, p.title, p.short_desc, p.long_desc, p.logo_url, p.website_url, p.github_url, 
               p.docs_url, p.audit_reports, p.markdown_content, p.submitter_id, p.approved, p.is_verified, 
               p.analytics_list, p.security_score, p.ux_score, p.overall_score, p.vibes_score,
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

	// Note: Complex sorting by vote count temporarily simplified to sort by date
	// TODO: Implement proper vote-based sorting later

	// Add where clause to queries
	countQuery += " " + whereClause
	query += " " + whereClause

	// Add sorting - temporarily all sort by newest until we implement proper vote counting
	query += " ORDER BY p.created_at DESC"

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
			&product.WebsiteURL,
			&product.GitHubURL,
			&product.DocsURL,
			pq.Array(&product.AuditReports),
			&product.MarkdownContent,
			&product.SubmitterID,
			&product.Approved,
			&product.IsVerified,
			pq.Array(&product.AnalyticsList),
			&product.SecurityScore,
			&product.UXScore,
			&product.OverallScore,
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

		// Load submitter
		if err = r.loadProductSubmitter(product); err != nil {
			return nil, 0, err
		}

		// Load last editor
		if err = r.loadProductLastEditor(product); err != nil {
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
	// Always initialize as empty slice to ensure it's never nil
	product.Categories = []models.Category{}

	query := `
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at
		FROM categories c
		JOIN product_categories pc ON c.id = pc.category_id
		WHERE pc.product_id = $1
	`

	rows, err := r.db.Query(query, product.ID)
	if err != nil {
		log.Printf("loadProductCategories: Query failed for product %s: %v", product.ID, err)
		// Don't return error for missing relationships, just keep empty slice
		return nil
	}
	defer rows.Close()

	categoryCount := 0
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
			log.Printf("loadProductCategories: Failed to scan category for product %s: %v", product.ID, err)
			return fmt.Errorf("failed to scan category: %w", err)
		}
		product.Categories = append(product.Categories, category)
		categoryCount++
	}

	log.Printf("loadProductCategories: Loaded %d categories for product %s", categoryCount, product.ID)
	return nil
}

func (r *PostgresRepository) loadProductChains(product *models.Product) error {
	// Always initialize as empty slice to ensure it's never nil
	product.Chains = []models.Chain{}

	query := `
		SELECT c.id, c.name, c.icon, c.created_at, c.updated_at
		FROM chains c
		JOIN product_chains pc ON c.id = pc.chain_id
		WHERE pc.product_id = $1
	`

	rows, err := r.db.Query(query, product.ID)
	if err != nil {
		log.Printf("loadProductChains: Query failed for product %s: %v", product.ID, err)
		// Don't return error for missing relationships, just keep empty slice
		return nil
	}
	defer rows.Close()

	chainCount := 0
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
			log.Printf("loadProductChains: Failed to scan chain for product %s: %v", product.ID, err)
			return fmt.Errorf("failed to scan chain: %w", err)
		}
		product.Chains = append(product.Chains, chain)
		chainCount++
	}

	log.Printf("loadProductChains: Loaded %d chains for product %s", chainCount, product.ID)
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

// loadProductSubmitter loads the submitter user information for a product
func (r *PostgresRepository) loadProductSubmitter(product *models.Product) error {
	if product.SubmitterID == "" {
		return nil // No submitter to load
	}

	query := `
		SELECT id, wallet_address, twitter_handle, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	submitter := &models.User{}
	err := r.db.QueryRow(query, product.SubmitterID).Scan(
		&submitter.ID,
		&submitter.WalletAddress,
		&submitter.TwitterHandle,
		&submitter.CreatedAt,
		&submitter.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Submitter not found, but don't fail the product load
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load product submitter: %w", err)
	}

	product.Submitter = submitter
	return nil
}

// loadProductLastEditor loads the last editor user information for a product
func (r *PostgresRepository) loadProductLastEditor(product *models.Product) error {
	if product.LastEditorID == nil || *product.LastEditorID == "" {
		return nil // No last editor to load
	}

	query := `
		SELECT id, wallet_address, twitter_handle, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	lastEditor := &models.User{}
	err := r.db.QueryRow(query, *product.LastEditorID).Scan(
		&lastEditor.ID,
		&lastEditor.WalletAddress,
		&lastEditor.TwitterHandle,
		&lastEditor.CreatedAt,
		&lastEditor.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Last editor not found, but don't fail the product load
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load product last editor: %w", err)
	}

	product.LastEditor = lastEditor
	return nil
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

// GetChains returns all chains
func (r *PostgresRepository) GetChains() ([]models.Chain, error) {
	query := `SELECT id, name, icon, created_at, updated_at FROM chains ORDER BY name ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get chains: %w", err)
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
			return nil, fmt.Errorf("failed to scan chain: %w", err)
		}

		chains = append(chains, chain)
	}

	return chains, nil
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

// generateID generates a unique ID for database entities
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ensureCategoryExists creates a category if it doesn't exist, returns the category ID
func (r *PostgresRepository) ensureCategoryExists(tx *sql.Tx, categoryName string) (string, error) {
	log.Printf("ensureCategoryExists: Checking for category '%s'", categoryName)

	// Check if category already exists by name (case insensitive)
	var existingID string
	err := tx.QueryRow("SELECT id FROM categories WHERE LOWER(name) = LOWER($1)", categoryName).Scan(&existingID)
	if err == nil {
		// Category exists, return its ID
		log.Printf("ensureCategoryExists: Found existing category '%s' with ID: %s", categoryName, existingID)
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		log.Printf("ensureCategoryExists: Database error while checking category '%s': %v", categoryName, err)
		return "", fmt.Errorf("failed to check category existence: %w", err)
	}

	log.Printf("ensureCategoryExists: Category '%s' doesn't exist, creating new one", categoryName)

	// Category doesn't exist, create it using name as ID
	_, err = tx.Exec(`
		INSERT INTO categories (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, categoryName, categoryName, fmt.Sprintf("Auto-generated category for %s", categoryName), time.Now(), time.Now())

	if err != nil {
		log.Printf("ensureCategoryExists: FAILED to create category '%s': %v", categoryName, err)
		return "", fmt.Errorf("failed to create category: %w", err)
	}

	log.Printf("ensureCategoryExists: Successfully created new category '%s'", categoryName)
	return categoryName, nil
}

// ensureChainExists creates a chain if it doesn't exist, returns the chain ID
func (r *PostgresRepository) ensureChainExists(tx *sql.Tx, chainName string) (string, error) {
	log.Printf("ensureChainExists: Checking for chain '%s'", chainName)

	// Check if chain already exists by name (case insensitive)
	var existingID string
	err := tx.QueryRow("SELECT id FROM chains WHERE LOWER(name) = LOWER($1)", chainName).Scan(&existingID)
	if err == nil {
		// Chain exists, return its ID
		log.Printf("ensureChainExists: Found existing chain '%s' with ID: %s", chainName, existingID)
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		log.Printf("ensureChainExists: Database error while checking chain '%s': %v", chainName, err)
		return "", fmt.Errorf("failed to check chain existence: %w", err)
	}

	log.Printf("ensureChainExists: Chain '%s' doesn't exist, creating new one", chainName)

	// Chain doesn't exist, create it using name as ID
	_, err = tx.Exec(`
		INSERT INTO chains (id, name, icon, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, chainName, chainName, "", time.Now(), time.Now())

	if err != nil {
		log.Printf("ensureChainExists: FAILED to create chain '%s': %v", chainName, err)
		return "", fmt.Errorf("failed to create chain: %w", err)
	}

	log.Printf("ensureChainExists: Successfully created new chain '%s'", chainName)
	return chainName, nil
}

// DeleteAllProducts deletes all products and related data from the database
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

	log.Printf("DeleteAllProducts: Starting complete cleanup of product data...")

	// Delete in order to respect foreign key constraints

	// 1. Delete product field changes (references product_revisions)
	result, err := tx.Exec("DELETE FROM product_field_changes")
	if err != nil {
		return fmt.Errorf("failed to delete product field changes: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	log.Printf("DeleteAllProducts: Deleted %d product field changes", rowsAffected)

	// 2. Delete product revisions (references products)
	result, err = tx.Exec("DELETE FROM product_revisions")
	if err != nil {
		return fmt.Errorf("failed to delete product revisions: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("DeleteAllProducts: Deleted %d product revisions", rowsAffected)

	// 3. Delete product categories junction table
	result, err = tx.Exec("DELETE FROM product_categories")
	if err != nil {
		return fmt.Errorf("failed to delete product categories: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("DeleteAllProducts: Deleted %d product-category relationships", rowsAffected)

	// 4. Delete product chains junction table
	result, err = tx.Exec("DELETE FROM product_chains")
	if err != nil {
		return fmt.Errorf("failed to delete product chains: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("DeleteAllProducts: Deleted %d product-chain relationships", rowsAffected)

	// 5. Delete upvotes (references products)
	result, err = tx.Exec("DELETE FROM upvotes")
	if err != nil {
		return fmt.Errorf("failed to delete upvotes: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("DeleteAllProducts: Deleted %d upvotes", rowsAffected)

	// 6. Finally, delete all products
	result, err = tx.Exec("DELETE FROM products")
	if err != nil {
		return fmt.Errorf("failed to delete products: %w", err)
	}
	rowsAffected, _ = result.RowsAffected()
	log.Printf("DeleteAllProducts: Deleted %d products", rowsAffected)

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("DeleteAllProducts: Complete cleanup successful - database is now clean!")
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
		SET title = $1, short_desc = $2, long_desc = $3, logo_url = $4, website_url = $5,
			github_url = $6, docs_url = $7, audit_reports = $8, markdown_content = $9, 
			is_verified = $10, analytics_list = $11, security_score = $12, ux_score = $13, 
			overall_score = $14, vibes_score = $15, current_revision_number = $16, 
			last_editor_id = $17, updated_at = $18
		WHERE id = $19
	`,
		newProductData.Title,
		newProductData.ShortDesc,
		newProductData.LongDesc,
		newProductData.LogoURL,
		newProductData.WebsiteURL,
		newProductData.GitHubURL,
		newProductData.DocsURL,
		pq.Array(newProductData.AuditReports),
		newProductData.MarkdownContent,
		newProductData.IsVerified,
		pq.Array(newProductData.AnalyticsList),
		newProductData.SecurityScore,
		newProductData.UXScore,
		newProductData.OverallScore,
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

	// Handle diff data properly - use NULL if no changes
	var diffData interface{}
	if changes != nil && len(*changes) > 0 {
		diffDataBytes, err := json.Marshal(changes)
		if err != nil {
			return fmt.Errorf("failed to marshal diff data: %w", err)
		}
		diffData = string(diffDataBytes)
	} else {
		diffData = nil // This will be inserted as NULL
	}

	// Insert revision - let the database generate the ID with DEFAULT gen_random_uuid()
	var revisionID string
	err = tx.QueryRow(`
		INSERT INTO product_revisions (product_id, revision_number, editor_id, edit_summary, diff_data, product_data)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`,
		productID, revisionNumber, editorID, editSummary, diffData, string(productJSON),
	).Scan(&revisionID)

	if err != nil {
		return fmt.Errorf("failed to insert revision: %w", err)
	}

	// Insert field changes if provided
	if changes != nil && len(*changes) > 0 {
		for _, change := range *changes {
			_, err = tx.Exec(`
				INSERT INTO product_field_changes (revision_id, field_name, old_value, new_value, change_type)
				VALUES ($1, $2, $3, $4, $5)
			`,
				revisionID, change.FieldName, change.OldValue, change.NewValue, change.ChangeType,
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

	// Helper function to safely dereference string pointers
	safeString := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	// Compare all fields
	addChange("title", from.Title, to.Title)
	addChange("short_desc", from.ShortDesc, to.ShortDesc)
	addChange("long_desc", from.LongDesc, to.LongDesc)
	addChange("logo_url", from.LogoURL, to.LogoURL)
	addChange("website_url", safeString(from.WebsiteURL), safeString(to.WebsiteURL))
	addChange("github_url", safeString(from.GitHubURL), safeString(to.GitHubURL))
	addChange("docs_url", safeString(from.DocsURL), safeString(to.DocsURL))
	addChange("markdown_content", from.MarkdownContent, to.MarkdownContent)
	addChange("security_score", fmt.Sprintf("%.2f", from.SecurityScore), fmt.Sprintf("%.2f", to.SecurityScore))
	addChange("ux_score", fmt.Sprintf("%.2f", from.UXScore), fmt.Sprintf("%.2f", to.UXScore))
	addChange("overall_score", fmt.Sprintf("%.2f", from.OverallScore), fmt.Sprintf("%.2f", to.OverallScore))
	addChange("vibes_score", fmt.Sprintf("%.2f", from.VibesScore), fmt.Sprintf("%.2f", to.VibesScore))

	// Compare boolean fields
	addChange("is_verified", fmt.Sprintf("%t", from.IsVerified), fmt.Sprintf("%t", to.IsVerified))
	addChange("approved", fmt.Sprintf("%t", from.Approved), fmt.Sprintf("%t", to.Approved))

	// Compare arrays (simplified comparison)
	fromAnalytics, _ := json.Marshal(from.AnalyticsList)
	toAnalytics, _ := json.Marshal(to.AnalyticsList)
	addChange("analytics_list", string(fromAnalytics), string(toAnalytics))

	fromAuditReports, _ := json.Marshal(from.AuditReports)
	toAuditReports, _ := json.Marshal(to.AuditReports)
	addChange("audit_reports", string(fromAuditReports), string(toAuditReports))

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
		SET title = $2, short_desc = $3, long_desc = $4, logo_url = $5, website_url = $6,
		    github_url = $7, docs_url = $8, audit_reports = $9, markdown_content = $10, 
		    approved = $11, is_verified = $12, analytics_list = $13, security_score = $14, 
		    ux_score = $15, overall_score = $16, vibes_score = $17, current_revision_number = $18,
		    last_editor_id = $19, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(
		query,
		product.ID,
		product.Title,
		product.ShortDesc,
		product.LongDesc,
		product.LogoURL,
		product.WebsiteURL,
		product.GitHubURL,
		product.DocsURL,
		pq.Array(product.AuditReports),
		product.MarkdownContent,
		product.Approved,
		product.IsVerified,
		pq.Array(product.AnalyticsList),
		product.SecurityScore,
		product.UXScore,
		product.OverallScore,
		product.VibesScore,
		product.CurrentRevisionNumber,
		product.LastEditorID,
	)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	return nil
}
