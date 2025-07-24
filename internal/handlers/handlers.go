package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

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
	router.HandleFunc("/trending", h.GetTrendingProducts).Methods("GET")
	router.HandleFunc("/{id}", h.GetProduct).Methods("GET")

	// Revision system endpoints
	router.HandleFunc("/{id}/history", h.GetProductHistory).Methods("GET")
	router.HandleFunc("/{id}/revisions/{revision}", h.GetProductRevision).Methods("GET")
	router.HandleFunc("/{id}/compare/{rev1}/{rev2}", h.CompareProductRevisions).Methods("GET")

	// Protected routes
	protectedRouter := router.NewRoute().Subrouter()
	protectedRouter.Use(middleware.Auth(svc.GetConfig()))

	protectedRouter.HandleFunc("", h.SubmitProduct).Methods("POST")
	protectedRouter.HandleFunc("/{id}/upvote", h.UpvoteProduct).Methods("POST")
	protectedRouter.HandleFunc("/{id}/scores", h.SubmitProductScore).Methods("POST")
	protectedRouter.HandleFunc("/{id}/my-score", h.GetMyProductScore).Methods("GET")
	protectedRouter.HandleFunc("/{id}", h.UpdateProduct).Methods("PUT")
	protectedRouter.HandleFunc("/{id}/edit", h.SubmitProductEdit).Methods("POST")

	// Admin-only revision routes
	protectedRouter.HandleFunc("/{id}/revert/{revision}", h.RevertProduct).Methods("POST")
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

// RegisterChainHandlers registers chain-related routes
func RegisterChainHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	router.HandleFunc("", h.GetChains).Methods("GET")
}

// RegisterAdminHandlers registers admin-related routes
func RegisterAdminHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	// Redis-based pending changes endpoints
	router.HandleFunc("/pending-changes", h.GetPendingChanges).Methods("GET")
	router.HandleFunc("/pending-products/{id}", h.GetPendingProduct).Methods("GET")
	router.HandleFunc("/pending-edits/{id}", h.GetPendingEdit).Methods("GET")

	// Approval/rejection endpoints
	router.HandleFunc("/approve/{id}", h.ApproveEdit).Methods("POST")
	router.HandleFunc("/reject/{id}", h.RejectEdit).Methods("POST")
	router.HandleFunc("/recent-edits", h.GetRecentEdits).Methods("GET")
}

