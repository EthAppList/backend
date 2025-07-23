package service

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"

	"github.com/wesjorgensen/EthAppList/backend/internal/config"
	"github.com/wesjorgensen/EthAppList/backend/internal/models"
	"github.com/wesjorgensen/EthAppList/backend/internal/redis"
)

// DataRepository interface defines the methods required by the service
type DataRepository interface {
	// User methods
	CreateUser(user *models.User) error
	GetUserByWallet(walletAddress string) (*models.User, error)

	// Product methods
	CreateProduct(product *models.Product) error
	GetProductByID(id string) (*models.Product, error)
	GetProductByIDAdmin(id string) (*models.Product, error)
	GetProducts(categoryID, chainID, searchTerm, sortOption string, page, perPage int) ([]*models.Product, int, error)
	UpdateProduct(product *models.Product) error
	DeleteAllProducts() error

	// Product revision methods
	CreateProductRevision(productID string, editorID *string, editSummary *string, changes []models.ProductFieldChange, newProductData *models.Product) error
	GetProductRevisions(productID string, page, perPage int) ([]models.RevisionSummary, int, error)
	GetProductRevision(productID string, revisionNumber int) (*models.ProductRevision, error)
	CompareProductRevisions(productID string, fromRevision, toRevision int) (*models.ProductDiff, error)
	RevertProductToRevision(productID string, revisionNumber int, editorID *string, reason string) error
	GetRecentEdits(limit int) ([]models.RevisionSummary, error)

	// Category methods
	GetCategories() ([]models.Category, error)
	CreateCategory(category *models.Category) error

	// Chain methods
	GetChains() ([]models.Chain, error)

	// Upvote methods
	UpvoteProduct(userID, productID string) error
}

// Service implements business logic for the application
type Service struct {
	repo  DataRepository
	redis *redis.RedisService
	cfg   *config.Config
}

// New creates a new service with Redis support
func New(repo DataRepository, redisService *redis.RedisService, cfg *config.Config) *Service {
	return &Service{
		repo:  repo,
		redis: redisService,
		cfg:   cfg,
	}
}

// GetConfig returns the config for middleware and other components
func (s *Service) GetConfig() *config.Config {
	return s.cfg
}

