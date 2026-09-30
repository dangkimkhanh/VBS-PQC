# Ứng dụng di động xác minh văn bằng

Ứng dụng Flutter giúp người xác minh kiểm tra văn bằng số của hệ thống VBS PQC. Ứng dụng
không có chức năng đăng nhập; mọi thông tin đều lấy từ API xác minh công khai. Phần tổng quan
của hệ thống xem [README gốc](../../README.md).

## Chức năng

- Quét mã QR trên văn bằng bằng camera, chọn ảnh có mã QR hoặc nhập mã văn bằng, liên kết
  xác minh.
- Hiển thị kết luận xác minh theo màu, thông tin văn bằng và kết quả ba lớp kiểm tra: chữ ký
  ML-DSA, toàn vẹn tệp PDF và bằng chứng Blockchain.
- Đối chiếu tệp PDF đang giữ với văn bằng gốc; giá trị SHA-256 được tính ngay trên điện thoại.

Ứng dụng đọc mã văn bằng từ đường dẫn dạng `/verify/{mã}` rồi gọi
`GET /api/v1/public/degrees/{mã}` của backend.

## Cấu trúc

```
lib/
  main.dart                     Khởi chạy ứng dụng
  config.dart                   Địa chỉ backend (API_BASE_URL)
  theme.dart                    Màu sắc, kiểu chữ
  models/degree_verification.dart  Dữ liệu kết quả xác minh
  services/api_service.dart     Đọc mã xác minh, gọi API xác minh
  screens/                      Màn hình khởi động, trang chủ, quét QR, kết quả xác minh
  widgets/                      Thành phần dùng chung
assets/                         Logo và biểu tượng ứng dụng
test/                           Kiểm thử đơn vị
android/, ios/, web/            Mã nền tảng
```

## Yêu cầu

- Flutter 3.35 trở lên (Dart 3.9)
- Android SDK để build APK; Xcode để build cho iOS

## Chạy và build

```bash
flutter pub get
flutter run
```

Địa chỉ backend mặc định khai báo trong `lib/config.dart`. Đổi địa chỉ khi build bằng tham số
`API_BASE_URL`:

```bash
flutter build apk --release --dart-define=API_BASE_URL=https://<ten-mien-cua-he-thong>
```

Tệp APK nằm ở `build/app/outputs/flutter-apk/app-release.apk`.

Chạy bản web (dùng để xem thử giao diện trên trình duyệt):

```bash
flutter run -d chrome --dart-define=API_BASE_URL=https://<ten-mien-cua-he-thong>
```

## Kiểm thử

```bash
flutter analyze
flutter test
```

Kiểm thử đơn vị bao phủ việc đọc mã xác minh từ mã hoặc liên kết và việc đọc dữ liệu trả về
từ API xác minh.

## Biểu tượng ứng dụng

Biểu tượng được sinh từ `assets/icon/` bằng `flutter_launcher_icons`:

```bash
dart run flutter_launcher_icons
```
