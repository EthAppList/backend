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

	// Revision system endpoints
	router.HandleFunc("/{id}/history", h.GetProductHistory).Methods("GET")
	router.HandleFunc("/{id}/revisions/{revision}", h.GetProductRevision).Methods("GET")
	router.HandleFunc("/{id}/compare/{rev1}/{rev2}", h.CompareProductRevisions).Methods("GET")

	// Protected routes
	protectedRouter := router.NewRoute().Subrouter()
	protectedRouter.Use(middleware.Auth(svc.GetConfig()))

	protectedRouter.HandleFunc("", h.SubmitProduct).Methods("POST")
	protectedRouter.HandleFunc("/{id}/upvote", h.UpvoteProduct).Methods("POST")
	protectedRouter.HandleFunc("/{id}", h.UpdateProduct).Methods("PUT")

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

// RegisterAdminHandlers registers admin-related routes
func RegisterAdminHandlers(router *mux.Router, svc *service.Service) {
	h := New(svc)

	router.HandleFunc("/pending", h.GetPendingEdits).Methods("GET")
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

	// Parse request body which should include both product data and edit summary
	var req struct {
		Product     models.Product `json:"product"`
		EditSummary string         `json:"edit_summary"`
		MinorEdit   bool           `json:"minor_edit,omitempty"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
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

	err = h.svc.UpdateProduct(&req.Product, user.ID, req.EditSummary, req.MinorEdit)
	if err != nil {
		http.Error(w, "Failed to update product: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Product updated successfully",
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
