#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# API base URL - modify as needed
BASE_URL="http://localhost:8080"
API_URL="$BASE_URL/api"
PRODUCTION_URL="https://backend-production-29b8.up.railway.app"
PRODUCTION_API_URL="$PRODUCTION_URL/api"

# Prompt for environment
echo -e "${BLUE}Which environment would you like to test?${NC}"
echo "1) Local ($BASE_URL)"
echo "2) Production ($PRODUCTION_URL)"
read -p "Enter your choice (1/2): " env_choice

if [ "$env_choice" = "2" ]; then
  API_URL="$PRODUCTION_API_URL"
  BASE_URL="$PRODUCTION_URL"
  echo -e "${YELLOW}Using production URL: $PRODUCTION_URL${NC}"
else
  echo -e "${YELLOW}Using local URL: $BASE_URL${NC}"
fi

# Prompt for JWT token
echo -e "${BLUE}Please enter your JWT token (from frontend):${NC}"
read -p "JWT token: " JWT_TOKEN

# Variables to store test data
CREATED_PRODUCT_ID=""
CREATED_PRODUCT_RESPONSE=""

# Function to test an endpoint
test_endpoint() {
  local method=$1
  local endpoint=$2
  local description=$3
  local auth_required=${4:-false}
  local data=${5:-""}
  local expected_status=${6:-200}
  local use_api_prefix=${7:-true}
  local store_response=${8:-false}

  echo -e "\n${BLUE}Testing: $description${NC}"
  
  # Determine the full URL based on whether to use the API prefix
  local full_url
  if [ "$use_api_prefix" = true ]; then
    full_url="$API_URL$endpoint"
  else
    full_url="$BASE_URL$endpoint"
  fi
  
  echo "$method $full_url"

  # Build the curl command
  cmd="curl -s -X $method -w \"%{http_code}\" \"$full_url\""
  
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

  # Store the response if requested
  if [ "$store_response" = true ]; then
    CREATED_PRODUCT_RESPONSE="$response"
    # Try to extract the product ID using basic string manipulation
    # Looking for "id":"SOME_ID" pattern
    if [[ "$response" =~ \"id\":\"([^\"]+)\" ]]; then
      CREATED_PRODUCT_ID="${BASH_REMATCH[1]}"
      echo -e "${YELLOW}Extracted product ID: $CREATED_PRODUCT_ID${NC}"
    fi
  fi

  # Check if status code is as expected
  if [ "$status_code" -eq "$expected_status" ]; then
    echo -e "${GREEN}✓ Success (Status: $status_code)${NC}"
  else
    echo -e "${RED}✗ Failed (Status: $status_code)${NC}"
  fi
  
  # Output response without trying to prettify
  echo "Response: $response"
}

# Function to run all tests
run_tests() {
  echo -e "\n${BLUE}===== Starting API Tests =====${NC}"
  
  # Health check (outside of /api)
  test_endpoint "GET" "/health" "Health Check" false "" 200 false
  
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
  
  # Get a single product - we'll try the first one we can find
  test_endpoint "GET" "/products/1" "Get Product by ID" false "" 404
  
  # Submit a product (authenticated)
  test_product='{
    "title": "Test Product '$(date +%s)'",
    "short_desc": "A test product created via API test script",
    "long_desc": "This is a longer description of the test product",
    "logo_url": "https://example.com/logo.png",
    "markdown_content": "# Test Product\n\nThis is a test product description with markdown.",
    "is_verified": false,
    "analytics_list": ["<iframe src=\"https://example.com/analytics1\"></iframe>", "<iframe src=\"https://example.com/analytics2\"></iframe>"],
    "security_score": 0.75,
    "ux_score": 0.85,
    "decent_score": 0.65,
    "vibes_score": 0.90,
    "categories": [{"id": "1"}],
    "chains": [{"id": "1"}]
  }'
  
  # Store the response to extract the product ID
  test_endpoint "POST" "/products" "Submit Product" true "$test_product" 201 true true
  
  # Try to upvote the product we just created
  if [ -n "$CREATED_PRODUCT_ID" ]; then
    test_endpoint "POST" "/products/$CREATED_PRODUCT_ID/upvote" "Upvote Product (ID: $CREATED_PRODUCT_ID)" true "" 204
  else
    echo -e "${YELLOW}Warning: Could not extract product ID for upvote test${NC}"
    # Fallback to using a known product ID if available, or skip
    test_endpoint "POST" "/products/1" "Upvote Product (fallback)" true "" 404
  fi
  
  ## CATEGORY ENDPOINTS ##
  echo -e "\n${YELLOW}===== CATEGORY ENDPOINTS =====${NC}"
  
  # Get categories (public)
  test_endpoint "GET" "/categories" "Get Categories" false
  
  # Submit a category (authenticated) with a unique name
  timestamp=$(date +%s)
  test_category='{
    "name": "Test Category '"$timestamp"'",
    "description": "A test category created via API test script"
  }'
  
  test_endpoint "POST" "/categories" "Submit Category" true "$test_category" 201
  
  ## ADMIN ENDPOINTS ##
  echo -e "\n${YELLOW}===== ADMIN ENDPOINTS =====${NC}"
  
  # Get pending edits (admin only)
  test_endpoint "GET" "/admin/pending" "Get Pending Edits" true
  
  # Since we can't easily extract pending edit IDs without Python/jq,
  # we'll just inform the user
  echo -e "${YELLOW}Note: Approve/Reject pending edits test skipped (requires JSON parsing)${NC}"
  
  echo -e "\n${BLUE}===== API Tests Completed =====${NC}"
}

# Run all tests
run_tests 