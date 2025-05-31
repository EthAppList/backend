package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"github.com/wesjorgensen/EthAppList/backend/internal/config"
	"github.com/wesjorgensen/EthAppList/backend/internal/handlers"
	"github.com/wesjorgensen/EthAppList/backend/internal/middleware"
	"github.com/wesjorgensen/EthAppList/backend/internal/repository"
	"github.com/wesjorgensen/EthAppList/backend/internal/service"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Initialize configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}

	// Initialize PostgreSQL repository
	log.Println("Using PostgreSQL repository...")
	pgRepo, err := repository.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL repository: %v", err)
	}

	// Make sure to close the connection when done
	defer pgRepo.Close()

	// Initialize service layer
	svc := service.New(pgRepo, cfg)

	// Initialize router
	r := mux.NewRouter()

	// Apply middleware
	r.Use(middleware.Logging)
	r.Use(middleware.RequestID)

	// Set up API routes
	apiRouter := r.PathPrefix("/api").Subrouter()

	// Auth routes
	authRouter := apiRouter.PathPrefix("/auth").Subrouter()
	handlers.RegisterAuthHandlers(authRouter, svc)

	// Product routes
	productsRouter := apiRouter.PathPrefix("/products").Subrouter()
	handlers.RegisterProductHandlers(productsRouter, svc)

	// Category routes
	categoriesRouter := apiRouter.PathPrefix("/categories").Subrouter()
	handlers.RegisterCategoryHandlers(categoriesRouter, svc)

	// User routes
	userRouter := apiRouter.PathPrefix("/user").Subrouter()
	handlers.RegisterUserHandlers(userRouter, svc)

	// Admin routes
	adminRouter := apiRouter.PathPrefix("/admin").Subrouter()
	adminRouter.Use(middleware.AdminOnly(cfg))
	handlers.RegisterAdminHandlers(adminRouter, svc)

	// Temporary endpoint for testing - DELETE ALL PRODUCTS
	dropRouter := apiRouter.PathPrefix("/drop").Subrouter()
	dropRouter.Use(middleware.AdminOnly(cfg))
	dropRouter.HandleFunc("", handlers.New(svc).DeleteAllProducts).Methods("POST")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Enable CORS
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Update with your frontend domain in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), corsMiddleware.Handler(r)))
}
