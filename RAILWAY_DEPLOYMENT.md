# Deploying to Railway

This document explains how to deploy the EthAppList backend on Railway.

## Prerequisites

1. Create a [Railway account](https://railway.app/)
2. Install the [Railway CLI](https://docs.railway.app/develop/cli)
3. Login to Railway: `railway login`

## Deployment Steps

### 1. Setup PostgreSQL on Railway

1. Create a new project in Railway
2. Add a PostgreSQL database to your project:
   ```
   railway add
   ```
   Select PostgreSQL when prompted.

3. Note the automatically generated `DATABASE_URL` which Railway will provide as an environment variable to your service.

### 2. Deploy the Backend

1. Link your local project to your Railway project:
   ```
   railway link
   ```

2. Add required environment variables:
   ```
   railway variables set JWT_SECRET=your_jwt_secret ADMIN_WALLET_ADDRESS=your_admin_wallet_address
   ```

3. Deploy the application:
   ```
   railway up
   ```

4. Alternatively, you can connect your GitHub repository for automatic deployments:
   - Go to your Railway project dashboard
   - Click "Add Service" → "GitHub Repo"
   - Select your repository
   - Railway will automatically build and deploy your application

## Verification

1. Check the deployment status:
   ```
   railway status
   ```

2. Open your service URL to verify it's running:
   ```
   railway open
   ```

3. Test the API by visiting: `https://<your-railway-url>/health`

## Maintenance

- View logs:
  ```
  railway logs
  ```

- Update environment variables:
  ```
  railway variables set KEY=VALUE
  ```

- Redeploy after changes:
  ```
  railway up
  ```

## Important Notes

1. Railway provides the `DATABASE_URL` environment variable automatically when you connect a PostgreSQL service to your deployment.

2. To use Railway's built-in PostgreSQL instead of Supabase, you don't need to set individual `DB_*` environment variables - the application will automatically parse the `DATABASE_URL`.

3. For database migrations, the application will automatically run migrations on startup if needed.

4. The deployment will use the Docker image defined in the Dockerfile for consistent builds. 