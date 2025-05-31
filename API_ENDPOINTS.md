# EthAppList Backend API Endpoints

This document provides a comprehensive overview of all available API endpoints in the EthAppList backend.

## Base URL
The API is served at `/api` with the following route structure.

## Authentication
Protected endpoints require JWT token authentication via the `Authorization` header:
```
Authorization: Bearer <token>
```

Admin endpoints require additional admin privileges.

---

## Health Check

### GET `/health`
Health check endpoint to verify server status.

**Authentication:** None  
**Response:** `200 OK` with "OK" message

---

## Authentication Endpoints

### POST `/api/auth/wallet`
Authenticate a user via wallet signature.

**Authentication:** None  
**Request Body:**
```json
{
  "wallet_address": "string",
  "signature": "string", 
  "message": "string"
}
```

**Response:**
```json
{
  "token": "string"
}
```

---

## User Endpoints

### GET `/api/user/profile` 🔒
Get current user profile and admin status.

**Authentication:** Required  
**Response:**
```json
{
  "id": "string",
  "wallet_address": "string",
  "twitter_handle": "string",
  "created_at": "timestamp",
  "updated_at": "timestamp",
  "submitted_products": "integer",
  "upvotes": "integer",
  "is_admin": "boolean"
}
```

### GET `/api/user/permissions` 🔒
Check user permissions (curator/admin status).

**Authentication:** Required  
**Response:**
```json
{
  "is_admin": "boolean",
  "is_curator": "boolean"
}
```

---

## Product Endpoints

### GET `/api/products`
Get all products with optional filtering and pagination.

**Authentication:** None  
**Query Parameters:**
- `category` (optional): Filter by category ID
- `chain` (optional): Filter by blockchain/chain ID
- `search` (optional): Search term for products
- `sort` (optional): Sort option (default: "new")
- `page` (optional): Page number (default: 1)
- `per_page` (optional): Items per page (default: 10)

**Response:**
```json
{
  "products": [Product],
  "total": "integer",
  "page": "integer", 
  "per_page": "integer",
  "pages": "integer"
}
```

### GET `/api/products/{id}`
Get a specific product by ID.

**Authentication:** None  
**Path Parameters:**
- `id`: Product ID

**Response:** Product object

### POST `/api/products` 🔒
Submit a new product.

**Authentication:** Required  
**Request Body:** Product object

**Response:** `201 Created` with created product object

### PUT `/api/products/{id}` 🔒
Update an existing product with revision tracking.

**Authentication:** Required  
**Path Parameters:**
- `id`: Product ID

**Request Body:**
```json
{
  "product": Product,
  "edit_summary": "string",
  "minor_edit": "boolean (optional)"
}
```

**Response:** Success message

### POST `/api/products/{id}/upvote` 🔒
Upvote a product.

**Authentication:** Required  
**Path Parameters:**
- `id`: Product ID

**Response:** `204 No Content`

### GET `/api/products/{id}/history`
Get edit history for a product.

**Authentication:** None  
**Path Parameters:**
- `id`: Product ID

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `per_page` (optional): Items per page (default: 20, max: 100)

**Response:**
```json
{
  "revisions": [RevisionSummary],
  "total": "integer",
  "page": "integer",
  "per_page": "integer", 
  "pages": "integer"
}
```

### GET `/api/products/{id}/revisions/{revision}`
Get a specific revision of a product.

**Authentication:** None  
**Path Parameters:**
- `id`: Product ID
- `revision`: Revision number

**Response:** Product revision object

### GET `/api/products/{id}/compare/{rev1}/{rev2}`
Compare two revisions of a product.

**Authentication:** None  
**Path Parameters:**
- `id`: Product ID
- `rev1`: First revision number
- `rev2`: Second revision number

**Response:** Diff object showing changes between revisions

### POST `/api/products/{id}/revert/{revision}` 🔒
Revert a product to a specific revision.

**Authentication:** Required (Admin-level permissions recommended)  
**Path Parameters:**
- `id`: Product ID
- `revision`: Target revision number

**Request Body:**
```json
{
  "reason": "string (optional)"
}
```

**Response:** Success message

---

## Category Endpoints

### GET `/api/categories`
Get all categories.

**Authentication:** None  
**Response:** Array of category objects

### POST `/api/categories` 🔒
Submit a new category.

**Authentication:** Required  
**Request Body:** Category object

**Response:** `201 Created` with created category object

---

## Admin Endpoints 🔐

All admin endpoints require admin privileges.

### GET `/api/admin/pending`
Get all pending edits awaiting approval.

**Authentication:** Admin required  
**Response:** Array of pending edit objects

### POST `/api/admin/approve/{id}`
Approve a pending edit.

**Authentication:** Admin required  
**Path Parameters:**
- `id`: Edit ID

**Response:** `204 No Content`

### POST `/api/admin/reject/{id}`
Reject a pending edit.

**Authentication:** Admin required  
**Path Parameters:**
- `id`: Edit ID

**Response:** `204 No Content`

### GET `/api/admin/recent-edits`
Get recent edits across all products.

**Authentication:** Admin required  
**Query Parameters:**
- `limit` (optional): Number of edits to return (default: 50, max: 200)

**Response:**
```json
{
  "recent_edits": [RevisionSummary],
  "count": "integer",
  "limit": "integer"
}
```

---

## Testing/Development Endpoints 🔐

### POST `/api/drop` 
**⚠️ DANGEROUS - Delete all products (testing only)**

**Authentication:** Admin required  
**Response:**
```json
{
  "message": "All products have been deleted successfully"
}
```

---

## Response Codes

- `200 OK`: Successful GET request
- `201 Created`: Successful POST request creating a resource
- `204 No Content`: Successful request with no response body
- `400 Bad Request`: Invalid request format or parameters
- `401 Unauthorized`: Authentication required or failed
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `409 Conflict`: Conflict with current state (e.g., already upvoted)
- `500 Internal Server Error`: Server-side error

---

## Legend

- 🔒 = Authentication required
- 🔐 = Admin authentication required
- ⚠️ = Dangerous operation (use with caution) 