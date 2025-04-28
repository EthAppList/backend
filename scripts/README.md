# Scripts Directory

This directory contains utility scripts for setting up and managing the EthAppList backend.

## Structure

- `cmd/` - Contains command-line tools with each tool in its own package
  - `dockersetup/` - Script for setting up the database in Docker environment
  - `setup/` - General setup script with Docker/non-Docker mode
  - `setupdb/` - Script for setting up Supabase database
- `pkg/` - Shared packages used by the command-line tools
  - `utils/` - Common utility functions

## Usage

### Docker Setup

To set up the database in a Docker environment:

```bash
go run scripts/cmd/dockersetup/main.go
```

### Standard Setup

To run the standard setup:

```bash
go run scripts/cmd/setup/main.go
```

### Supabase Setup

To set up your Supabase database:

```bash
go run scripts/cmd/setupdb/main.go
```

## Notes

- Each command tool is in its own package with a `main.go` file
- Common functions are in the `pkg/utils` package to avoid duplication
- For Docker deployments, the Docker initialization scripts should be used 