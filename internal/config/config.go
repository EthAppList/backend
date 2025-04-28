package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// Config holds all application configuration
type Config struct {
	// JWT configuration
	JWTSecret string

	// Server configuration
	Port        string
	Environment string

	// Admin configuration
	AdminWallet string

	// Database configuration
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

// New creates a new configuration from environment variables
func New() (*Config, error) {
	// Check if we're running on Railway with DATABASE_URL
	databaseURL := os.Getenv("DATABASE_URL")

	// Parse Railway DATABASE_URL if available
	var dbHost, dbPort, dbUser, dbPassword, dbName string
	if databaseURL != "" {
		// Parse the URL
		parsedURL, err := url.Parse(databaseURL)
		if err == nil {
			// Extract host and port
			hostParts := strings.Split(parsedURL.Host, ":")
			if len(hostParts) > 0 {
				dbHost = hostParts[0]
			}
			if len(hostParts) > 1 {
				dbPort = hostParts[1]
			}

			// Extract user and password
			if parsedURL.User != nil {
				dbUser = parsedURL.User.Username()
				dbPassword, _ = parsedURL.User.Password()
			}

			// Extract database name
			if len(parsedURL.Path) > 1 {
				dbName = parsedURL.Path[1:] // Remove leading '/'
			}
		}
	}

	// If no DATABASE_URL, use individual environment variables
	if dbHost == "" {
		dbHost = os.Getenv("DB_HOST")
	}
	if dbPort == "" {
		dbPort = os.Getenv("DB_PORT")
	}
	if dbUser == "" {
		dbUser = os.Getenv("DB_USER")
	}
	if dbPassword == "" {
		dbPassword = os.Getenv("DB_PASSWORD")
	}
	if dbName == "" {
		dbName = os.Getenv("DB_NAME")
	}

	// Require database connection details
	if dbHost == "" {
		return nil, errors.New("DB_HOST or DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}

	adminWallet := os.Getenv("ADMIN_WALLET_ADDRESS")
	if adminWallet == "" {
		return nil, errors.New("ADMIN_WALLET_ADDRESS is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development" // Default environment
	}

	return &Config{
		JWTSecret:   jwtSecret,
		Port:        port,
		Environment: environment,
		AdminWallet: adminWallet,
		DBHost:      dbHost,
		DBPort:      dbPort,
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBName:      dbName,
	}, nil
}

// IsDevelopment returns true if the environment is set to development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if the environment is set to production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}
