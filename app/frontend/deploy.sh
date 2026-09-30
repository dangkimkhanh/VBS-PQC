#!/bin/bash
set -e

### --- Load NVM & Node ---
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
nvm use 24

# ================= CẤU HÌNH =================
APP_NAME="my-nextjs-app"
PROD_DIR="/var/www/nextjs-fe"
# ===========================================

echo "🚀 Bắt đầu deploy $APP_NAME..."

# 1. Cài đặt dependencies (Nếu truyền tham số "install")
if [ "$1" = "install" ]; then
  echo "📦 [Install Mode] Cleaning node_modules & .next & running npm ci..."
  rm -rf node_modules .next
  npm ci
else
  echo "⏭ Skipping npm ci (dùng 'deploy.sh install' để cài lại deps)"
fi

# 2. Build Next.js
echo "🏗 Building Next.js (Standalone mode)..."
npm run build

# 3. Copy artifact sang thư mục Production
echo "📤 Copying build to $PROD_DIR..."

# Dọn sạch thư mục production (giữ lại .env nếu muốn)
rm -rf "$PROD_DIR"/*
rm -rf "$PROD_DIR"/.* 2>/dev/null || true
mkdir -p "$PROD_DIR"

# ✅ Bước A: Copy TOÀN BỘ standalone (server.js, node_modules, .next/server, manifests...)
cp -a .next/standalone/. "$PROD_DIR/"

# ✅ Bước B: Copy .next/static TỪ ROOT .next/ (vì nó KHÔNG nằm trong standalone)
mkdir -p "$PROD_DIR/.next/static"
cp -a .next/static/. "$PROD_DIR/.next/static/"

# ✅ Bước C: Copy public assets (chỉ copy nội dung)
if [ -d "public" ]; then
  mkdir -p "$PROD_DIR/public"
  cp -a public/. "$PROD_DIR/public/" 2>/dev/null || true
fi

# ✅ Bước D: Copy .env file (nếu tồn tại)
if [ -f ".env" ]; then
  cp .env "$PROD_DIR/.env"
fi

# 4. Tạo/Cập nhật file ecosystem.config.js
echo "⚙️ Generating ecosystem.config.js..."
cat > "$PROD_DIR/ecosystem.config.js" << EOF
module.exports = {
  apps: [{
    name: '$APP_NAME',
    script: './server.js',
    cwd: '$PROD_DIR',
    instances: 1,
    exec_mode: 'fork',
    env: {
      NODE_ENV: 'production',
      PORT: 3000,
      HOSTNAME: '127.0.0.1'
    }
  }]
};
EOF

# 5. Khởi động hoặc Restart PM2
echo "🔄 Managing PM2 process..."
if pm2 describe "$APP_NAME" >/dev/null 2>&1; then
  echo "♻️ Restarting $APP_NAME..."
  pm2 restart "$APP_NAME"
else
  echo "🚀 Starting new process: $APP_NAME..."
  cd "$PROD_DIR"
  pm2 start ecosystem.config.js
fi

pm2 save

echo ""
echo "✅ Deploy thành công!"
echo "👉 Kiểm tra log: pm2 logs $APP_NAME"
echo "👉 Xem trạng thái: pm2 status"