# Tusker

One single place for all your integrations and backend tasks.

Tired of setting up the same OAuth flow, email provider, or SMS integration for every new project? Tusker is a SaaS platform that lets you configure an external integration once and access it via a single API — from any app, forever.

**Bottom line: Once you integrate with an external API, you never have to do it again.**

## Table of Contents

- [How it works](#how-it-works)
- [API](#api)
- [Deploying to DigitalOcean](#deploying-to-digitalocean)
- [Supported providers](#supported-providers)
- [Security](#security)
- [Running locally](#running-locally)

## How it works

1. Create a tenant and get an API key
2. Configure your provider credentials (e.g. Google OAuth client ID/secret)
3. Point your app at Tusker — Tusker handles the OAuth flow, token exchange, and secure storage
4. Fetch tokens via API whenever your app needs them

## API

**Health**
```
GET    /health                           Returns 200 {"status":"ok"} — used by Docker/load-balancer health checks
```

**OAuth**
```
POST   /tenants                          Provision a tenant, get API key (shown once)
POST   /oauth/:provider/config           Set OAuth provider credentials (client_id, client_secret)
GET    /oauth/:provider/authorize        Start OAuth flow — redirect your users here
GET    /oauth/:provider/callback         Provider redirects here (Tusker-owned, register this with your provider)
GET    /oauth/:provider/token?user_id=   Fetch a stored access token (auto-refreshed if expired)
DELETE /oauth/:provider/token?user_id=   Revoke a stored token

GET    /jobs/:id      Poll the status of a queued job (pending|running|completed|failed)

POST   /email/:provider/config           Set email provider credentials (provider-specific JSON)
POST   /email/:provider/send             Queue an email (async, returns 202 + job_id); add ?sync=true to send immediately
POST   /email/templates                  Upsert a named email template
GET    /email/templates                  List templates (custom + built-in defaults)
DELETE /email/templates/:name            Delete a custom template (reverts to built-in default if one exists)
POST   /email/:provider/send-template    Send email rendered from a named template + variables
```

**Email config bodies by provider:**

SMTP (`/email/smtp/config`):
```json
{ "host": "smtp.example.com", "port": 587, "username": "user@example.com", "password": "secret" }
```

SendGrid (`/email/sendgrid/config`):
```json
{ "api_key": "SG.xxxx" }
```

Send request (`/email/:provider/send`):
```json
{ "to": ["alice@example.com"], "from": "noreply@myapp.com", "subject": "Hello", "body": "Hi there!", "html": false }
```

Send-template request (`/email/:provider/send-template`):
```json
{ "template": "welcome", "to": ["alice@example.com"], "from": "noreply@myapp.com", "variables": { "ServiceName": "MyApp", "UserName": "Alice" } }
```

Built-in default templates (can be overridden per tenant): `welcome`, `login_alert`, `password_reset`, `magic_link`.

**SMS**
```
POST   /sms/:provider/config             Set provider credentials (account_sid, auth_token)
POST   /sms/:provider/send              Queue an SMS (async, returns 202 + job_id); add ?sync=true to send immediately
```

**Code execution (Judge0)**
```
POST   /code/:provider/config            Optional: override Judge0 URL and set auth token per tenant
POST   /code/:provider/execute           Submit code for execution (async, returns 202 + job_id); add ?sync=true to run immediately
GET    /code/executions/:job_id          Fetch stdout/stderr/status after async job completes
```

Execute request (`/code/judge0/execute`):
```json
{ "source_code": "print('hello')", "language_id": 71, "stdin": "" }
```

`language_id` follows [Judge0 language IDs](https://github.com/judge0/judge0/blob/master/docs/api/languages.md) (e.g. 71 = Python 3, 62 = Java, 54 = C++17).

All endpoints (except `/tenants` and `/oauth/:provider/callback`) require:
```
Authorization: Bearer <api_key>
```

Tusker will also monitor your API usage and send you regular updates on how the apps are using
the APIs. You can setup alerts for that.

## Go SDK

A first-party Go SDK lives in the `sdk/` directory as a standalone module (`github.com/gsarma/tusker/sdk`). It covers all Tusker API endpoints with no external dependencies.

**Install**

```bash
go get github.com/gsarma/tusker/sdk
```

**Quick start**

```go
import tusker "github.com/gsarma/tusker/sdk"

// Provision a tenant (one-time, no API key needed)
provisioner := tusker.NewProvisioner("https://api.tusker.io")
tenant, _ := provisioner.CreateTenant(ctx)
// Store tenant.APIKey securely — shown only once

// Authenticated client
client := tusker.New("https://api.tusker.io", tenant.APIKey)

// Configure SMTP and send an email (async)
client.Email.SetConfig(ctx, "smtp", tusker.SMTPConfig{Host: "smtp.example.com", Port: 587, Username: "u", Password: "p"})
resp, _ := client.Email.Send(ctx, "smtp", tusker.SendEmailRequest{
    To: []string{"alice@example.com"}, From: "noreply@myapp.com",
    Subject: "Hello", Body: "Hi there!",
}, nil)

// Poll job status
job, _ := client.Jobs.Get(ctx, resp.JobID)
fmt.Println(job.Status) // pending | running | completed | failed
```

See `sdk/example_test.go` for OAuth, SMS, and template examples.

## Deploying to DigitalOcean

The `infra/` directory contains Terraform config and shell scripts to provision and deploy Tusker on a DigitalOcean droplet.

**Prerequisites**

- [Terraform](https://developer.hashicorp.com/terraform/install) installed
- A DigitalOcean account with an [API token](https://cloud.digitalocean.com/account/api/tokens)
- An SSH key uploaded to your DigitalOcean account (`doctl compute ssh-key list`)

**Provision the droplet**

```bash
cp infra/terraform.tfvars.example infra/terraform.tfvars
# edit terraform.tfvars with your token and SSH key name

cd infra
terraform init
terraform apply
```

`terraform apply` is idempotent — re-running it will not create a second droplet. On first boot, the droplet automatically installs PostgreSQL, generates a `ROOT_ENCRYPTION_KEY`, and registers the Tusker systemd service.

**Deploy the app**

```bash
# from the repo root — reads the droplet IP from terraform output automatically
./infra/scripts/deploy.sh

# or pass the IP explicitly
DROPLET_IP=x.x.x.x ./infra/scripts/deploy.sh
```

The deploy script cross-compiles the Go binary for `linux/amd64`, copies it to the droplet, runs database migrations, and restarts the service.

**Environment**

Runtime config lives in `/etc/tusker/tusker.env` on the droplet:

| Variable | Description |
|---|---|
| `DATABASE_URL` | Postgres connection string (set to local DB by default) |
| `ROOT_ENCRYPTION_KEY` | Auto-generated 32-byte hex AES key |
| `TUSKER_BASE_URL` | Public base URL — update once you point a domain at the droplet |
| `PORT` | HTTP port (default `8080`) |
## Supported providers

**OAuth**
- Google
- Slack

**Email**
- SMTP
- SendGrid
**SMS**
- Twilio

**Code execution**
- Judge0 CE (self-hosted via Docker)

## Security

- Per-tenant envelope encryption (AES-256-GCM): client secrets and tokens are encrypted at rest
- API keys are never stored — only a SHA-256 hash is kept
- Tenant credentials are fully isolated

## Running with Docker

The quickest way to get the full stack (Postgres + migrations + server) running locally:

```bash
# Set a root encryption key (required)
echo "ROOT_ENCRYPTION_KEY=$(openssl rand -hex 32)" > .env

docker compose up --build
```

`docker compose up` runs the following services in order:
1. **postgres** — Postgres 16 (Tusker database)
2. **migrate** — applies all DB migrations
3. **redis** — Redis 7 (Judge0 job queue)
4. **judge0-db** — Postgres 16 (Judge0's dedicated database)
5. **judge0-server** — Judge0 CE API on port 2358
6. **judge0-workers** — Judge0 worker processes
7. **api** — the Tusker server (API + background worker) on port 8080

Judge0 requires `privileged: true` for its sandbox. Configuration is in `judge0.conf` (change passwords before deploying to production).

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

For local end-to-end testing of the OAuth flow, run the test client in a second terminal:
```bash
go run ./cmd/testclient   # listens on :9999, catches the post-auth redirect
```
