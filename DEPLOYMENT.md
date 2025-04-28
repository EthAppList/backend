# Deploying the Crypto Products Backend

This guide provides two methods for deploying the Crypto Products Backend:

1. Docker-based deployment (recommended for ease of setup)
2. Traditional Arch Linux deployment

## Docker-based Deployment (Recommended)

Docker provides an easy way to deploy the application without having to manually install dependencies or configure services.

### Prerequisites

- Server with Docker and Docker Compose installed
- Domain name pointing to your server
- Git

### 1. Install Docker and Docker Compose

If you don't have Docker installed, you can install it with the following commands:

For Arch Linux:
```bash
sudo pacman -S docker docker-compose
sudo systemctl start docker
sudo systemctl enable docker
```

For Ubuntu/Debian:
```bash
sudo apt update
sudo apt install docker.io docker-compose
sudo systemctl start docker
sudo systemctl enable docker
```

### 2. Clone the Repository

```bash
mkdir -p /opt/apps
cd /opt/apps
git clone https://github.com/yourusername/crypto-products-backend.git
cd crypto-products-backend
```

### 3. Configure Environment Variables

Edit the `docker-compose.yml` file to update the environment variables:

```bash
nano docker-compose.yml
```

Update the following variables in both the `app` and `postgres` services:
- `DB_PASSWORD` / `POSTGRES_PASSWORD`: Set a strong password
- `JWT_SECRET`: Generate a secure random string (e.g., `openssl rand -hex 32`)
- `ADMIN_WALLET_ADDRESS`: Set to your admin wallet address

### 4. Start the Application

```bash
docker-compose up -d
```

This command will:
1. Build the application Docker image
2. Start PostgreSQL database
3. Initialize the database schema
4. Start the application

### 5. Verify Deployment

Check if the containers are running:

```bash
docker-compose ps
```

View application logs:

```bash
docker-compose logs -f app
```

### 6. Set Up Nginx as Reverse Proxy with SSL

Install Nginx and Certbot:

```bash
sudo pacman -S nginx certbot certbot-nginx   # For Arch Linux
# OR
sudo apt install nginx certbot python3-certbot-nginx   # For Ubuntu/Debian
```

Create an Nginx configuration file:

```bash
sudo nano /etc/nginx/sites-available/crypto-backend
```

Add the following configuration:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

Create a symbolic link to enable the site:

```bash
sudo mkdir -p /etc/nginx/sites-enabled
sudo ln -s /etc/nginx/sites-available/crypto-backend /etc/nginx/sites-enabled/
```

Test the Nginx configuration:

```bash
sudo nginx -t
```

Start Nginx:

```bash
sudo systemctl start nginx
sudo systemctl enable nginx
```

Set up SSL with Certbot:

```bash
sudo certbot --nginx -d your-domain.com
```

Follow the prompts to complete the SSL setup.

### 7. Updating the Application

To update the application:

```bash
cd /opt/apps/crypto-products-backend
git pull
docker-compose down
docker-compose up -d --build
```

### 8. Backing Up the Database

To create a backup of the PostgreSQL database:

```bash
docker-compose exec postgres pg_dump -U crypto_admin -d crypto_products > backup.sql
```

To restore from a backup:

```bash
cat backup.sql | docker-compose exec -T postgres psql -U crypto_admin -d crypto_products
```

## Traditional Arch Linux Deployment

If you prefer to deploy without Docker, follow these steps for a traditional deployment on Arch Linux.

### 1. Server Setup

#### 1.1 Update System Packages

First, connect to your server via SSH and ensure your system is up to date:

```bash
ssh username@your-server.com
sudo pacman -Syu
```

#### 1.2 Install Required Software

Install the necessary packages:

```bash
sudo pacman -S postgresql nginx go git certbot certbot-nginx
```

### 2. PostgreSQL Database Setup

#### 2.1 Initialize PostgreSQL

```bash
sudo -u postgres initdb -D /var/lib/postgres/data
```

#### 2.2 Start and Enable PostgreSQL Service

```bash
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

#### 2.3 Create Database and User

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

### 3. Application Setup

#### 3.1 Clone the Repository

```bash
mkdir -p /opt/apps
cd /opt/apps
git clone https://github.com/yourusername/crypto-products-backend.git
cd crypto-products-backend
```

#### 3.2 Configure Environment Variables

Create an `.env` file:

```bash
cp .env.example .env
nano .env
```

Update the values in the `.env` file:

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=crypto_admin
DB_PASSWORD=your_secure_password
DB_NAME=crypto_products
JWT_SECRET=generate_a_secure_random_string
ADMIN_WALLET_ADDRESS=your_admin_wallet_address
PORT=8080
ENVIRONMENT=production
```

> Generate a secure JWT secret with: `openssl rand -hex 32`

#### 3.3 Setup Database Schema

Run the database setup:

```bash
go run scripts/setup.go
```

#### 3.4 Build the Application

```bash
go build -o app cmd/server/main.go
```

### 4. Setting Up systemd Service

Create a systemd service file:

```bash
sudo nano /etc/systemd/system/crypto-backend.service
```

Add the following content:

```ini
[Unit]
Description=Crypto Products Backend
After=network.target postgresql.service

[Service]
User=your_user
WorkingDirectory=/opt/apps/crypto-products-backend
ExecStart=/opt/apps/crypto-products-backend/app
Restart=on-failure
RestartSec=5
Environment=ENVIRONMENT=production

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable crypto-backend
sudo systemctl start crypto-backend
```

Check that it's running:

```bash
sudo systemctl status crypto-backend
```

### 5. Nginx Reverse Proxy Setup

#### 5.1 Create Nginx Configuration

```bash
sudo nano /etc/nginx/sites-available/crypto-backend
```

Add the following configuration:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

Create a symbolic link to enable the site:

```bash
sudo mkdir -p /etc/nginx/sites-enabled
sudo ln -s /etc/nginx/sites-available/crypto-backend /etc/nginx/sites-enabled/
```

Test the Nginx configuration:

```bash
sudo nginx -t
```

#### 5.2 Start Nginx

```bash
sudo systemctl start nginx
sudo systemctl enable nginx
```

#### 5.3 Configure SSL with Certbot

```bash
sudo certbot --nginx -d your-domain.com
```

Follow the prompts to complete the SSL setup.

## Maintenance and Troubleshooting

### View Application Logs

For Docker deployment:
```bash
docker-compose logs -f app
```

For traditional deployment:
```bash
sudo journalctl -u crypto-backend -f
```

### Check Service Status

For Docker deployment:
```bash
docker-compose ps
```

For traditional deployment:
```bash
sudo systemctl status crypto-backend
```

### Test API Endpoint

```bash
curl -i http://localhost:8080/health
```

## Security Considerations

1. Setup a firewall:

```bash
sudo pacman -S ufw
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow http
sudo ufw allow https
sudo ufw enable
```

2. Configure automatic updates.

3. Regularly check and install security updates.

## Scaling Considerations

For higher traffic:

1. **Database Optimization**: Review PostgreSQL settings and adjust for your server specs.

2. **Load Balancing**: For Docker, consider using Docker Swarm or Kubernetes for orchestration.

3. **Monitoring**: Add monitoring tools like Prometheus and Grafana.

4. **Caching**: Implement Redis for caching frequently accessed data. 