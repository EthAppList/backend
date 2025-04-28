package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/wesjorgensen/EthAppList/backend/internal/config"
	"github.com/wesjorgensen/EthAppList/backend/internal/models"
)

// RequestIDKey is the context key for the request ID
type RequestIDKey string

// UserKey is the context key for the user information
type UserKey string

const (
	// RequestIDContextKey is the key used to store the request ID in the context
	RequestIDContextKey RequestIDKey = "requestID"
	// UserContextKey is the key used to store the user in the context
	UserContextKey UserKey = "user"
)

// Logging middleware logs request information
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Context().Value(RequestIDContextKey)
		if requestID == nil {
			requestID = "unknown"
		}

		log.Printf("[%s] %s %s started", requestID, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s completed in %v", requestID, r.Method, r.URL.Path, time.Since(start))
	})
}

// RequestID middleware adds a unique ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		ctx := context.WithValue(r.Context(), RequestIDContextKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth middleware handles authentication
func Auth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			// Extract the token from the Authorization header
			// Format: "Bearer {token}"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Parse and validate the token
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			if !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Extract claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// Create user from claims
			walletAddr, ok := claims["wallet"].(string)
			if !ok {
				http.Error(w, "Invalid token: missing wallet address", http.StatusUnauthorized)
				return
			}

			// Set user in context
			user := &models.User{
				WalletAddress: walletAddr,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminOnly middleware restricts access to admin users
func AdminOnly(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First apply Auth middleware to get the user
			Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Get the user from context
				user, ok := r.Context().Value(UserContextKey).(*models.User)
				if !ok {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Check if user is admin
				if user.WalletAddress != cfg.AdminWallet {
					http.Error(w, "Forbidden: admin access required", http.StatusForbidden)
					return
				}

				next.ServeHTTP(w, r)
			})).ServeHTTP(w, r)
		})
	}
}
