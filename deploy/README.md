# Triển khai VBS PQC lên VPS

Dành cho VPS Ubuntu 22.04/24.04 x86_64, một tên miền trỏ về IP của VPS.
Mọi thứ chạy trên cùng một máy: nginx (80/443) chuyển tới Next.js (127.0.0.1:3000) và API Go
(127.0.0.1:8080); MongoDB và MinIO chạy trong Docker, chỉ nghe trên 127.0.0.1.
Mạng Hyperledger Fabric cũng chạy trên VPS này, dựng và quản lý bằng ChainLaunch; backend kết nối
peer của KMAUniverMSP qua Fabric Gateway tại `127.0.0.1:7059` (channel `ediploma-channel`,
chaincode `ediploma-chaincode`).

Hệ thống đang chạy tại `https://54.79.163.176.nip.io` (tên miền nip.io trỏ về IP của VPS).

## 1. Cài dịch vụ (một lần)

```bash
sudo apt-get update && sudo apt-get install -y git
sudo git clone https://github.com/dangkimkhanh/VBS-PQC.git /opt/vbs-pqc
sudo bash /opt/vbs-pqc/deploy/install-vps.sh
sudo chown -R vbs:vbs /opt/vbs-pqc
```

## 2. Cấu hình (một lần)

```bash
cd /opt/vbs-pqc
# Backend: điền mọi giá trị <...>
sudo -u vbs cp deploy/backend.env.example app/backend/.env
sudo -u vbs nano app/backend/.env
# Frontend
sudo -u vbs cp deploy/frontend.env.example app/frontend/.env.production
sudo -u vbs nano app/frontend/.env.production
```

Chép chứng chỉ Fabric từ máy của bạn (không có trong git), chạy trên máy của bạn:

```bash
scp -r app/backend/secrets root@<IP-VPS>:/opt/vbs-pqc/app/backend/
```

rồi trên VPS:

```bash
sudo chown -R vbs:vbs /opt/vbs-pqc/app/backend/secrets
sudo chmod 600 /opt/vbs-pqc/app/backend/secrets/fabric/*
```

## 3. Dịch vụ hệ thống và nginx (một lần)

```bash
sudo cp /opt/vbs-pqc/deploy/systemd/vbs-*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable vbs-backend vbs-frontend

sudo cp /opt/vbs-pqc/deploy/nginx/vbs-pqc.conf /etc/nginx/sites-available/vbs-pqc
sudo sed -i 's/example.com/<ten-mien-cua-ban>/' /etc/nginx/sites-available/vbs-pqc
sudo ln -sf /etc/nginx/sites-available/vbs-pqc /etc/nginx/sites-enabled/vbs-pqc
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d <ten-mien-cua-ban>
```

## 4. Chạy và cập nhật

Lần đầu và mỗi lần có code mới trên GitHub:

```bash
sudo bash /opt/vbs-pqc/deploy/update.sh
```

Script kéo code mới, bật MongoDB/MinIO, build backend và frontend rồi khởi động lại dịch vụ.

## Kiểm tra và xem log

```bash
systemctl status vbs-backend vbs-frontend
journalctl -u vbs-backend -f        # log backend: Fabric, gửi email, tạo PDF
journalctl -u vbs-frontend -f
docker compose -f /opt/vbs-pqc/app/backend/docker-compose.yml ps
```

## Lưu ý

- `APP_URL` (backend) và `NEXT_PUBLIC_API_URL` (frontend) là `https://<tên miền>`; đổi
  `NEXT_PUBLIC_API_URL` thì phải build lại (chạy lại `update.sh`).
- Chuyển dữ liệu cũ sang VPS thì phải giữ nguyên `PQC_MASTER_KEY` cũ, nếu không mọi khóa ký
  ML-DSA đã tạo sẽ không giải mã được.
- Tài khoản quản trị hệ thống chỉ được tạo ở lần chạy đầu tiên từ `ADMIN_EMAIL`/`ADMIN_PASSWORD`.
- Repo để private thì `git pull` cần token: `sudo -u vbs git -C /opt/vbs-pqc remote set-url origin https://<token>@github.com/dangkimkhanh/VBS-PQC.git`.
