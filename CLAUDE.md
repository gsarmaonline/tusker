# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Tusker** is a Go-based SaaS platform that provides a single unified API for common backend integrations (OAuth, email, SMS, workers, etc.), so developers configure a provider once and call Tusker instead of wiring up each provider directly.

## Commands

```bash
go build ./...                          # Build all packages
go run ./cmd/server                     # Run the server
go test ./...                           # Run all tests
go test ./... -run TestName             # Run a single test by name
go vet ./...                            # Static analysis
sqlc generate                           # Regenerate store from db/queries + db/migrations
```

## Environment Variables

| Variable | Description |
|---|---|
| `DATABASE_URL` | Postgres connection string |
| `ROOT_ENCRYPTION_KEY` | 32-byte hex-encoded AES root key (64 hex chars) |
| `TUSKER_BASE_URL` | Public base URL (e.g. `https://tusker.io`) — used to build OAuth callback URLs |
| `PORT` | HTTP port (default `8080`) |

Generate a root key: `openssl rand -hex 32`

## Architecture

### Directory Structure

```
cmd/server/         — main.go, wires dependencies and starts Gin
internal/
  api/              — Gin route registration and HTTP handlers
  crypto/           — AES-256-GCM envelope encryption
  tenant/           — tenant service, API key hashing, Gin auth middleware
  oauth/            — Provider interface, Google implementation, state encoding
  store/            — sqlc-generated DB layer (do not edit manually)
db/
  migrations/       — golang-migrate SQL migrations
  queries/          — sqlc SQL query definitions
sqlc.yaml           — sqlc code generation config
```

### Key Design Decisions

**Envelope encryption:** Each tenant has a data key (stored encrypted in `tenants.encrypted_data_key`) encrypted with a single root key. Secrets (OAuth client secrets, access/refresh tokens) are encrypted with the tenant's data key using AES-256-GCM. This isolates tenants and makes key rotation straightforward.

**Multi-tenancy:** All sensitive DB rows are scoped by `tenant_id`. Tenants authenticate via `Authorization: Bearer <api_key>` — only a SHA-256 hash of the API key is stored.

**OAuth flow:** Tusker owns a single callback URL per provider (`/oauth/:provider/callback`). Tenant identity and the customer's redirect URI are encoded in the OAuth `state` parameter (base64 JSON + CSRF nonce). On callback, Tusker decodes state, exchanges the code, encrypts and stores the token, then redirects to the customer's app.

**Adding a new OAuth provider:** Implement the `oauth.Provider` interface (`internal/oauth/provider.go`) and add a case in `handler.go:buildProvider`.

**DB queries:** Write SQL in `db/queries/`, run `sqlc generate` to regenerate `internal/store/`. Never edit generated files in `internal/store/`.
