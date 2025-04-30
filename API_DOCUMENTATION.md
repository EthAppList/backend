# API Documentation

This document provides detailed information about the API endpoints available in the Crypto Products Backend.

## Base URL

- Local Development: `http://localhost:8080/api`
- Production: `https://backend-production-29b8.up.railway.app/api`

## Authentication

Most endpoints that modify data require authentication. Authentication is done via JWT tokens.

### Authenticate Wallet

```
POST /auth/wallet
```

**Request:**
```json
{
  "wallet_address": "0x123abc...",
  "signature": "0x456def...",
  "message": "Sign this message to authenticate with Crypto Products"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Authentication:** None required

**Status Codes:**
- 200: Success
- 401: Authentication failed
- 400: Invalid request body

## Products

### Get Products

Retrieve a list of products with optional filtering.

```
GET /products
```

**Query Parameters:**
- `category`: Filter by category ID
- `chain`: Filter by chain ID
- `search`: Search term to filter by title or description
- `sort`: Sorting option (`new`, `top_day`, `top_week`, `top_month`, `top_year`, `top_all`)
- `page`: Page number (default: 1)
- `per_page`: Items per page (default: 10)

**Response:**
```json
{
  "products": [
    {
      "id": "1627590420123456789",
      "title": "Product Name",
      "short_desc": "Short description",
      "long_desc": "Longer description",
      "logo_url": "https://example.com/logo.png",
      "markdown_content": "# Markdown Content\n\nMore details about the product.",
      "submitter_id": "1627590420987654321",
      "approved": true,
      "is_verified": false,
      "analytics_list": ["<iframe src=\"https://example.com/analytics1\"></iframe>"],
      "security_score": 0.75,
      "ux_score": 0.85,
      "decent_score": 0.65,
      "vibes_score": 0.90,
      "created_at": "2023-01-01T12:00:00Z",
      "updated_at": "2023-01-01T12:00:00Z",
      "categories": [
        {
          "id": "1",
          "name": "DeFi",
          "description": "Decentralized Finance applications"
        }
      ],
      "chains": [
        {
          "id": "1",
          "name": "Ethereum",
          "icon": "https://cryptologos.cc/logos/ethereum-eth-logo.png"
        }
      ],
      "upvote_count": 42,
      "submitter": {
        "id": "1627590420987654321",
        "wallet_address": "0x123abc..."
      }
    }
  ],
  "total": 42,
  "page": 1,
  "per_page": 10,
  "pages": 5
}
```

**Authentication:** None required

**Status Codes:**
- 200: Success
- 500: Server error

### Get Product by ID

Retrieve a specific product by its ID.

```
GET /products/{id}
```

**Response:**
```json
{
  "id": "1627590420123456789",
  "title": "Product Name",
  "short_desc": "Short description",
  "long_desc": "Longer description",
  "logo_url": "https://example.com/logo.png",
  "markdown_content": "# Markdown Content\n\nMore details about the product.",
  "submitter_id": "1627590420987654321",
  "approved": true,
  "is_verified": false,
  "analytics_list": ["<iframe src=\"https://example.com/analytics1\"></iframe>"],
  "security_score": 0.75,
  "ux_score": 0.85,
  "decent_score": 0.65,
  "vibes_score": 0.90,
  "created_at": "2023-01-01T12:00:00Z",
  "updated_at": "2023-01-01T12:00:00Z",
  "categories": [
    {
      "id": "1",
      "name": "DeFi",
      "description": "Decentralized Finance applications"
    }
  ],
  "chains": [
    {
      "id": "1",
      "name": "Ethereum",
      "icon": "https://cryptologos.cc/logos/ethereum-eth-logo.png"
    }
  ],
  "upvote_count": 42,
  "submitter": {
    "id": "1627590420987654321",
    "wallet_address": "0x123abc..."
  }
}
```

**Authentication:** None required

**Status Codes:**
- 200: Success
- 404: Product not found
- 500: Server error

### Submit Product

Submit a new product.

```
POST /products
```

**Request:**
```json
{
  "title": "New Product",
  "short_desc": "Short description",
  "long_desc": "Longer description",
  "logo_url": "https://example.com/logo.png",
  "markdown_content": "# Markdown Content\n\nMore details about the product.",
  "is_verified": false,
  "analytics_list": ["<iframe src=\"https://example.com/analytics1\"></iframe>"],
  "security_score": 0.75,
  "ux_score": 0.85,
  "decent_score": 0.65,
  "vibes_score": 0.90,
  "categories": [
    {"id": "1"}
  ],
  "chains": [
    {"id": "1"}
  ]
}
```

**Response:**
The created product object

**Authentication:** Required

**Status Codes:**
- 201: Created
- 400: Invalid request
- 401: Unauthorized
- 500: Server error

### Upvote Product

Upvote a product (one upvote per user per product).

```
POST /products/{id}/upvote
```

**Authentication:** Required

**Status Codes:**
- 204: Upvote successful (No Content)
- 401: Unauthorized
- 404: Product not found
- 409: Already upvoted
- 500: Server error

## Categories

### Get Categories

Retrieve a list of all categories.

```
GET /categories
```

**Response:**
```json
[
  {
    "id": "1",
    "name": "DeFi",
    "description": "Decentralized Finance applications",
    "created_at": "2023-01-01T12:00:00Z",
    "updated_at": "2023-01-01T12:00:00Z",
    "product_count": 42
  }
]
```

**Authentication:** None required

**Status Codes:**
- 200: Success
- 500: Server error

### Submit Category

Create a new category.

```
POST /categories
```

**Request:**
```json
{
  "name": "New Category",
  "description": "Description of the new category"
}
```

**Response:**
The created category object

**Authentication:** Required

**Status Codes:**
- 201: Created
- 400: Invalid request
- 401: Unauthorized
- 500: Server error

## Admin Endpoints

Admin endpoints require both authentication and admin privileges.

### Get Pending Edits

Get a list of pending edits awaiting approval.

```
GET /admin/pending
```

**Response:**
```json
[
  {
    "id": "1627590420123456789",
    "user_id": "1627590420987654321",
    "entity_type": "product",
    "entity_id": "1627590420123456789",
    "change_type": "create",
    "change_data": "{\"title\":\"New Product\",...}",
    "status": "pending",
    "created_at": "2023-01-01T12:00:00Z"
  }
]
```

**Authentication:** Admin only

**Status Codes:**
- 200: Success
- 401: Unauthorized
- 403: Forbidden (not an admin)
- 500: Server error

### Approve Edit

Approve a pending edit.

```
POST /admin/approve/{id}
```

**Authentication:** Admin only

**Status Codes:**
- 204: Approved successfully (No Content)
- 401: Unauthorized
- 403: Forbidden (not an admin)
- 404: Edit not found
- 500: Server error

### Reject Edit

Reject a pending edit.

```
POST /admin/reject/{id}
```

**Authentication:** Admin only

**Status Codes:**
- 204: Rejected successfully (No Content)
- 401: Unauthorized
- 403: Forbidden (not an admin)
- 404: Edit not found
- 500: Server error

### Delete All Products (Temporary Testing Endpoint)

Delete all products from the database for testing purposes.

```
POST /drop
```

**Authentication:** Admin only

**Status Codes:**
- 200: Products deleted successfully
- 401: Unauthorized
- 403: Forbidden (not an admin)
- 500: Server error

## Utility Endpoints

### Health Check

Check if the API is running.

```
GET /health
```

**Response:**
```
OK
```

**Authentication:** None required

**Status Codes:**
- 200: Service is healthy 