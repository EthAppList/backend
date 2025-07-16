#!/bin/sh

# Parse DATABASE_URL if provided (Railway style)
if [ -n "$DATABASE_URL" ]; then
  # Parse DATABASE_URL to extract components for logging
  DB_HOST=$(echo $DATABASE_URL | sed -n 's/.*@\([^:]*\).*/\1/p')
  DB_PORT=$(echo $DATABASE_URL | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
  DB_USER=$(echo $DATABASE_URL | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
  DB_NAME=$(echo $DATABASE_URL | sed -n 's/.*\/\([^?]*\).*/\1/p')
  
  echo "Using DATABASE_URL from Railway"
  echo "Connecting to: ${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
else
  echo "Using individual database environment variables"
  echo "Connecting to: ${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
fi

# Wait a moment for database to be ready (in case of docker-compose)
if [ "$ENVIRONMENT" != "production" ]; then
  echo "Waiting for database to be ready..."
  sleep 3
fi

# Start the application
# The Go application will handle database migrations automatically
echo "Starting the application..."
exec ./app 