package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nedpals/supabase-go"

	"github.com/wesjorgensen/EthAppList/backend/internal/config"
	"github.com/wesjorgensen/EthAppList/backend/internal/models"
)

// Repository handles all database interactions
type Repository struct {
	client *supabase.Client
	cfg    *config.Config
}

// New creates a new repository instance
func New(cfg *config.Config) (*Repository, error) {
	client := supabase.CreateClient(cfg.SupabaseURL, cfg.SupabaseKey)
	if client == nil {
		return nil, errors.New("failed to create Supabase client")
	}

	return &Repository{
		client: client,
		cfg:    cfg,
	}, nil
}

// CreateUser creates a new user in the database
func (r *Repository) CreateUser(user *models.User) error {
	if user.ID == "" {
		user.ID = generateID()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	var result []models.User
	err := r.client.DB.From("users").Insert(user).Execute(&result)
	if err != nil {
		return err
	}

	if len(result) > 0 {
		*user = result[0]
	}

	return nil
}

// GetUserByWallet gets a user by their wallet address
func (r *Repository) GetUserByWallet(walletAddress string) (*models.User, error) {
	var users []models.User
	err := r.client.DB.From("users").Select("*").Eq("wallet_address", walletAddress).Execute(&users)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, errors.New("user not found")
	}

	return &users[0], nil
}

