# VBS PQC - Hệ thống quản lý văn bằng số sử dụng ký số hậu lượng tử

VBS PQC là hệ thống cấp phát, quản lý và xác minh văn bằng số cho các cơ sở đào tạo.
Mỗi văn bằng được ký bằng thuật toán chữ ký hậu lượng tử ML-DSA (FIPS 204) và được neo
lên mạng Hyperledger Fabric theo lô dưới dạng Merkle root. Người xác minh kiểm tra văn bằng
công khai qua trang web hoặc ứng dụng di động mà không cần tài khoản.

## Mục lục

- [Tính năng](#tính-năng)
- [Kiến trúc](#kiến-trúc)
- [Công nghệ](#công-nghệ)
- [Cấu trúc thư mục](#cấu-trúc-thư-mục)
- [Chạy trên máy phát triển](#chạy-trên-máy-phát-triển)
- [Chaincode](#chaincode)
- [Kiểm thử](#kiểm-thử)
- [Triển khai](#triển-khai)
- [Bảo mật](#bảo-mật)
- [Tài liệu liên quan](#tài-liệu-liên-quan)

## Tính năng

Hệ thống có bốn vai trò.

**Quản trị viên hệ thống**

- Tạo trường cùng tài khoản quản trị của trường, gửi liên kết kích hoạt qua email.
- Cập nhật, khóa, mở khóa trường; gửi lại liên kết kích hoạt.
- Xem tổng quan toàn hệ thống và nhật ký quản trị.

**Quản trị viên trường**

- Quản lý chuyên ngành và hồ sơ sinh viên, nhập danh sách sinh viên từ Excel.
- Quản lý giao diện mẫu và mẫu văn bằng theo từng chuyên ngành.
- Quản lý khóa ký ML-DSA: chọn bộ tham số ML-DSA-44, ML-DSA-65 (mặc định) hoặc ML-DSA-87,
  kích hoạt, luân chuyển và thu hồi khóa.
- Quản lý văn bằng theo đợt cấp: nhập danh sách tốt nghiệp từ Excel vào một đợt, lọc theo
  chuyên ngành, đợt cấp, mã sinh viên hoặc họ tên.
- Cấp và ký văn bằng đơn lẻ hoặc theo đợt, có tiến độ x/y; có thể chạy ngầm nhiều lô cùng lúc.
- Ghi lô văn bằng lên Hyperledger Fabric theo đợt cấp, cho một hoặc tất cả chuyên ngành.
- Thu hồi văn bằng đơn lẻ hoặc theo đợt, gửi email thông báo kèm lý do tới sinh viên,
  ghi việc thu hồi lên sổ cái; tạo văn bằng thay thế.

**Sinh viên**

- Kích hoạt tài khoản bằng mã OTP gửi tới email trường.
- Xem thông tin cá nhân, văn bằng của mình và mã QR xác minh.

**Người xác minh**

- Xác minh công khai bằng mã văn bằng, liên kết hoặc mã QR, trên web hoặc ứng dụng di động.
- Đối chiếu tệp PDF đang giữ với văn bằng gốc; giá trị băm được tính ngay trên thiết bị.

Kết quả xác minh gồm ba lớp độc lập và bốn mức đảm bảo:

| Lớp kiểm tra | Nội dung |
|---|---|
| Chữ ký ML-DSA | Chữ ký trên manifest hợp lệ và khóa còn hiệu lực tại thời điểm ký |
| Toàn vẹn PDF | SHA-256 của tệp PDF khớp giá trị đã ký |
| Bằng chứng Fabric | Merkle proof dẫn tới root của lô trên sổ cái, chữ ký giao dịch hợp lệ, trạng thái thu hồi trên sổ cái |

| Mức đảm bảo | Điều kiện |
|---|---|
| Hợp lệ đầy đủ | Cả ba lớp hợp lệ |
| Chỉ chữ ký hợp lệ | Chữ ký và PDF hợp lệ, văn bằng chưa được ghi lên Fabric |
| Không hợp lệ | Một lớp kiểm tra thất bại |
| Đã thu hồi | Văn bằng bị thu hồi trong cơ sở dữ liệu hoặc trên sổ cái |

## Kiến trúc

```
  Trình duyệt / Ứng dụng di động
              |
              | HTTPS
              v
            nginx
         /         \
        /           \  /api/
       v             v
  Next.js         Backend Go (Gin)
  (giao diện)      |    |    |      \
                   |    |    |       \  Fabric Gateway (gRPC, X.509)
                   v    v    v        v
               MongoDB MinIO SMTP   Hyperledger Fabric
                                    (chaincode CCAAS)
```

- **Backend** tổ chức theo tầng: `routes` khai báo API và gắn middleware; `internal/middleware`
  kiểm tra JWT, trạng thái phiên, vai trò, quyền sở hữu dữ liệu và giới hạn tần suất;
  `internal/handlers` chuyển đổi yêu cầu HTTP; `internal/service` chứa nghiệp vụ;
  `internal/repository` truy cập MongoDB; `pkg/blockchain` và `pkg/database` đóng gói kết nối
  Fabric, MongoDB và MinIO.
- **MongoDB** lưu dữ liệu nghiệp vụ: trường, tài khoản, sinh viên, chuyên ngành, mẫu bằng,
  đợt cấp, văn bằng, bằng chứng chữ ký, khóa ký và nhật ký.
- **MinIO** lưu tệp PDF văn bằng.
- **Hyperledger Fabric** chỉ lưu Merkle root của từng lô, chữ ký giao dịch và bản ghi thu hồi;
  không lưu dữ liệu cá nhân hay tệp PDF.

### Luồng phát hành

1. Backend điền dữ liệu vào mẫu HTML, sinh PDF (wkhtmltopdf hoặc Chrome headless) và tính SHA-256.
2. Backend dựng manifest JSON có thứ tự trường cố định, gồm SHA-256 của PDF, rồi ký bằng khóa
   ML-DSA đang hoạt động của trường với ngữ cảnh `PQC-EDIPLOMA-V1`.
3. Khi ghi lô, mỗi văn bằng thành một lá Merkle là SHA-256 của anchor payload (mã văn bằng, khóa,
   manifest_hash, băm chữ ký và băm PDF). Phong bì giao dịch được ký ML-DSA với ngữ cảnh
   `VBS-PQC-FABRIC-TX-V1`; chaincode kiểm tra chữ ký và chỉ ghi khi lô chưa tồn tại.
4. Khi thu hồi, mỗi văn bằng mang Merkle proof tới lô đã neo; phong bì được ký với ngữ cảnh
   `VBS-PQC-FABRIC-REVOKE-V1`. Lý do thu hồi chỉ lưu dạng băm trên sổ cái.

Chi tiết về thiết kế mật mã xem [PQC_ARCHITECTURE.md](PQC_ARCHITECTURE.md).

## Công nghệ

| Thành phần | Công nghệ |
|---|---|
| Backend | Go 1.24, Gin, MongoDB Go Driver, golang-jwt, excelize |
| Chữ ký hậu lượng tử | Cloudflare CIRCL (ML-DSA-44/65/87) |
| Blockchain | Hyperledger Fabric 3.1, Fabric Gateway, Fabric Contract API, chaincode chạy theo mô hình CCAAS |
| Lưu trữ | MongoDB, MinIO |
| Sinh PDF | wkhtmltopdf hoặc Google Chrome headless |
| Frontend | Next.js 15, React 19, TypeScript, Tailwind CSS, shadcn/ui, SWR, React Hook Form, Zod |
| Ứng dụng di động | Flutter, Dart; chỉ phục vụ xác minh |
| Triển khai | nginx, systemd, Docker Compose, Let's Encrypt |

## Cấu trúc thư mục

```
.
|-- app/
|   |-- backend/              API Go
|   |   |-- cmd/server/       Điểm khởi chạy
|   |   |-- internal/         handlers, middleware, service, repository, models
|   |   |-- pkg/              Kết nối Fabric, MongoDB, MinIO; mẫu văn bằng HTML
|   |   |-- routes/           Khai báo API
|   |   |-- docker-compose.yml  MongoDB và MinIO cho máy phát triển
|   |-- frontend/             Ứng dụng web Next.js
|   |   |-- src/app/          Trang theo vai trò: admin, education-admin, student; auth, verify
|   |   |-- src/components/   Thành phần giao diện
|   |   |-- src/lib/          Gọi API, xác thực, tiện ích
|   |-- mobile/               Ứng dụng di động Flutter xác minh văn bằng (Android, iOS)
|-- chaincode/                Chaincode Go (Contract API), đóng gói image CCAAS
|-- deploy/                   Script cài đặt VPS, cấu hình nginx và systemd
|-- docs/                     Tài liệu trình diễn và ảnh minh họa
|-- PQC_ARCHITECTURE.md       Thiết kế ký số hậu lượng tử
```

## Chạy trên máy phát triển

### Yêu cầu

- Go 1.24 trở lên
- Node.js 22
- Docker và Docker Compose (cho MongoDB, MinIO)
- wkhtmltopdf hoặc Google Chrome (để sinh PDF)
- Mạng Hyperledger Fabric là tùy chọn: khi không kết nối được, backend vẫn chạy nhưng các chức
  năng ghi Blockchain báo không khả dụng.

### Backend

```bash
cd app/backend
cp ../../deploy/backend.env.example .env
# Điền các giá trị trong .env (xem bảng dưới)
docker compose --env-file .env up -d   # MongoDB và MinIO
go run ./cmd/server
```

Các biến môi trường chính:

| Biến | Ý nghĩa |
|---|---|
| `PORT` | Cổng của API, mặc định 8080 |
| `APP_URL` | Địa chỉ giao diện web, dùng trong liên kết email và mã QR |
| `MONGODB_URI`, `DB_NAME` | Kết nối MongoDB |
| `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `MINIO_USE_SSL` | Kết nối MinIO |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | Tài khoản quản trị viên hệ thống, tạo ở lần chạy đầu |
| `JWT_SECRET` | Khóa ký JWT phiên đăng nhập |
| `PQC_MASTER_KEY` | Khóa chủ 32 byte mã hóa base64, dùng mã hóa khóa bí mật ML-DSA bằng AES-256-GCM |
| `EMAIL_HOST`, `EMAIL_PORT`, `EMAIL_FROM`, `EMAIL_PASSWORD` | Máy chủ SMTP gửi thư |
| `FABRIC_*` | Kết nối Fabric Gateway: MSP, endpoint, channel, chaincode, đường dẫn chứng chỉ |
| `FABRIC_STARTUP_CHECK` | Kiểm tra kết nối Fabric khi khởi động |

Tạo khóa ngẫu nhiên:

```bash
openssl rand -base64 32   # PQC_MASTER_KEY
openssl rand -hex 32      # JWT_SECRET
```

`PQC_MASTER_KEY` phải được giữ ổn định: đổi khóa này sẽ không giải mã được các khóa ký đã lưu.

### Frontend

```bash
cd app/frontend
npm ci
# Tạo .env.local:
#   NEXT_PUBLIC_API_URL=http://localhost:8080
#   SESSION_SECRET=<openssl rand -hex 32>
npm run dev
```

Giao diện chạy tại `http://localhost:3000`. Đăng nhập bằng tài khoản quản trị viên hệ thống
đã khai báo trong `.env` của backend, tạo trường, rồi dùng tài khoản trường để quản lý văn bằng.

### Ứng dụng di động

```bash
cd app/mobile
flutter pub get
flutter run --dart-define=API_BASE_URL=http://<ip-may-chay-backend>:8080
```

Chi tiết build APK xem [app/mobile/README.md](app/mobile/README.md).

## Chaincode

Chaincode viết bằng Go với Fabric Contract API, chạy theo mô hình Chaincode-as-a-Service.
Phiên bản hiện tại ghi trong [chaincode/VERSION](chaincode/VERSION).

Các hàm phục vụ văn bằng số:

| Hàm | Chức năng |
|---|---|
| `IssueEDiplomaBatch` | Ghi một lô văn bằng (Merkle root) sau khi kiểm tra chữ ký ML-DSA; không cho ghi đè |
| `ReadEDiplomaBatch` | Đọc một lô |
| `RevokeEDiplomaBatch` | Ghi thu hồi; kiểm tra Merkle proof của từng văn bằng với lô đã neo |
| `ReadEDiplomaRevocation`, `GetEDiplomaRevocation` | Đọc bản ghi thu hồi, tra trạng thái thu hồi theo mã văn bằng |

```bash
cd chaincode
make test      # Kiểm thử đơn vị
make image     # Build image Docker kmasc-chaincode:<VERSION>
```

Hướng dẫn đóng gói và cài đặt trên mạng Fabric xem [chaincode/CCAAS_DEPLOYMENT.md](chaincode/CCAAS_DEPLOYMENT.md).

## Kiểm thử

```bash
# Backend: kiểm thử đơn vị cho manifest, mã hóa khóa, hiệu lực khóa, Merkle, thu hồi
cd app/backend
go test ./internal/...

# Chaincode
cd chaincode
make test

# Frontend: kiểm tra kiểu, lint và build production
cd app/frontend
npx tsc --noEmit
npm run lint
npm run build

# Ứng dụng di động
cd app/mobile
flutter analyze
flutter test
```

## Triển khai

Hệ thống được triển khai trên một máy chủ Ubuntu: nginx nhận HTTPS và chuyển `/api/` tới
backend, các đường dẫn còn lại tới frontend; backend và frontend chạy bằng systemd; MongoDB và
MinIO chạy bằng Docker Compose và chỉ nghe trên `127.0.0.1`.

Hướng dẫn cài đặt lần đầu xem [deploy/README.md](deploy/README.md). Sau đó, mỗi lần có mã mới:

```bash
sudo bash /opt/vbs-pqc/deploy/update.sh
```

Script lấy mã mới, build backend và frontend, rồi khởi động lại dịch vụ.

## Bảo mật

- Mật khẩu băm bằng bcrypt; token kích hoạt, đặt lại mật khẩu và OTP chỉ lưu giá trị SHA-256.
- Khóa bí mật ML-DSA được mã hóa AES-256-GCM với dữ liệu xác thực bổ sung gắn với trường và
  mã khóa; chỉ được giải mã trong bộ nhớ lúc ký và không bao giờ trả qua API.
- Hiệu lực khóa được đánh giá tại thời điểm ký, nên luân chuyển hoặc thu hồi khóa không làm
  mất hiệu lực các văn bằng đã ký trước đó.
- Phân quyền hai lớp: vai trò trong JWT và quyền sở hữu dữ liệu theo trường hoặc theo sinh viên;
  truy cập dữ liệu của đơn vị khác trả về 404.
- Phiên bị vô hiệu ngay khi tài khoản bị khóa hoặc đổi mật khẩu.
- API công khai và các API xác thực có giới hạn tần suất theo địa chỉ IP.
- Các giá trị bí mật (`.env`, chứng chỉ Fabric trong `app/backend/secrets`) không được đưa vào git.

Hạn chế hiện tại: khóa bí mật còn lưu trên máy chủ ứng dụng, chưa dùng HSM hoặc KMS; backend ghi
lô bằng một danh tính Fabric chung; chỉ văn bằng đã ghi lên Blockchain mới ghi được thu hồi lên sổ cái.

## Tài liệu liên quan

- [PQC_ARCHITECTURE.md](PQC_ARCHITECTURE.md): thiết kế ký số hậu lượng tử
- [deploy/README.md](deploy/README.md): triển khai lên VPS
- [chaincode/CCAAS_DEPLOYMENT.md](chaincode/CCAAS_DEPLOYMENT.md): đóng gói và cài đặt chaincode
- [app/backend/README.md](app/backend/README.md): backend, nhóm API
- [app/mobile/README.md](app/mobile/README.md): ứng dụng di động, cách build APK
- [docs/DEMO-VAN-BANG.md](docs/DEMO-VAN-BANG.md): kịch bản trình diễn
