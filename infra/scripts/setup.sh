#!/bin/bash
# Cloud-init script — runs once on first boot as root.
# Installs PostgreSQL, golang-migrate, creates the tusker user,
# directories, env file template, and systemd service.
set -euo pipefail

APP_USER="tusker"
APP_DIR="/opt/tusker"
ENV_FILE="/etc/tusker/tusker.env"
SERVICE_NAME="tusker"
MIGRATE_VERSION="v4.17.0"
PG_DB="tusker"
PG_USER="tusker"
PG_PASSWORD="$(openssl rand -hex 16)"

# ── System packages ──────────────────────────────────────────────────────────
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl openssl postgresql postgresql-client

# ── PostgreSQL: create DB and user ───────────────────────────────────────────
systemctl enable --now postgresql

sudo -u postgres psql -c "CREATE USER ${PG_USER} WITH PASSWORD '${PG_PASSWORD}';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE ${PG_DB} OWNER ${PG_USER};" 2>/dev/null || true
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${PG_DB} TO ${PG_USER};" 2>/dev/null || true
# pgcrypto extension (required by schema)
sudo -u postgres psql -d "${PG_DB}" -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;" 2>/dev/null || true

DATABASE_URL="postgres://${PG_USER}:${PG_PASSWORD}@localhost:5432/${PG_DB}?sslmode=disable"

# ── golang-migrate ───────────────────────────────────────────────────────────
curl -fsSL \
  "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
  | tar -xz -C /usr/local/bin migrate
chmod +x /usr/local/bin/migrate

# ── App user and directories ─────────────────────────────────────────────────
useradd --system --shell /bin/false --create-home --home-dir "${APP_DIR}" "${APP_USER}" 2>/dev/null || true
mkdir -p "${APP_DIR}/migrations" /etc/tusker
chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"

# ── Environment file ──────────────────────────────────────────────────────────
# Values here are defaults; replace before running the first deploy.
cat > "${ENV_FILE}" <<EOF
DATABASE_URL=${DATABASE_URL}
ROOT_ENCRYPTION_KEY=$(openssl rand -hex 32)
TUSKER_BASE_URL=http://$(curl -sf http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address 2>/dev/null || echo "127.0.0.1"):8080
PORT=8080
EOF
chmod 600 "${ENV_FILE}"
chown root:root "${ENV_FILE}"

# ── systemd service ───────────────────────────────────────────────────────────
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Tusker API Server
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_USER}
EnvironmentFile=${ENV_FILE}
ExecStart=/usr/local/bin/tusker
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=tusker

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"

echo "Setup complete. Edit ${ENV_FILE} then run deploy.sh to ship the binary."
