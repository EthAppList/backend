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
    echo "Database already has tables, running additional migrations..."
    
    # Check if is_verified column exists (from add_product_fields.sql)
    HAS_IS_VERIFIED=$(psql $DB_CONN -t -c "SELECT COUNT(*) FROM information_schema.columns WHERE table_name='products' AND column_name='is_verified'")
    if [ "$HAS_IS_VERIFIED" -eq "0" ]; then
        echo "Running product fields migration..."
        if [ -f "/root/migrations/add_product_fields.sql" ]; then
            psql $DB_CONN -f "/root/migrations/add_product_fields.sql"
            echo "Product fields migration completed"
        fi
    fi
    
    # Check if product_revisions table exists (from add_product_revisions.sql)
    HAS_REVISIONS=$(psql $DB_CONN -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_name='product_revisions'")
    if [ "$HAS_REVISIONS" -eq "0" ]; then
        echo "Running product revisions migration..."
        if [ -f "/root/migrations/add_product_revisions.sql" ]; then
            psql $DB_CONN -f "/root/migrations/add_product_revisions.sql"
            echo "Product revisions migration completed"
        fi
    fi
    
    echo "Migration check completed"
else
    echo "Initializing database with schema..."
    
    # Read and execute all SQL migration files in order
    echo "Running initial migration..."
    if [ -f "/root/migrations/init.sql" ]; then
        psql $DB_CONN -f "/root/migrations/init.sql"
        echo "Initial migration completed"
    else
        echo "Error: init.sql migration file not found!"
        exit 1
    fi
    
    echo "Running product fields migration..."
    if [ -f "/root/migrations/add_product_fields.sql" ]; then
        psql $DB_CONN -f "/root/migrations/add_product_fields.sql"
        echo "Product fields migration completed"
    else
        echo "Warning: add_product_fields.sql migration file not found!"
    fi
    
    echo "Running product revisions migration..."
    if [ -f "/root/migrations/add_product_revisions.sql" ]; then
        psql $DB_CONN -f "/root/migrations/add_product_revisions.sql"
        echo "Product revisions migration completed"
    else
        echo "Warning: add_product_revisions.sql migration file not found!"
    fi
    
    echo "All database migrations completed successfully"
fi 