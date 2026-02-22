# Tusker

One single place for all your integrations and backend tasks.

Tired of setting up the same OAuth flow, email provider, or SMS integration for every new project? Tusker is a SaaS platform that lets you configure an external integration once and access it via a single API — from any app, forever.

**Bottom line: Once you integrate with an external API, you never have to do it again.**

## How it works

1. Create a tenant and get an API key
2. Configure your provider credentials (e.g. Google OAuth client ID/secret)
3. Point your app at Tusker — Tusker handles the OAuth flow, token exchange, and secure storage
4. Fetch tokens via API whenever your app needs them

## API

```
POST   /tenants                          Provision a tenant, get API key (shown once)
POST   /oauth/:provider/config           Set provider credentials (client_id, client_secret)
GET    /oauth/:provider/authorize        Start OAuth flow — redirect your users here
GET    /oauth/:provider/callback         Provider redirects here (Tusker-owned, register this with your provider)
GET    /oauth/:provider/token?user_id=   Fetch a stored access token
DELETE /oauth/:provider/token?user_id=   Revoke a stored token
```

All endpoints (except `/tenants` and `/oauth/:provider/callback`) require:
```
Authorization: Bearer <api_key>
```

## Supported providers

- Google

## Security

- Per-tenant envelope encryption (AES-256-GCM): client secrets and tokens are encrypted at rest
- API keys are never stored — only a SHA-256 hash is kept
- Tenant credentials are fully isolated

## Running locally

```bash
export DATABASE_URL="postgres://..."
export ROOT_ENCRYPTION_KEY="$(openssl rand -hex 32)"
export TUSKER_BASE_URL="http://localhost:8080"

go run ./cmd/server
```

Run migrations before first start:
```bash
migrate -path db/migrations -database "$DATABASE_URL" up
```
