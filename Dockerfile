FROM golang:1.22-alpine AS builder

# Install git and SSL certificates for downloading dependencies
RUN apk add --no-cache git ca-certificates tzdata && update-ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app cmd/server/main.go

# Create final lightweight image
FROM alpine:latest  

# Add runtime dependencies and PostgreSQL client tools
RUN apk --no-cache add ca-certificates postgresql-client netcat-openbsd

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/app .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/scripts ./scripts
COPY --from=builder /app/go.mod ./go.mod
COPY --from=builder /app/go.sum ./go.sum

# Make the scripts executable
RUN chmod +x ./scripts/docker-init.sh ./scripts/db/setup_postgres.sh

# Expose the application port
EXPOSE 8080

# Run the initialization script
CMD ["./scripts/docker-init.sh"] 