// AuthenticateWallet verifies a wallet signature and returns a JWT token
func (s *Service) AuthenticateWallet(address, signature, message string) (string, error) {
	// Validate the signature
	valid, err := s.verifySignature(address, signature, message)
	if err != nil || !valid {
		return "", errors.New("invalid signature")
	}

	// Check if user exists
	user, err := s.repo.GetUserByWallet(address)
	if err != nil {
		// Create new user if not exists
		user = &models.User{
			WalletAddress: address,
		}
		err = s.repo.CreateUser(user)
		if err != nil {
			return "", fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Generate JWT token
	token, err := s.generateJWT(user)
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	return token, nil
}

// GetProducts returns a list of products based on filter
func (s *Service) GetProducts(categoryID, chainID, searchTerm, sortOption string, page, perPage int) ([]*models.Product, int, error) {
	return s.repo.GetProducts(categoryID, chainID, searchTerm, sortOption, page, perPage)
}

// GetProduct returns a single APPROVED product by ID (public API)
func (s *Service) GetProduct(id string) (*models.Product, error) {
	return s.repo.GetProductByID(id)
}

// SubmitProduct creates a new product submission for admin review
func (s *Service) SubmitProduct(product *models.Product, userWallet string) error {
	// Set default values for submission
	product.CurrentRevisionNumber = 1

	// SECURITY: Always set scores to default values (50) regardless of what client sends
	// This prevents malicious users from injecting high scores via API calls
	product.SecurityScore = 0.5
	product.UXScore = 0.5
	product.OverallScore = 0.5
	product.VibesScore = 0.5

	// Check if this is an admin submission - if so, auto-approve (git push force)
	if s.IsUserAdmin(userWallet) {
		// Admin submissions go directly to main database (bypass Redis)
		product.Approved = true
		product.ID = "" // Let repository generate clean ID for approved products
		return s.repo.CreateProduct(product)
	}

	// For non-admin users, store in Redis as pending change (git-like workflow)
	pendingProduct := &models.PendingProduct{
		UserID:  product.SubmitterID,
		Product: *product,
	}

	err := s.redis.StorePendingProduct(pendingProduct)
	if err != nil {
		return fmt.Errorf("failed to store pending product in Redis: %w", err)
	}

	return nil
}

// SubmitProductEdit submits edits to an existing product for admin review
func (s *Service) SubmitProductEdit(productID string, updatedProduct *models.Product, userID string, userWallet string) error {
	// Ensure the product exists (use admin method to find any product, approved or not)
	existingProduct, err := s.repo.GetProductByIDAdmin(productID)
	if err != nil {
		return fmt.Errorf("product not found: %w", err)
	}

	// Ensure we're updating the correct product ID
	updatedProduct.ID = existingProduct.ID

	// Check if this is an admin submission - if so, auto-approve (git push force)
	if s.IsUserAdmin(userWallet) {
		// For admin edits, directly update the main product using revision system
		// This bypasses Redis and goes straight to the database like git push force
		editSummary := "Admin edit (auto-approved)"
		err = s.UpdateProduct(updatedProduct, userID, editSummary, false, true)
		if err != nil {
			return fmt.Errorf("failed to auto-approve admin edit: %w", err)
		}

		return nil
	}

	// For non-admin users, store edit in Redis as pending change (git-like workflow)
	pendingEdit := &models.PendingProductEdit{
		UserID:       userID,
		ProductID:    productID,
		OriginalData: *existingProduct,
		UpdatedData:  *updatedProduct,
		EditSummary:  "User-submitted edit",
	}

	err = s.redis.StorePendingEdit(pendingEdit)
	if err != nil {
		return fmt.Errorf("failed to store pending edit in Redis: %w", err)
	}

	return nil
}

// GetCategories returns all categories
func (s *Service) GetCategories() ([]models.Category, error) {
	return s.repo.GetCategories()
}

// SubmitCategory creates a new category
func (s *Service) SubmitCategory(category *models.Category) error {
	return s.repo.CreateCategory(category)
}

// GetChains returns all chains
func (s *Service) GetChains() ([]models.Chain, error) {
	return s.repo.GetChains()
}

// UpvoteProduct adds an upvote to a product
func (s *Service) UpvoteProduct(userID, productID string) error {
	return s.repo.UpvoteProduct(userID, productID)
}

// ApproveEdit approves a pending edit from Redis and merges it to main database
func (s *Service) ApproveEdit(editID string) error {
	// Try to approve as pending product first
	pendingProduct, err := s.redis.ApprovePendingProduct(editID)
	if err == nil {
		// Successfully found and approved a pending product - now create it in main database
		product := pendingProduct.Product
		product.Approved = true
		product.ID = "" // Let repository generate clean ID

		err = s.repo.CreateProduct(&product)
		if err != nil {
			return fmt.Errorf("failed to create approved product in database: %w", err)
		}

		return nil
	}

	// Try to approve as pending edit
	pendingEdit, err := s.redis.ApprovePendingEdit(editID)
	if err == nil {
		// Successfully found and approved a pending edit - now merge it to main database
		updatedProduct := pendingEdit.UpdatedData
		updatedProduct.Approved = true

		// Use the revision system to apply the changes
		editSummary := pendingEdit.EditSummary
		if editSummary == "" {
			editSummary = "Approved user edit"
		}

		err = s.UpdateProduct(&updatedProduct, pendingEdit.UserID, editSummary, false, true)
		if err != nil {
			return fmt.Errorf("failed to merge approved edit to database: %w", err)
		}

		return nil
	}

	return fmt.Errorf("pending change with ID %s not found in Redis", editID)
}

// RejectEdit rejects a pending edit from Redis
func (s *Service) RejectEdit(editID string) error {
	err := s.redis.RejectPendingChange(editID)
	if err != nil {
		return fmt.Errorf("failed to reject pending change: %w", err)
	}

	return nil
}

// GetAllPendingChanges returns all pending changes from Redis
func (s *Service) GetAllPendingChanges() (*models.PendingChangesList, error) {
	return s.redis.GetAllPendingChanges()
}

// GetPendingProduct returns a specific pending product from Redis
func (s *Service) GetPendingProduct(id string) (*models.PendingProduct, error) {
	return s.redis.GetPendingProduct(id)
}

// GetPendingEdit returns a specific pending edit from Redis
func (s *Service) GetPendingEdit(id string) (*models.PendingProductEdit, error) {
	return s.redis.GetPendingEdit(id)
}

// GetUserByWallet gets a user by their wallet address
func (s *Service) GetUserByWallet(walletAddress string) (*models.User, error) {
	return s.repo.GetUserByWallet(walletAddress)
}

// IsUserAdmin checks if a user is an administrator based on their wallet address
func (s *Service) IsUserAdmin(walletAddress string) bool {
	return strings.ToLower(walletAddress) == strings.ToLower(s.cfg.AdminWallet)
}

// DeleteAllProducts removes all products from the database (for testing purposes only)
func (s *Service) DeleteAllProducts() error {
	return s.repo.DeleteAllProducts()
}

// Revision system service methods

// GetProductHistory returns the revision history for a product
func (s *Service) GetProductHistory(productID string, page, perPage int) ([]models.RevisionSummary, int, error) {
	return s.repo.GetProductRevisions(productID, page, perPage)
}

// GetProductRevision returns a specific revision of a product
func (s *Service) GetProductRevision(productID string, revisionNumber int) (*models.ProductRevision, error) {
	return s.repo.GetProductRevision(productID, revisionNumber)
}

// CompareProductRevisions compares two revisions of a product
func (s *Service) CompareProductRevisions(productID string, fromRevision, toRevision int) (*models.ProductDiff, error) {
	return s.repo.CompareProductRevisions(productID, fromRevision, toRevision)
}

// RevertProduct reverts a product to a specific revision
func (s *Service) RevertProduct(productID string, revisionNumber int, editorID, reason string) error {
	return s.repo.RevertProductToRevision(productID, revisionNumber, &editorID, reason)
}

// GetRecentEdits returns recent product edits across all products
func (s *Service) GetRecentEdits(limit int) ([]models.RevisionSummary, error) {
	return s.repo.GetRecentEdits(limit)
}

// Helper functions

// verifySignature verifies an Ethereum signature
func (s *Service) verifySignature(walletAddress, signature, message string) (bool, error) {
	// Convert wallet address to lowercase for consistency
	walletAddress = strings.ToLower(walletAddress)

	// Format the message according to Ethereum standards
	fullMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)

	// Hash the message
	hash := crypto.Keccak256Hash([]byte(fullMessage))

	// Decode the signature
	signatureBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return false, err
	}

	// Adjust V value in signature if needed (Ethereum signature quirk)
	if len(signatureBytes) == 65 {
		// The V value is the last byte
		if signatureBytes[64] >= 27 {
			signatureBytes[64] -= 27
		}
	}

	// Recover the public key from the signature
	sigPublicKey, err := crypto.SigToPub(hash.Bytes(), signatureBytes)
	if err != nil {
		return false, err
	}

	// Convert the public key to an Ethereum address
	recoveredAddress := strings.ToLower(crypto.PubkeyToAddress(*sigPublicKey).Hex())

	// Compare the recovered address with the provided address
	return recoveredAddress == walletAddress, nil
}

