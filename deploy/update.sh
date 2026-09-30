#!/usr/bin/env bash
# Pull the latest code, rebuild, and restart VBS PQC. Run on the VPS after
# install-vps.sh and the first-time configuration (see deploy/README.md):
#
#   sudo bash /opt/vbs-pqc/deploy/update.sh
set -euo pipefail

APP_USER=vbs
APP_DIR=/opt/vbs-pqc
as_app() { sudo -u "$APP_USER" -H bash -c "cd '$APP_DIR' && $1"; }

echo "==> Cập nhật mã nguồn"
as_app "git pull --ff-only"

echo "==> MongoDB và MinIO"
(cd "$APP_DIR/app/backend" && docker compose --env-file .env up -d)

echo "==> Build backend"
as_app "cd app/backend && go build -trimpath -o bin/server ./cmd/server"

echo "==> Build frontend"
as_app "cd app/frontend && npm ci && npm run build \
  && rm -rf .next/standalone/.next/static .next/standalone/public \
  && cp -r .next/static .next/standalone/.next/static \
  && if [ -d public ]; then cp -r public .next/standalone/public; fi"

echo "==> Khởi động lại dịch vụ"
systemctl restart vbs-backend vbs-frontend
sleep 3
systemctl --no-pager --lines=5 status vbs-backend vbs-frontend
