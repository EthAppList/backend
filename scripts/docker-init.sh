#!/bin/sh

# Check if DATABASE_URL is provided (Railway style)
if [ -n "$DATABASE_URL" ]; then
  # Parse DATABASE_URL to extract components
  DB_HOST=$(echo $DATABASE_URL | sed -n 's/.*@\([^:]*\).*/\1/p')
  DB_PORT=$(echo $DATABASE_URL | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
  DB_USER=$(echo $DATABASE_URL | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
  DB_PASSWORD=$(echo $DATABASE_URL | sed -n 's/.*:\/\/[^:]*:\([^@]*\).*/\1/p')
  DB_NAME=$(echo $DATABASE_URL | sed -n 's/.*\/\([^?]*\).*/\1/p')
  
  # Export parsed values as environment variables
  export DB_HOST=$DB_HOST
  export DB_PORT=$DB_PORT
  export DB_USER=$DB_USER
  export DB_PASSWORD=$DB_PASSWORD
  export DB_NAME=$DB_NAME
  
  echo "Using DATABASE_URL from Railway"
fi

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL to start..."
sleep 5

echo "Testing connection to PostgreSQL..."
while ! nc -z ${DB_HOST:-postgres} ${DB_PORT:-5432}; do
  echo "PostgreSQL not available yet - waiting..."
  sleep 2
done
echo "PostgreSQL is up and running!"

# Initialize the database using the database setup script directly
echo "Setting up the database schema..."
cd /root && ./scripts/db/setup_postgres.sh

# Start the application
echo "Starting the application..."
exec ./app 