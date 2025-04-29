package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/wesjorgensen/EthAppList/backend/internal/middleware"
	"github.com/wesjorgensen/EthAppList/backend/internal/models"
	"github.com/wesjorgensen/EthAppList/backend/internal/service"
)

// Handler contains all HTTP handlers
type Handler struct {
	svc *service.Service
}

// New creates a new handler
func New(svc *service.Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

// RegisterAuthHandlers registers auth-related routes
func RegisterAuthHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	router.HandleFunc("/wallet", h.AuthenticateWallet).Methods("POST")
}

// RegisterProductHandlers registers product-related routes
func RegisterProductHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	router.HandleFunc("", h.GetProducts).Methods("GET")
	router.HandleFunc("/{id}", h.GetProduct).Methods("GET")

	// Protected routes
	protectedRouter := router.NewRoute().Subrouter()
	protectedRouter.Use(middleware.Auth(svc.GetConfig()))

	protectedRouter.HandleFunc("", h.SubmitProduct).Methods("POST")
	protectedRouter.HandleFunc("/{id}/upvote", h.UpvoteProduct).Methods("POST")
}

// RegisterCategoryHandlers registers category-related routes
func RegisterCategoryHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	router.HandleFunc("", h.GetCategories).Methods("GET")

	// Protected routes
	protectedRouter := router.NewRoute().Subrouter()
	protectedRouter.Use(middleware.Auth(svc.GetConfig()))

	protectedRouter.HandleFunc("", h.SubmitCategory).Methods("POST")
}

// RegisterAdminHandlers registers admin-related routes
func RegisterAdminHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	router.HandleFunc("/pending", h.GetPendingEdits).Methods("GET")
	router.HandleFunc("/approve/{id}", h.ApproveEdit).Methods("POST")
	router.HandleFunc("/reject/{id}", h.RejectEdit).Methods("POST")
}

// AuthenticateWallet handles wallet authentication
func (h *Handler) AuthenticateWallet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WalletAddress string `json:"wallet_address"`
		Signature     string `json:"signature"`
		Message       string `json:"message"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, err := h.svc.AuthenticateWallet(req.WalletAddress, req.Signature, req.Message)
	if err != nil {
		http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	resp := struct {
		Token string `json:"token"`
	}{
		Token: token,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetProducts handles getting all products with filters
func (h *Handler) GetProducts(w http.ResponseWriter, r *http.Request) {
	// Parse filters from query parameters
	categoryID := r.URL.Query().Get("category")
	chainID := r.URL.Query().Get("chain")
	searchTerm := r.URL.Query().Get("search")
	sortOption := r.URL.Query().Get("sort")

	// Default to "new" if sort is not specified
	if sortOption == "" {
		sortOption = "new"
	}

	// Parse pagination parameters
	page := 1
	perPage := 10

	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	if pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	if perPageStr != "" {
		parsedPerPage, err := strconv.Atoi(perPageStr)
		if err == nil && parsedPerPage > 0 {
			perPage = parsedPerPage
		}
	}

	// Call the service to get products
	products, total, err := h.svc.GetProducts(categoryID, chainID, searchTerm, sortOption, page, perPage)
	if err != nil {
		http.Error(w, "Failed to get products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare the response with pagination metadata
	response := struct {
		Products []*models.Product `json:"products"`
		Total    int               `json:"total"`
		Page     int               `json:"page"`
		PerPage  int               `json:"per_page"`
		Pages    int               `json:"pages"`
	}{
		Products: products,
		Total:    total,
		Page:     page,
		PerPage:  perPage,
		Pages:    (total + perPage - 1) / perPage, // Ceiling division to get total pages
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProduct handles getting a single product
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	product, err := h.svc.GetProduct(id)
	if err != nil {
		http.Error(w, "Failed to get product: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

// SubmitProduct handles product submission
func (h *Handler) SubmitProduct(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user ID is missing
	if user.ID == "" {
		// Look up the user from the database by wallet address
		fullUser, err := h.svc.GetUserByWallet(user.WalletAddress)
		if err != nil {
			http.Error(w, "Failed to get user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		user = fullUser
	}

	var product models.Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set submitter ID from the user (either from token or looked up)
	product.SubmitterID = user.ID

	err = h.svc.SubmitProduct(&product)
	if err != nil {
		http.Error(w, "Failed to submit product: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

// UpvoteProduct handles product upvoting
func (h *Handler) UpvoteProduct(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if user ID is missing
	if user.ID == "" {
		// Look up the user from the database by wallet address
		fullUser, err := h.svc.GetUserByWallet(user.WalletAddress)
		if err != nil {
			http.Error(w, "Failed to get user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		user = fullUser
	}

	vars := mux.Vars(r)
	productID := vars["id"]

	err := h.svc.UpvoteProduct(user.ID, productID)
	if err != nil {
		if err.Error() == "already upvoted" {
			http.Error(w, "Already upvoted", http.StatusConflict)
		} else {
			http.Error(w, "Failed to upvote product: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCategories handles getting all categories
func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.svc.GetCategories()
	if err != nil {
		http.Error(w, "Failed to get categories: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// SubmitCategory handles category submission
func (h *Handler) SubmitCategory(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	_, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Note: Current implementation doesn't use user ID for category creation
	// but we might need it in the future for audit trails or permissions

	var category models.Category
	err := json.NewDecoder(r.Body).Decode(&category)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.svc.SubmitCategory(&category)
	if err != nil {
		http.Error(w, "Failed to submit category: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(category)
}

// GetPendingEdits handles getting all pending edits
func (h *Handler) GetPendingEdits(w http.ResponseWriter, r *http.Request) {
	pendingEdits, err := h.svc.GetPendingEdits()
	if err != nil {
		http.Error(w, "Failed to get pending edits: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingEdits)
}

// ApproveEdit handles approving a pending edit
func (h *Handler) ApproveEdit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	err := h.svc.ApproveEdit(id)
	if err != nil {
		http.Error(w, "Failed to approve edit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RejectEdit handles rejecting a pending edit
func (h *Handler) RejectEdit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	err := h.svc.RejectEdit(id)
	if err != nil {
		http.Error(w, "Failed to reject edit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
