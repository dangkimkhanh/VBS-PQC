#!/usr/bin/env bash
# One-time setup of a fresh Ubuntu 22.04 / 24.04 (x86_64) VPS for VBS PQC.
# Installs Docker (MongoDB, MinIO), Go, Node.js, Google Chrome (PDF rendering),
# Vietnamese-capable fonts, nginx, certbot and the firewall.
#
#   sudo bash deploy/install-vps.sh
set -euo pipefail

GO_VERSION=1.24.4
NODE_MAJOR=22
APP_USER=vbs
APP_DIR=/opt/vbs-pqc

if [ "$(id -u)" -ne 0 ]; then
  echo "Chạy bằng quyền root: sudo bash $0" >&2
  exit 1
fi
if [ "$(dpkg --print-architecture)" != "amd64" ]; then
  echo "Script này dành cho VPS x86_64 (amd64)." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl gnupg git ufw nginx certbot python3-certbot-nginx \
  fontconfig fonts-liberation fonts-dejavu-core fonts-noto-core

# --- Docker Engine + Compose plugin (MongoDB, MinIO) ---
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker

# --- Go (backend build) ---
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
rm /tmp/go.tgz
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

# --- Node.js (frontend build and runtime) ---
curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
apt-get install -y nodejs

# --- Google Chrome: the backend prints diploma PDFs with headless Chrome ---
curl -fsSL https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb -o /tmp/chrome.deb
apt-get install -y /tmp/chrome.deb
rm /tmp/chrome.deb

# --- Service account that runs the backend and frontend ---
id -u "$APP_USER" >/dev/null 2>&1 || useradd --system --create-home --shell /bin/bash "$APP_USER"
mkdir -p "$APP_DIR"
chown "$APP_USER:$APP_USER" "$APP_DIR"

# --- Firewall: only SSH and HTTP(S) are public ---
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw --force enable

echo
echo "Đã cài xong: $(go version), node $(node -v), $(google-chrome --version), $(docker compose version)"
