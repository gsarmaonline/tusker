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

**OAuth**
```
POST   /tenants                          Provision a tenant, get API key (shown once)
POST   /oauth/:provider/config           Set provider credentials (client_id, client_secret)
GET    /oauth/:provider/authorize        Start OAuth flow — redirect your users here
GET    /oauth/:provider/callback         Provider redirects here (Tusker-owned, register this with your provider)
GET    /oauth/:provider/token?user_id=   Fetch a stored access token (auto-refreshed if expired)
DELETE /oauth/:provider/token?user_id=   Revoke a stored token
```

**SMS**
```
POST   /sms/:provider/config             Set provider credentials (account_sid, auth_token)
POST   /sms/:provider/send              Send an SMS message (from, to, body)
```

All endpoints (except `/tenants` and `/oauth/:provider/callback`) require:
```
Authorization: Bearer <api_key>
```

Tusker will also monitor your API usage and send you regular updates on how the apps are using
the APIs. You can setup alerts for that.

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

**SMS**
- Twilio

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

For local end-to-end testing of the OAuth flow, run the test client in a second terminal:
```bash
go run ./cmd/testclient   # listens on :9999, catches the post-auth redirect
```
