# Backend VBS PQC

API Go (Gin) của hệ thống quản lý văn bằng số sử dụng ký số hậu lượng tử. Phần tổng quan,
kiến trúc và cách chạy toàn hệ thống xem [README gốc](../../README.md).

## Cấu trúc

```
cmd/server/            Điểm khởi chạy: nạp cấu hình, kết nối MongoDB, MinIO, Fabric, tạo router
routes/router.go       Khai báo API và gắn middleware theo vai trò
internal/
  middleware/          JWT, trạng thái phiên, vai trò, quyền sở hữu dữ liệu, giới hạn tần suất
  handlers/            Chuyển đổi yêu cầu HTTP sang lời gọi service
  service/             Nghiệp vụ: xác thực, trường, sinh viên, đợt cấp, văn bằng,
                       ký ML-DSA, ghi lô và ghi thu hồi lên Fabric, gửi email
  repository/          Truy cập MongoDB
  models/              Dữ liệu, manifest, proof, bản ghi lô và thu hồi
pkg/blockchain/        Fabric Gateway client
pkg/database/          Kết nối MongoDB, MinIO; mẫu văn bằng HTML có sẵn
utils/                 JWT, băm mật khẩu, email, ngày tháng
docker-compose.yml     MongoDB và MinIO cho máy phát triển
```

## Chạy

```bash
cp ../../deploy/backend.env.example .env   # rồi điền các giá trị
docker compose --env-file .env up -d        # MongoDB và MinIO
go run ./cmd/server
```

API mặc định chạy ở cổng `8080`, tiền tố `/api/v1`. Tài khoản quản trị viên hệ thống được tạo
ở lần chạy đầu từ `ADMIN_EMAIL` và `ADMIN_PASSWORD`. Danh sách biến môi trường xem README gốc.

Không kết nối được Fabric thì backend vẫn chạy; các chức năng ghi lô, ghi thu hồi báo không
khả dụng và lớp bằng chứng Blockchain khi xác minh được báo là không xác định.

## Nhóm API chính

| Tiền tố | Vai trò | Chức năng |
|---|---|---|
| `/auth` | Công khai | Đăng nhập, kích hoạt tài khoản, OTP, quên và đặt lại mật khẩu |
| `/universities` | Quản trị viên hệ thống | Tạo, cập nhật, khóa, mở khóa trường; gửi lại liên kết kích hoạt |
| `/faculties`, `/users` | Quản trị viên trường | Chuyên ngành, hồ sơ sinh viên, nhập Excel |
| `/template-samples`, `/templates` | Quản trị viên trường | Giao diện mẫu và mẫu văn bằng |
| `/issuance-rounds` | Quản trị viên trường | Tạo và tìm đợt cấp |
| `/ediplomas` | Quản trị viên trường | Hồ sơ văn bằng, cấp và ký theo đợt, thu hồi, văn bằng thay thế |
| `/pqc` | Quản trị viên trường | Khóa ký ML-DSA, ký văn bằng |
| `/blockchain` | Quản trị viên trường | Ghi lô văn bằng và ghi thu hồi lên Fabric |
| `/public/degrees/:id`, `/pqc/ediplomas/:id/verify` | Công khai | Xác minh văn bằng |

## Kiểm thử

```bash
go test ./internal/...
```

Các kiểm thử đơn vị bao phủ manifest, mã hóa khóa bí mật, hiệu lực khóa theo thời điểm ký,
ba bộ tham số ML-DSA, Merkle proof, bản ghi thu hồi và dữ liệu điền vào văn bằng.
