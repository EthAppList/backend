#!/bin/sh

# Check if Railway's DATABASE_URL is available and use it directly if so
if [ -n "$DATABASE_URL" ]; then
    DB_CONN="$DATABASE_URL"
    echo "Using DATABASE_URL from Railway"
else
    # Set PostgreSQL connection details from environment variables
    export PGHOST=${DB_HOST:-postgres}
    export PGPORT=${DB_PORT:-5432}
    export PGUSER=${DB_USER:-postgres}
    export PGPASSWORD=${DB_PASSWORD:-your_secure_password}
    export PGDATABASE=${DB_NAME:-crypto_products}
    
    # Set DB connection string
    DB_CONN="postgresql://$PGUSER:$PGPASSWORD@$PGHOST:$PGPORT/$PGDATABASE"
    echo "Using standard PostgreSQL connection variables"
fi

# Wait for PostgreSQL to be ready
echo "Checking if PostgreSQL is ready..."
pg_isready -h ${PGHOST:-$DB_HOST} -p ${PGPORT:-$DB_PORT} -U ${PGUSER:-$DB_USER}

# Check if the tables already exist
TABLES=$(psql $DB_CONN -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'")

if [ "$TABLES" -gt "0" ]; then
    echo "Database already initialized, skipping setup"
else
    echo "Initializing database with schema..."
    
    # Read and execute the SQL migration file
    if [ -f "/root/migrations/init.sql" ]; then
        psql $DB_CONN -f "/root/migrations/init.sql"
        echo "Database initialization completed successfully"
    else
        echo "Error: Migration file not found!"
        exit 1
    fi
fi 