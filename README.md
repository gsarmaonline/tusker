# Tusker

One single place for all your integrations and backend tasks.

Tired of thinking whether to setup an AWS EC2 VM or setup a PaaS worker to perform a task?
Tired of setting up the same thing again and again? Like email provider, SMS provider, setting
up Google Oauth?

Let's fix it once and for all.

Tusker is the one stop setup machine(s) where you can setup all your workflows so that every
task is just an API away.

Need a cron job? Fire an API.
Need to send an email? Fire an API.

You don't have to do all the API integration and the token management anymore.

Worried about data leaks across different apps? Don't be. Tusker allows you to categorise
your secrets based on apps as well. It also allows you to share the same secret but operate
on entire different constraints/templates based on the app.

Bottom line: Once you integrate with an external API, you never have to do it again.

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
