# Crypto Products Backend

A Go backend for a crypto product listing website (similar to Product Hunt, but for crypto products exclusively) using PostgreSQL for the database.

## Features

- **Wallet-based Authentication**: Users sign in with their crypto wallet and make a signature for authentication
- **JWT Tokens**: Generated after successful wallet signature verification for session management
- **Product Listings**: Store products with title, descriptions, logo, and markdown content
- **Dynamic Categories**: Allow users to submit products under existing or new categories
- **Chain Filtering**: Products can be associated with multiple blockchain networks
- **Upvote System**: Users can upvote products they like (one upvote per product per user)
- **Time-based Sorting**: View products sorted by new or top upvotes (day, week, year, all-time)
- **Moderation System**: Admin approval required for product submissions and edits
- **Image Storage**: Support for product logos and images in markdown content
- **Search Functionality**: Find products by title, description, category, or chain

## Project Structure

```
.
├── cmd/
│   └── server/           # Application entry point
├── internal/
│   ├── auth/             # Authentication logic
│   ├── config/           # Configuration management
│   ├── handlers/         # HTTP handlers
│   ├── middleware/       # HTTP middleware
│   ├── models/           # Data models
│   ├── repository/       # Database interactions
│   └── service/          # Business logic
├── migrations/           # Database migrations
├── scripts/              # Helper scripts
└── web/                  # Web assets (if any)
```

## Prerequisites

- Docker and Docker Compose (for Docker deployment)
- Go 1.21 or newer (for local development)
- PostgreSQL 14 or newer (for local development without Docker)

## Setup

### Option 1: Docker Setup (Recommended)

1. Clone this repository:

```bash
git clone https://github.com/yourusername/crypto-products-backend.git
cd crypto-products-backend
```

2. Update environment variables in docker-compose.yml:

```bash
# Edit docker-compose.yml with your preferred text editor
# Update values for:
# - DB_PASSWORD/POSTGRES_PASSWORD
# - JWT_SECRET
# - ADMIN_WALLET_ADDRESS
```

3. Start the containers:

```bash
docker-compose up -d
```

4. The application will be available at http://localhost:8080

### Option 2: Manual Setup

1. Clone this repository:

```bash
git clone https://github.com/yourusername/crypto-products-backend.git
cd crypto-products-backend
```

2. Install dependencies:

```bash
go mod download
```

3. Set up PostgreSQL:

```bash
# For Ubuntu/Debian
sudo apt install postgresql postgresql-contrib

# For Arch Linux
sudo pacman -S postgresql

# Start the PostgreSQL service
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

4. Create a database and user:

```bash
sudo -u postgres psql
```

In the PostgreSQL shell:

```sql
CREATE USER crypto_admin WITH PASSWORD 'your_secure_password';
CREATE DATABASE crypto_products;
GRANT ALL PRIVILEGES ON DATABASE crypto_products TO crypto_admin;
\q
```

5. Create a `.env` file based on `.env.example` and configure your database connection:

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=crypto_admin
DB_PASSWORD=your_secure_password
DB_NAME=crypto_products
JWT_SECRET=your_jwt_secret
ADMIN_WALLET_ADDRESS=your_admin_wallet_address
PORT=8080
ENVIRONMENT=development

# Optional: Supabase fallback configuration (if needed)
SUPABASE_URL=
SUPABASE_KEY=
SUPABASE_ANON_KEY=
```

6. Set up the database schema:

```bash
go run scripts/setup_db.go
```

7. Build and run the server:

```bash
go build -o app cmd/server/main.go
./app
```

Alternatively, run it directly:

```bash
go run cmd/server/main.go
```

## API Endpoints

### Authentication

- `POST /api/auth/wallet` - Authenticate with a wallet signature
  - Request: `{"wallet_address": "0x...", "signature": "0x...", "message": "..."}`
  - Response: `{"token": "jwt_token_here"}`

### Products

- `GET /api/products` - Get products (with filter options)
  - Query parameters:
    - `category` - Filter by category ID
    - `chain` - Filter by chain ID
    - `search` - Search term for title/description
    - `sort` - Sorting options: `new`, `top_day`, `top_week`, `top_month`, `top_year`, `top_all`
    - `page` - Page number (default: 1)
    - `per_page` - Items per page (default: 10)
  - Response: 
    ```json
    {
      "products": [Product],
      "meta": {
        "total": 100,
        "page": 1,
        "per_page": 10,
        "pages": 10
      }
    }
    ```

- `GET /api/products/{id}` - Get a specific product
  - Response: Product object with categories, chains, and upvote count

- `POST /api/products` - Submit a new product (requires auth)
  - Request: Product object with categories and chains
  - Response: Created product object

- `POST /api/products/{id}/upvote` - Upvote a product (requires auth)
  - Response: 204 No Content on success

### Categories

- `GET /api/categories` - Get all categories
  - Response: Array of category objects with product counts

- `POST /api/categories` - Submit a new category (requires auth)
  - Request: `{"name": "Category Name", "description": "Description"}`
  - Response: Created category object

### Admin Endpoints

- `GET /api/admin/pending` - Get pending submissions (admin only)
  - Response: Array of pending edit objects

- `POST /api/admin/approve/{id}` - Approve a submission (admin only)
  - Response: 204 No Content on success

- `POST /api/admin/reject/{id}` - Reject a submission (admin only)
  - Response: 204 No Content on success

## Data Models

### Product

```json
{
  "id": "string",
  "title": "string",
  "short_desc": "string",
  "long_desc": "string",
  "logo_url": "string",
  "markdown_content": "string",
  "submitter_id": "string",
  "approved": boolean,
  "created_at": "timestamp",
  "updated_at": "timestamp",
  "categories": [Category],
  "chains": [Chain],
  "upvote_count": number
}
```

### Category

```json
{
  "id": "string",
  "name": "string",
  "description": "string",
  "created_at": "timestamp",
  "updated_at": "timestamp",
  "product_count": number
}
```

### Chain

```json
{
  "id": "string",
  "name": "string",
  "icon": "string",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

## Database Integration

This backend uses PostgreSQL for all database operations:

1. **Database Schema**: Tables for users, products, categories, chains, upvotes, etc.
2. **Indexes**: For optimized query performance
3. **Foreign Keys**: To maintain data integrity
4. **SQL Functions**: For complex queries like finding top products by period

### Database Schema

The database schema includes the following tables:

- `users` - User profiles
- `products` - Product listings
- `categories` - Product categories
- `product_categories` - Many-to-many relationship between products and categories
- `chains` - Blockchain networks
- `product_chains` - Many-to-many relationship between products and chains
- `upvotes` - Product upvotes with user reference
- `pending_edits` - Queue for submissions pending approval

## Frontend Integration

To integrate with a Next.js frontend:

1. Make API calls to the endpoints described above
2. For authentication, implement wallet signature generation (e.g., using ethers.js)
3. Store JWT token in cookies or localStorage
4. Use token for authenticated requests

## Development

```bash
# Run with hot reload
go run cmd/server/main.go

# Run tests
go test ./...

# Check for linting issues
go vet ./...

# Start Docker development environment
docker-compose up
```

## Deployment

For detailed deployment instructions, please see the [DEPLOYMENT.md](DEPLOYMENT.md) file, which includes:

1. **Docker-based Deployment**: The easiest way to deploy with Docker and Docker Compose
2. **Traditional Deployment**: Step-by-step instructions for deploying on Arch Linux without Docker

## License

[MIT License](LICENSE)

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request 