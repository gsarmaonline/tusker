#!/bin/bash
# Deploy the Tusker binary to an existing droplet.
#
# Usage:
#   ./infra/scripts/deploy.sh [droplet-ip]
#
# Or set DROPLET_IP in the environment. If neither is given, the script
# reads the IP from `terraform output` inside the infra/ directory.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DROPLET_IP="${1:-${DROPLET_IP:-}}"
SSH_USER="${SSH_USER:-root}"
REMOTE_BIN="/usr/local/bin/tusker"
REMOTE_MIGRATIONS="/opt/tusker/migrations"
ENV_FILE="/etc/tusker/tusker.env"

# ── Resolve droplet IP ────────────────────────────────────────────────────────
if [[ -z "${DROPLET_IP}" ]]; then
  if command -v terraform &>/dev/null && [[ -f "${REPO_ROOT}/infra/main.tf" ]]; then
    echo "No DROPLET_IP given — reading from terraform output..."
    DROPLET_IP="$(terraform -chdir="${REPO_ROOT}/infra" output -raw droplet_ip 2>/dev/null)" || true
  fi
fi

if [[ -z "${DROPLET_IP}" ]]; then
  echo "Error: droplet IP not found. Pass it as the first argument, set DROPLET_IP,"
  echo "       or run 'terraform apply' inside infra/ first." >&2
  exit 1
fi

SSH_TARGET="${SSH_USER}@${DROPLET_IP}"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10"

echo "Deploying to ${SSH_TARGET}..."

# ── Build Linux amd64 binary ─────────────────────────────────────────────────
mkdir -p "${REPO_ROOT}/bin"
echo "Building binary (linux/amd64)..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" \
  -o "${REPO_ROOT}/bin/tusker" \
  "${REPO_ROOT}/cmd/server"

# ── Wait for SSH to become available (useful right after terraform apply) ─────
echo "Waiting for SSH on ${DROPLET_IP}..."
for i in $(seq 1 30); do
  ssh ${SSH_OPTS} -o BatchMode=yes "${SSH_TARGET}" true 2>/dev/null && break
  echo "  attempt ${i}/30 — retrying in 5s..."
  sleep 5
done

# ── Copy binary ───────────────────────────────────────────────────────────────
echo "Copying binary..."
scp ${SSH_OPTS} "${REPO_ROOT}/bin/tusker" "${SSH_TARGET}:${REMOTE_BIN}"
ssh ${SSH_OPTS} "${SSH_TARGET}" "chmod +x ${REMOTE_BIN}"

# ── Copy migration files ──────────────────────────────────────────────────────
echo "Copying migrations..."
ssh ${SSH_OPTS} "${SSH_TARGET}" "mkdir -p ${REMOTE_MIGRATIONS}"
scp ${SSH_OPTS} -r "${REPO_ROOT}/db/migrations/." "${SSH_TARGET}:${REMOTE_MIGRATIONS}/"

# ── Run migrations ────────────────────────────────────────────────────────────
echo "Running migrations..."
ssh ${SSH_OPTS} "${SSH_TARGET}" "
  set -a; source ${ENV_FILE}; set +a
  migrate -path ${REMOTE_MIGRATIONS} -database \"\${DATABASE_URL}\" up
"

# ── Restart service ───────────────────────────────────────────────────────────
echo "Restarting tusker service..."
ssh ${SSH_OPTS} "${SSH_TARGET}" "systemctl restart tusker"
sleep 2
ssh ${SSH_OPTS} "${SSH_TARGET}" "systemctl is-active tusker"

echo ""
echo "Done. Tusker is running at http://${DROPLET_IP}:8080"