// generateJWT generates a JWT token for a user
func (s *Service) generateJWT(user *models.User) (string, error) {
	// Set token expiration
	expirationTime := time.Now().Add(24 * 7 * time.Hour) // 1 week

	// Create the claims
	claims := jwt.MapClaims{
		"wallet": user.WalletAddress,
		"id":     user.ID,
		"exp":    expirationTime.Unix(),
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// UpdateProduct handles direct product updates with edit summaries
func (s *Service) UpdateProduct(product *models.Product, editorID, editSummary string, minorEdit bool, isAdmin bool) error {
	// Get the current product to compare changes
	currentProduct, err := s.repo.GetProductByID(product.ID)
	if err != nil {
		return err
	}

	// SECURITY: For non-admin users, preserve existing scores to prevent manipulation
	// Only admins should be able to modify product scores
	if !isAdmin {
		// Non-admin users cannot modify scores - preserve current values
		product.SecurityScore = currentProduct.SecurityScore
		product.UXScore = currentProduct.UXScore
		product.OverallScore = currentProduct.OverallScore
		product.VibesScore = currentProduct.VibesScore
	}

	// Calculate field changes between current and updated product
	changes := calculateProductChanges(currentProduct, product)

	// Create a revision record for this update first
	err = s.repo.CreateProductRevision(product.ID, &editorID, &editSummary, changes, product)
	if err != nil {
		return err
	}

	// Update the product's revision number and last editor
	product.CurrentRevisionNumber = currentProduct.CurrentRevisionNumber + 1
	product.LastEditorID = &editorID

	// Note: We need to add UpdateProduct to the repository interface
	// For now, this will cause a linter error that we'll address

	// Update the product in the database
	err = s.repo.UpdateProduct(product)
	if err != nil {
		return err
	}

	return nil
}

// calculateProductChanges compares two products and returns the field changes
func calculateProductChanges(oldProduct, newProduct *models.Product) []models.ProductFieldChange {
	var changes []models.ProductFieldChange

	// Compare each field and record changes
	if oldProduct.Title != newProduct.Title {
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "title",
			OldValue:   &oldProduct.Title,
			NewValue:   &newProduct.Title,
			ChangeType: "modified",
		})
	}

	if oldProduct.ShortDesc != newProduct.ShortDesc {
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "short_desc",
			OldValue:   &oldProduct.ShortDesc,
			NewValue:   &newProduct.ShortDesc,
			ChangeType: "modified",
		})
	}

	if oldProduct.LongDesc != newProduct.LongDesc {
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "long_desc",
			OldValue:   &oldProduct.LongDesc,
			NewValue:   &newProduct.LongDesc,
			ChangeType: "modified",
		})
	}

	if oldProduct.LogoURL != newProduct.LogoURL {
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "logo_url",
			OldValue:   &oldProduct.LogoURL,
			NewValue:   &newProduct.LogoURL,
			ChangeType: "modified",
		})
	}

	if oldProduct.MarkdownContent != newProduct.MarkdownContent {
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "markdown_content",
			OldValue:   &oldProduct.MarkdownContent,
			NewValue:   &newProduct.MarkdownContent,
			ChangeType: "modified",
		})
	}

	if oldProduct.SecurityScore != newProduct.SecurityScore {
		oldValue := fmt.Sprintf("%.2f", oldProduct.SecurityScore)
		newValue := fmt.Sprintf("%.2f", newProduct.SecurityScore)
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "security_score",
			OldValue:   &oldValue,
			NewValue:   &newValue,
			ChangeType: "modified",
		})
	}

	if oldProduct.UXScore != newProduct.UXScore {
		oldValue := fmt.Sprintf("%.2f", oldProduct.UXScore)
		newValue := fmt.Sprintf("%.2f", newProduct.UXScore)
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "ux_score",
			OldValue:   &oldValue,
			NewValue:   &newValue,
			ChangeType: "modified",
		})
	}

	if oldProduct.OverallScore != newProduct.OverallScore {
		oldValue := fmt.Sprintf("%.2f", oldProduct.OverallScore)
		newValue := fmt.Sprintf("%.2f", newProduct.OverallScore)
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "overall_score",
			OldValue:   &oldValue,
			NewValue:   &newValue,
			ChangeType: "modified",
		})
	}

	if oldProduct.VibesScore != newProduct.VibesScore {
		oldValue := fmt.Sprintf("%.2f", oldProduct.VibesScore)
		newValue := fmt.Sprintf("%.2f", newProduct.VibesScore)
		changes = append(changes, models.ProductFieldChange{
			FieldName:  "vibes_score",
			OldValue:   &oldValue,
			NewValue:   &newValue,
			ChangeType: "modified",
		})
	}

	// Add ID for each change record (in real implementation, the repository would do this)
	for i := range changes {
		changes[i].ID = fmt.Sprintf("change_%d", i+1)
	}

	return changes
}
