#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# API base URL - modify as needed
BASE_URL="http://localhost:8080/api"
API_URL="$BASE_URL"
PRODUCTION_URL="https://backend-production-29b8.up.railway.app/api"

# Prompt for environment
echo -e "${BLUE}Which environment would you like to test?${NC}"
echo "1) Local ($BASE_URL)"
echo "2) Production ($PRODUCTION_URL)"
read -p "Enter your choice (1/2): " env_choice

if [ "$env_choice" = "2" ]; then
  API_URL="$PRODUCTION_URL"
  echo -e "${YELLOW}Using production URL: $API_URL${NC}"
else
  echo -e "${YELLOW}Using local URL: $API_URL${NC}"
fi

# Prompt for JWT token
echo -e "${BLUE}Please enter your JWT token (from frontend):${NC}"
read -p "JWT token: " JWT_TOKEN

# Function to test an endpoint
test_endpoint() {
  local method=$1
  local endpoint=$2
  local description=$3
  local auth_required=${4:-false}
  local data=${5:-""}
  local expected_status=${6:-200}

  echo -e "\n${BLUE}Testing: $description${NC}"
  echo "$method $endpoint"

  # Build the curl command
  cmd="curl -s -X $method -w \"%{http_code}\" \"$API_URL$endpoint\""
  
  # Add auth header if required
  if [ "$auth_required" = true ]; then
    cmd="$cmd -H \"Authorization: Bearer $JWT_TOKEN\""
  fi
  
  # Add content-type and data if needed
  if [ -n "$data" ]; then
    cmd="$cmd -H \"Content-Type: application/json\" -d '$data'"
  fi
  
  # Execute the command and capture response and status code
  result=$(eval $cmd)
  status_code=${result: -3}
  response=${result:0:${#result}-3}

  # Check if status code is as expected
  if [ "$status_code" -eq "$expected_status" ]; then
    echo -e "${GREEN}✓ Success (Status: $status_code)${NC}"
  else
    echo -e "${RED}✗ Failed (Status: $status_code)${NC}"
  fi
  
  # Pretty print JSON response if valid
  if [[ "$response" == "{"* || "$response" == "["* ]]; then
    echo "Response:"
    echo "$response" | python -m json.tool
  else
    echo "Response: $response"
  fi
}

# Function to run all tests
run_tests() {
  echo -e "\n${BLUE}===== Starting API Tests =====${NC}"
  
  # Health check (outside of /api)
  test_endpoint "GET" "/health" "Health Check" false "" 200
  
  ## AUTH ENDPOINTS ##
  echo -e "\n${YELLOW}===== AUTH ENDPOINTS =====${NC}"
  
  # Note: We can't fully test the authentication process since it requires a valid signature
  # But we can test the API structure
  test_endpoint "POST" "/auth/wallet" "Authenticate Wallet (should fail without proper signature)" false '{"wallet_address": "0x123", "signature": "invalid", "message": "test"}' 401
  
  ## PRODUCT ENDPOINTS ##
  echo -e "\n${YELLOW}===== PRODUCT ENDPOINTS =====${NC}"
  
  # Get products (public)
  test_endpoint "GET" "/products" "Get Products" false
  
  # Test filtering products
  test_endpoint "GET" "/products?page=1&per_page=5" "Get Products with Pagination" false
  test_endpoint "GET" "/products?sort=new" "Get Products Sorted by New" false
  
  # Try to get a single product (need to extract an ID from the list first)
  product_id=$(curl -s "$API_URL/products" | python -c "import sys, json; data=json.load(sys.stdin); print(data['products'][0]['id'] if data.get('products') and len(data['products']) > 0 else '')" 2>/dev/null)
  
  if [ -n "$product_id" ]; then
    test_endpoint "GET" "/products/$product_id" "Get Product by ID ($product_id)" false
  else
    echo -e "${YELLOW}No products found to test Get Product by ID${NC}"
  fi
  
  # Submit a product (authenticated)
  test_product='{
    "title": "Test Product",
    "short_desc": "A test product created via API test script",
    "long_desc": "This is a longer description of the test product",
    "logo_url": "https://example.com/logo.png",
    "markdown_content": "# Test Product\n\nThis is a test product description with markdown.",
    "categories": [{"id": "1"}],
    "chains": [{"id": "1"}]
  }'
  
  test_endpoint "POST" "/products" "Submit Product" true "$test_product" 201
  
  # Try to upvote a product (authenticated) if we have a product ID
  if [ -n "$product_id" ]; then
    test_endpoint "POST" "/products/$product_id/upvote" "Upvote Product" true "" 204
  else
    echo -e "${YELLOW}No products found to test Upvote${NC}"
  fi
  
  ## CATEGORY ENDPOINTS ##
  echo -e "\n${YELLOW}===== CATEGORY ENDPOINTS =====${NC}"
  
  # Get categories (public)
  test_endpoint "GET" "/categories" "Get Categories" false
  
  # Submit a category (authenticated)
  test_category='{
    "name": "Test Category",
    "description": "A test category created via API test script"
  }'
  
  test_endpoint "POST" "/categories" "Submit Category" true "$test_category" 201
  
  ## ADMIN ENDPOINTS ##
  echo -e "\n${YELLOW}===== ADMIN ENDPOINTS =====${NC}"
  
  # Get pending edits (admin only)
  test_endpoint "GET" "/admin/pending" "Get Pending Edits" true
  
  # Get a pending edit ID if available
  pending_id=$(curl -s -H "Authorization: Bearer $JWT_TOKEN" "$API_URL/admin/pending" | python -c "import sys, json; data=json.load(sys.stdin); print(data[0]['id'] if data and len(data) > 0 else '')" 2>/dev/null)
  
  # Try to approve a pending edit if available
  if [ -n "$pending_id" ]; then
    test_endpoint "POST" "/admin/approve/$pending_id" "Approve Pending Edit" true "" 204
  else
    echo -e "${YELLOW}No pending edits found to test approve/reject${NC}"
  fi
  
  echo -e "\n${BLUE}===== API Tests Completed =====${NC}"
}

# Run all tests
run_tests 