// RegisterUserHandlers registers user-related routes
func RegisterUserHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	// Protected routes that require authentication
	protectedRouter := router.NewRoute().Subrouter()
	protectedRouter.Use(middleware.Auth(svc.GetConfig()))

	protectedRouter.HandleFunc("/profile", h.GetUserProfile).Methods("GET")
	protectedRouter.HandleFunc("/permissions", h.GetUserPermissions).Methods("GET")
	protectedRouter.HandleFunc("/vote-states", h.GetUserVoteStates).Methods("GET")
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

	// Add vote counts to products
	err = h.svc.EnrichProductsWithVoteCounts(products)
	if err != nil {
		log.Printf("Warning: failed to enrich products with vote counts: %v", err)
		// Continue without vote counts
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

// GetProduct handles getting a single APPROVED product (public API)
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

	err = h.svc.SubmitProduct(&product, user.WalletAddress)
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

// GetChains handles getting all chains
func (h *Handler) GetChains(w http.ResponseWriter, r *http.Request) {
	chains, err := h.svc.GetChains()
	if err != nil {
		http.Error(w, "Failed to get chains: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chains)
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

// ApproveEdit handles approving a pending edit
func (h *Handler) ApproveEdit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	log.Printf("ApproveEdit: Attempting to approve edit with ID: %s", id)

	err := h.svc.ApproveEdit(id)
	if err != nil {
		log.Printf("ApproveEdit: Error approving edit %s: %v", id, err)
		http.Error(w, "Failed to approve edit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("ApproveEdit: Successfully approved edit with ID: %s", id)
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

// DeleteAllProducts handles the temporary endpoint to delete all products
func (h *Handler) DeleteAllProducts(w http.ResponseWriter, r *http.Request) {
	err := h.svc.DeleteAllProducts()
	if err != nil {
		http.Error(w, "Failed to delete products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Message string `json:"message"`
	}{
		Message: "All products have been deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProductHistory handles getting the edit history for a product
func (h *Handler) GetProductHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	// Parse pagination parameters
	page := 1
	perPage := 20

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
		if err == nil && parsedPerPage > 0 && parsedPerPage <= 100 {
			perPage = parsedPerPage
		}
	}

	revisions, total, err := h.svc.GetProductHistory(productID, page, perPage)
	if err != nil {
		http.Error(w, "Failed to get product history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Revisions []models.RevisionSummary `json:"revisions"`
		Total     int                      `json:"total"`
		Page      int                      `json:"page"`
		PerPage   int                      `json:"per_page"`
		Pages     int                      `json:"pages"`
	}{
		Revisions: revisions,
		Total:     total,
		Page:      page,
		PerPage:   perPage,
		Pages:     (total + perPage - 1) / perPage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProductRevision handles getting a specific revision of a product
func (h *Handler) GetProductRevision(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	revisionStr := vars["revision"]

	revision, err := strconv.Atoi(revisionStr)
	if err != nil {
		http.Error(w, "Invalid revision number", http.StatusBadRequest)
		return
	}

	productRevision, err := h.svc.GetProductRevision(productID, revision)
	if err != nil {
		http.Error(w, "Failed to get product revision: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(productRevision)
}

// CompareProductRevisions handles comparing two revisions of a product
func (h *Handler) CompareProductRevisions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	rev1Str := vars["rev1"]
	rev2Str := vars["rev2"]

	rev1, err := strconv.Atoi(rev1Str)
	if err != nil {
		http.Error(w, "Invalid revision number for rev1", http.StatusBadRequest)
		return
	}

	rev2, err := strconv.Atoi(rev2Str)
	if err != nil {
		http.Error(w, "Invalid revision number for rev2", http.StatusBadRequest)
		return
	}

	diff, err := h.svc.CompareProductRevisions(productID, rev1, rev2)
	if err != nil {
		http.Error(w, "Failed to compare revisions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

// RevertProduct handles reverting a product to a specific revision (admin only)
func (h *Handler) RevertProduct(w http.ResponseWriter, r *http.Request) {
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

	// TODO: Add admin check here - for now allowing any authenticated user
	// In production, you would check if user.IsAdmin or similar

	vars := mux.Vars(r)
	productID := vars["id"]
	revisionStr := vars["revision"]

	revision, err := strconv.Atoi(revisionStr)
	if err != nil {
		http.Error(w, "Invalid revision number", http.StatusBadRequest)
		return
	}

	// Parse request body for revert reason
	var req struct {
		Reason string `json:"reason"`
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Reason == "" {
		req.Reason = "Manual revert by admin"
	}

	err = h.svc.RevertProduct(productID, revision, user.ID, req.Reason)
	if err != nil {
		http.Error(w, "Failed to revert product: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Product reverted successfully",
	})
}

// GetRecentEdits handles getting recent edits across all products
func (h *Handler) GetRecentEdits(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter
	limit := 50 // Default limit

	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	edits, err := h.svc.GetRecentEdits(limit)
	if err != nil {
		http.Error(w, "Failed to get recent edits: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		RecentEdits []models.RevisionSummary `json:"recent_edits"`
		Count       int                      `json:"count"`
		Limit       int                      `json:"limit"`
	}{
		RecentEdits: edits,
		Count:       len(edits),
		Limit:       limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateProduct handles direct product updates with edit summaries
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
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

	// Log raw request body for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("UpdateProduct: Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	log.Printf("UpdateProduct: Received raw request body: %s", string(bodyBytes))

	// Restore request body so it can be read again
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Parse request body which should include both product data and edit summary
	var req struct {
		Product     models.Product `json:"product"`
		EditSummary string         `json:"edit_summary"`
		MinorEdit   bool           `json:"minor_edit,omitempty"`
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate edit summary is provided
	if req.EditSummary == "" {
		http.Error(w, "Edit summary is required", http.StatusBadRequest)
		return
	}

	// Ensure the product ID matches the URL parameter
	req.Product.ID = productID

	// Check if user is admin
	isAdmin := h.svc.IsUserAdmin(user.WalletAddress)

	err = h.svc.UpdateProduct(&req.Product, user.ID, req.EditSummary, req.MinorEdit, isAdmin)
	if err != nil {
		http.Error(w, "Failed to update product: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Product updated successfully",
	})
}

// SubmitProductEdit handles submitting edits to an existing product for admin review
func (h *Handler) SubmitProductEdit(w http.ResponseWriter, r *http.Request) {
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

	var product models.Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.svc.SubmitProductEdit(productID, &product, user.ID, user.WalletAddress)
	if err != nil {
		http.Error(w, "Failed to submit product edit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message":    "Product edit submitted for review",
		"product_id": productID,
		"status":     "pending",
	})
}

// GetUserProfile handles getting the current user's profile and admin status
func (h *Handler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	// Get user from context (middleware ensures user is authenticated)
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get full user data from database
	fullUser, err := h.svc.GetUserByWallet(user.WalletAddress)
	if err != nil {
		http.Error(w, "Failed to get user profile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is admin
	isAdmin := h.svc.IsUserAdmin(user.WalletAddress)

	// Prepare response with profile and admin status
	response := struct {
		*models.User
		IsAdmin bool `json:"is_admin"`
	}{
		User:    fullUser,
		IsAdmin: isAdmin,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserPermissions handles checking user permissions (curator/admin status)
func (h *Handler) GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	// Get user from context (middleware ensures user is authenticated)
	user, ok := r.Context().Value(middleware.UserContextKey).(*models.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check admin status
	isAdmin := h.svc.IsUserAdmin(user.WalletAddress)

	// For now, curator status is the same as admin status
	// In the future, this could be expanded to have separate curator roles
	isCurator := isAdmin

	response := struct {
		IsAdmin   bool `json:"is_admin"`
		IsCurator bool `json:"is_curator"`
	}{
		IsAdmin:   isAdmin,
		IsCurator: isCurator,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserVoteStates handles checking which products a user has voted for
func (h *Handler) GetUserVoteStates(w http.ResponseWriter, r *http.Request) {
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

	// Get product IDs from query parameter
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		http.Error(w, "Missing 'ids' parameter", http.StatusBadRequest)
		return
	}

	// Parse comma-separated product IDs
	productIDs := strings.Split(idsParam, ",")
	if len(productIDs) == 0 {
		http.Error(w, "No product IDs provided", http.StatusBadRequest)
		return
	}

	// Trim whitespace from each ID
	for i, id := range productIDs {
		productIDs[i] = strings.TrimSpace(id)
	}

	// Get vote states from Redis cache
	voteStates, err := h.svc.GetUserVoteStates(user.ID, productIDs)
	if err != nil {
		http.Error(w, "Failed to get vote states: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(voteStates)
}

// SubmitProductScore handles submitting or updating scores for a product
func (h *Handler) SubmitProductScore(w http.ResponseWriter, r *http.Request) {
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

	var scoreReq models.ScoreSubmissionRequest
	err := json.NewDecoder(r.Body).Decode(&scoreReq)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.svc.SubmitProductScore(productID, user.ID, &scoreReq)
	if err != nil {
		http.Error(w, "Failed to submit score: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated scores for this product
	count, overall, security, ux, vibes, err := h.svc.GetProductScoreStats(productID)
	if err != nil {
		http.Error(w, "Failed to get updated scores: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Message    string  `json:"message"`
		ScoreCount int     `json:"score_count"`
		Overall    float64 `json:"overall_score"`
		Security   float64 `json:"security_score"`
		UX         float64 `json:"ux_score"`
		Vibes      float64 `json:"vibes_score"`
	}{
		Message:    "Score submitted successfully",
		ScoreCount: count,
		Overall:    overall,
		Security:   security,
		UX:         ux,
		Vibes:      vibes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetMyProductScore handles getting the current user's score submission for a specific product
func (h *Handler) GetMyProductScore(w http.ResponseWriter, r *http.Request) {
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

	submission, err := h.svc.GetUserScoreSubmission(productID, user.ID)
	if err != nil {
		http.Error(w, "Failed to get your product score: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If no submission found, return 404 with message indicating they haven't submitted
	if submission == nil {
		response := struct {
			HasSubmitted bool   `json:"has_submitted"`
			Message      string `json:"message"`
		}{
			HasSubmitted: false,
			Message:      "You have not submitted a score for this product yet",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return the submission details
	response := struct {
		HasSubmitted bool                           `json:"has_submitted"`
		Submission   *models.ProductScoreSubmission `json:"submission"`
		Message      string                         `json:"message"`
	}{
		HasSubmitted: true,
		Submission:   submission,
		Message:      "Score submission found",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPendingChanges handles getting all pending changes from Redis (for admin dashboard)
func (h *Handler) GetPendingChanges(w http.ResponseWriter, r *http.Request) {
	// This endpoint replaces GetPendingEdits for the new Redis-based workflow
	pendingChanges, err := h.svc.GetAllPendingChanges()
	if err != nil {
		http.Error(w, "Failed to get pending changes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingChanges)
}

// GetPendingProduct handles getting a specific pending product from Redis
func (h *Handler) GetPendingProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	pendingProduct, err := h.svc.GetPendingProduct(id)
	if err != nil {
		http.Error(w, "Failed to get pending product: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingProduct)
}

// GetPendingEdit handles getting a specific pending edit from Redis
func (h *Handler) GetPendingEdit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	pendingEdit, err := h.svc.GetPendingEdit(id)
	if err != nil {
		http.Error(w, "Failed to get pending edit: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pendingEdit)
}

// GetTrendingProducts handles getting trending products
func (h *Handler) GetTrendingProducts(w http.ResponseWriter, r *http.Request) {
	// Get limit from query params
	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	// Use the optimized batch method to fetch trending products efficiently
	products, err := h.svc.GetTrendingProductsBatch(limit)
	if err != nil {
		http.Error(w, "Failed to get trending products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Note: User vote states are fetched separately via /api/user/vote-states
	// This keeps the trending endpoint fast and only fetches user data when needed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}
