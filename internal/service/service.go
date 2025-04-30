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
)

// DataRepository interface defines the methods required by the service
type DataRepository interface {
	// User methods
	CreateUser(user *models.User) error
	GetUserByWallet(walletAddress string) (*models.User, error)

	// Product methods
	CreateProduct(product *models.Product) error
	GetProductByID(id string) (*models.Product, error)
	GetProducts(categoryID, chainID, searchTerm, sortOption string, page, perPage int) ([]*models.Product, int, error)
	DeleteAllProducts() error

	// Category methods
	GetCategories() ([]models.Category, error)
	CreateCategory(category *models.Category) error

	// Upvote methods
	UpvoteProduct(userID, productID string) error

	// Admin methods
	GetPendingEdits() ([]models.PendingEdit, error)
	ApproveEdit(editID string) error
	RejectEdit(editID string) error
}

// Service implements business logic for the application
type Service struct {
	repo DataRepository
	cfg  *config.Config
}

// New creates a new service
func New(repo DataRepository, cfg *config.Config) *Service {
	return &Service{
		repo: repo,
		cfg:  cfg,
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

// GetProduct returns a single product by ID
func (s *Service) GetProduct(id string) (*models.Product, error) {
	return s.repo.GetProductByID(id)
}

// SubmitProduct creates a new product
func (s *Service) SubmitProduct(product *models.Product) error {
	return s.repo.CreateProduct(product)
}

// GetCategories returns all categories
func (s *Service) GetCategories() ([]models.Category, error) {
	return s.repo.GetCategories()
}

// SubmitCategory creates a new category
func (s *Service) SubmitCategory(category *models.Category) error {
	return s.repo.CreateCategory(category)
}

// UpvoteProduct adds an upvote to a product
func (s *Service) UpvoteProduct(userID, productID string) error {
	return s.repo.UpvoteProduct(userID, productID)
}

// GetPendingEdits returns all pending edits
func (s *Service) GetPendingEdits() ([]models.PendingEdit, error) {
	return s.repo.GetPendingEdits()
}

// ApproveEdit approves a pending edit
func (s *Service) ApproveEdit(editID string) error {
	return s.repo.ApproveEdit(editID)
}

// RejectEdit rejects a pending edit
func (s *Service) RejectEdit(editID string) error {
	return s.repo.RejectEdit(editID)
}

// GetUserByWallet gets a user by their wallet address
func (s *Service) GetUserByWallet(walletAddress string) (*models.User, error) {
	return s.repo.GetUserByWallet(walletAddress)
}

// DeleteAllProducts removes all products from the database (for testing purposes only)
func (s *Service) DeleteAllProducts() error {
	return s.repo.DeleteAllProducts()
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