// CreateProduct creates a new product
func (r *Repository) CreateProduct(product *models.Product) error {
	if product.ID == "" {
		product.ID = generateID()
	}
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()
	product.Approved = false // Start as unapproved

	var result []models.Product
	err := r.client.DB.From("products").Insert(product).Execute(&result)
	if err != nil {
		return err
	}

	if len(result) > 0 {
		*product = result[0]
	}

	// Handle categories and chains separately through junction tables
	if len(product.Categories) > 0 {
		for _, category := range product.Categories {
			err = r.addProductCategory(product.ID, category.ID)
			if err != nil {
				return err
			}
		}
	}

	if len(product.Chains) > 0 {
		for _, chain := range product.Chains {
			err = r.addProductChain(product.ID, chain.ID)
			if err != nil {
				return err
			}
		}
	}

	// Create a pending edit for admin approval
	pendingEdit := &models.PendingEdit{
		ID:         generateID(),
		UserID:     product.SubmitterID,
		EntityType: "product",
		EntityID:   product.ID,
		ChangeType: "create",
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	// Convert product to JSON for the change data
	changeData, err := json.Marshal(product)
	if err != nil {
		return err
	}

	pendingEdit.ChangeData = string(changeData)

	var pendingResult []models.PendingEdit
	err = r.client.DB.From("pending_edits").Insert(pendingEdit).Execute(&pendingResult)
	if err != nil {
		return err
	}

	return nil
}

// GetProducts gets products with filtering options
func (r *Repository) GetProducts(categoryID, chainID, searchTerm, sortOption string, page, perPage int) ([]*models.Product, int, error) {
	var products []models.Product

	query := r.client.DB.From("products").Select("*").Eq("approved", "true")

	// Apply category filter if provided
	if categoryID != "" {
		// First get product IDs from the junction table
		var junctions []struct {
			ProductID  string `json:"product_id"`
			CategoryID string `json:"category_id"`
		}

		err := r.client.DB.From("product_categories").
			Select("product_id").
			Eq("category_id", categoryID).
			Execute(&junctions)

		if err != nil {
			return nil, 0, err
		}

		// Use In operator to filter products by IDs (if supported)
		// This is a simplification, as Supabase client may not directly support this
		// In a real app, you'd need to handle this differently

		// For now, we'll fetch all products and filter manually
		err = query.Execute(&products)
		if err != nil {
			return nil, 0, err
		}

		// Manual filtering based on category
		filteredProducts := []models.Product{}
		for _, product := range products {
			for _, junction := range junctions {
				if product.ID == junction.ProductID {
					filteredProducts = append(filteredProducts, product)
					break
				}
			}
		}

		products = filteredProducts
	} else {
		// Fetch all products if no category filter
		err := query.Execute(&products)
		if err != nil {
			return nil, 0, err
		}
	}

	// Apply chain filter if provided (similar logic to category filter)
	if chainID != "" && len(products) > 0 {
		var junctions []struct {
			ProductID string `json:"product_id"`
			ChainID   string `json:"chain_id"`
		}

		err := r.client.DB.From("product_chains").
			Select("product_id").
			Eq("chain_id", chainID).
			Execute(&junctions)

		if err != nil {
			return nil, 0, err
		}

		// Manual filtering based on chain
		filteredProducts := []models.Product{}
		for _, product := range products {
			for _, junction := range junctions {
				if product.ID == junction.ProductID {
					filteredProducts = append(filteredProducts, product)
					break
				}
			}
		}

		products = filteredProducts
	}

	// Apply search filter if provided
	if searchTerm != "" && len(products) > 0 {
		// Manual search filtering (case insensitive)
		searchTerm = strings.ToLower(searchTerm)
		filteredProducts := []models.Product{}

		for _, product := range products {
			if strings.Contains(strings.ToLower(product.Title), searchTerm) ||
				strings.Contains(strings.ToLower(product.ShortDesc), searchTerm) {
				filteredProducts = append(filteredProducts, product)
			}
		}

		products = filteredProducts
	}

	// Get total count before pagination
	total := len(products)

	// Apply sorting
	switch sortOption {
	case "new":
		// Sort by created_at descending
		sort.Slice(products, func(i, j int) bool {
			return products[i].CreatedAt.After(products[j].CreatedAt)
		})
	case "top_day", "top_week", "top_month", "top_year", "top_all":
		// This would require sorting by upvote count within timeframe
		// For simplicity, we'll implement a basic version that just sorts by upvote count

		// First, load upvote counts for all products
		for i := range products {
			count, err := r.getProductUpvoteCount(products[i].ID)
			if err != nil {
				return nil, 0, err
			}
			products[i].UpvoteCount = count
		}

		// Sort by upvote count descending
		sort.Slice(products, func(i, j int) bool {
			return products[i].UpvoteCount > products[j].UpvoteCount
		})
	}

	// Apply pagination
	if perPage <= 0 {
		perPage = 10 // Default page size
	}
	if page <= 0 {
		page = 1 // Default page
	}

	start := (page - 1) * perPage
	end := start + perPage

	if start >= len(products) {
		// Return empty result if start is beyond the available results
		return []*models.Product{}, total, nil
	}

	if end > len(products) {
		end = len(products)
	}

	// Create slice of product pointers for the final result
	result := make([]*models.Product, end-start)
	for i := start; i < end; i++ {
		// Need to create a copy to avoid pointer issues
		product := products[i]

		// Ensure categories and chains are loaded
		err := r.loadProductCategories(&product)
		if err != nil {
			return nil, 0, err
		}

		err = r.loadProductChains(&product)
		if err != nil {
			return nil, 0, err
		}

		result[i-start] = &product
	}

	return result, total, nil
}

// GetProductByID gets a product by ID, renamed from GetProduct to match interface
func (r *Repository) GetProductByID(id string) (*models.Product, error) {
	var products []models.Product
	err := r.client.DB.From("products").Select("*").Eq("id", id).Execute(&products)
	if err != nil {
		return nil, err
	}

	if len(products) == 0 {
		return nil, errors.New("product not found")
	}

	product := &products[0]

	// Load related data
	err = r.loadProductCategories(product)
	if err != nil {
		return nil, err
	}

	err = r.loadProductChains(product)
	if err != nil {
		return nil, err
	}

	// Get upvote count
	count, err := r.getProductUpvoteCount(product.ID)
	if err != nil {
		return nil, err
	}
	product.UpvoteCount = count

	return product, nil
}

// GetProduct is kept for backward compatibility but now calls GetProductByID
func (r *Repository) GetProduct(id string) (*models.Product, error) {
	return r.GetProductByID(id)
}

// CreateCategory creates a new category
func (r *Repository) CreateCategory(category *models.Category) error {
	if category.ID == "" {
		category.ID = generateID()
	}
	category.CreatedAt = time.Now()
	category.UpdatedAt = time.Now()

	var result []models.Category
	err := r.client.DB.From("categories").Insert(category).Execute(&result)
	if err != nil {
		return err
	}

	if len(result) > 0 {
		*category = result[0]
	}

	return nil
}

// GetCategories gets all categories
func (r *Repository) GetCategories() ([]models.Category, error) {
	var categories []models.Category
	err := r.client.DB.From("categories").Select("*").Execute(&categories)
	if err != nil {
		return nil, err
	}

	// For each category, get the product count
	for i := range categories {
		count, err := r.getCategoryProductCount(categories[i].ID)
		if err != nil {
			return nil, err
		}
		categories[i].ProductCount = count
	}

	return categories, nil
}

// UpvoteProduct adds an upvote to a product
func (r *Repository) UpvoteProduct(userID, productID string) error {
	// Check if upvote already exists
	var upvotes []models.Upvote
	err := r.client.DB.From("upvotes").
		Select("*").
		Eq("user_id", userID).
		Eq("product_id", productID).
		Execute(&upvotes)

	if err != nil {
		return err
	}

	if len(upvotes) > 0 {
		// Upvote already exists, cannot upvote again
		return errors.New("already upvoted")
	}

	// Create new upvote
	upvote := models.Upvote{
		ID:        generateID(),
		UserID:    userID,
		ProductID: productID,
		CreatedAt: time.Now(),
	}

	var result []models.Upvote
	err = r.client.DB.From("upvotes").Insert(upvote).Execute(&result)
	if err != nil {
		return err
	}

	return nil
}

// GetPendingEdits gets all pending edits
func (r *Repository) GetPendingEdits() ([]models.PendingEdit, error) {
	var pendingEdits []models.PendingEdit
	err := r.client.DB.From("pending_edits").
		Select("*").
		Eq("status", "pending").
		Execute(&pendingEdits)

	if err != nil {
		return nil, err
	}

	return pendingEdits, nil
}

// ApproveEdit approves a pending edit
func (r *Repository) ApproveEdit(editID string) error {
	// Get the pending edit
	var edits []models.PendingEdit
	err := r.client.DB.From("pending_edits").
		Select("*").
		Eq("id", editID).
		Execute(&edits)

	if err != nil {
		return err
	}

	if len(edits) == 0 {
		return errors.New("edit not found")
	}

	edit := &edits[0]

	// Process based on entity type and change type
	if edit.EntityType == "product" {
		var product models.Product
		err = json.Unmarshal([]byte(edit.ChangeData), &product)
		if err != nil {
			return err
		}

		if edit.ChangeType == "create" {
			// Set the product as approved
			err = r.client.DB.From("products").
				Update(map[string]interface{}{"approved": true}).
				Eq("id", product.ID).
				Execute(nil)

			if err != nil {
				return err
			}
		} else if edit.ChangeType == "update" {
			// Update the product
			err = r.client.DB.From("products").
				Update(map[string]interface{}{
					"title":            product.Title,
					"short_desc":       product.ShortDesc,
					"long_desc":        product.LongDesc,
					"logo_url":         product.LogoURL,
					"markdown_content": product.MarkdownContent,
					"updated_at":       time.Now(),
				}).
				Eq("id", product.ID).
				Execute(nil)

			if err != nil {
				return err
			}
		}
	} else if edit.EntityType == "category" {
		var category models.Category
		err = json.Unmarshal([]byte(edit.ChangeData), &category)
		if err != nil {
			return err
		}

		if edit.ChangeType == "create" {
			// Nothing to do as categories are automatically approved
		} else if edit.ChangeType == "update" {
			// Update the category
			err = r.client.DB.From("categories").
				Update(map[string]interface{}{
					"name":        category.Name,
					"description": category.Description,
					"updated_at":  time.Now(),
				}).
				Eq("id", category.ID).
				Execute(nil)

			if err != nil {
				return err
			}
		}
	}

	// Update the edit status
	err = r.client.DB.From("pending_edits").
		Update(map[string]interface{}{
			"status":       "approved",
			"processed_at": time.Now(),
		}).
		Eq("id", editID).
		Execute(nil)

	if err != nil {
		return err
	}

	return nil
}

// RejectEdit rejects a pending edit
func (r *Repository) RejectEdit(editID string) error {
	// Update the edit status
	err := r.client.DB.From("pending_edits").
		Update(map[string]interface{}{
			"status":       "rejected",
			"processed_at": time.Now(),
		}).
		Eq("id", editID).
		Execute(nil)

	if err != nil {
		return err
	}

	return nil
}

// Helper functions

func (r *Repository) loadProductCategories(product *models.Product) error {
	// Get the category IDs for this product
	var junctions []struct {
		ProductID  string `json:"product_id"`
		CategoryID string `json:"category_id"`
	}

	err := r.client.DB.From("product_categories").
		Select("category_id").
		Eq("product_id", product.ID).
		Execute(&junctions)

	if err != nil {
		return err
	}

	if len(junctions) == 0 {
		product.Categories = []models.Category{}
		return nil
	}

	// Get all category IDs
	categoryIDs := make([]string, len(junctions))
	for i, j := range junctions {
		categoryIDs[i] = j.CategoryID
	}

	// Fetch the categories
	var categories []models.Category
	for _, catID := range categoryIDs {
		var cats []models.Category
		err := r.client.DB.From("categories").
			Select("*").
			Eq("id", catID).
			Execute(&cats)

		if err != nil {
			return err
		}

		if len(cats) > 0 {
			categories = append(categories, cats[0])
		}
	}

	product.Categories = categories
	return nil
}

func (r *Repository) loadProductChains(product *models.Product) error {
	// Get the chain IDs for this product
	var junctions []struct {
		ProductID string `json:"product_id"`
		ChainID   string `json:"chain_id"`
	}

	err := r.client.DB.From("product_chains").
		Select("chain_id").
		Eq("product_id", product.ID).
		Execute(&junctions)

	if err != nil {
		return err
	}

	if len(junctions) == 0 {
		product.Chains = []models.Chain{}
		return nil
	}

	// Get all chain IDs
	chainIDs := make([]string, len(junctions))
	for i, j := range junctions {
		chainIDs[i] = j.ChainID
	}

	// Fetch the chains
	var chains []models.Chain
	for _, chainID := range chainIDs {
		var ch []models.Chain
		err := r.client.DB.From("chains").
			Select("*").
			Eq("id", chainID).
			Execute(&ch)

		if err != nil {
			return err
		}

		if len(ch) > 0 {
			chains = append(chains, ch[0])
		}
	}

	product.Chains = chains
	return nil
}

func (r *Repository) getProductUpvoteCount(productID string) (int, error) {
	// This is a workaround for the Supabase client limitations
	// Ideally, we'd use a more efficient SQL query
	var upvotes []models.Upvote
	err := r.client.DB.From("upvotes").
		Select("*").
		Eq("product_id", productID).
		Execute(&upvotes)

	if err != nil {
		return 0, err
	}

	return len(upvotes), nil
}

func (r *Repository) getCategoryProductCount(categoryID string) (int, error) {
	// Get the count of products in this category
	var junctions []struct {
		ProductID  string `json:"product_id"`
		CategoryID string `json:"category_id"`
	}

	err := r.client.DB.From("product_categories").
		Select("product_id").
		Eq("category_id", categoryID).
		Execute(&junctions)

	if err != nil {
		return 0, err
	}

	return len(junctions), nil
}

func (r *Repository) addProductCategory(productID, categoryID string) error {
	junction := struct {
		ProductID  string `json:"product_id"`
		CategoryID string `json:"category_id"`
	}{
		ProductID:  productID,
		CategoryID: categoryID,
	}

	err := r.client.DB.From("product_categories").
		Insert(junction).
		Execute(nil)

	return err
}

func (r *Repository) addProductChain(productID, chainID string) error {
	junction := struct {
		ProductID string `json:"product_id"`
		ChainID   string `json:"chain_id"`
	}{
		ProductID: productID,
		ChainID:   chainID,
	}

	err := r.client.DB.From("product_chains").
		Insert(junction).
		Execute(nil)

	return err
}

// generateID generates a unique ID for database entities
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
