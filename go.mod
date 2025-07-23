module github.com/wesjorgensen/EthAppList/backend

go 1.22

toolchain go1.23.6

require (
	github.com/ethereum/go-ethereum v1.13.5
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/google/uuid v1.3.1
	github.com/gorilla/mux v1.8.0
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	github.com/redis/go-redis/v9 v9.11.0
	github.com/rs/cors v1.10.1
)

require (
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/holiman/uint256 v1.2.3 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
)
