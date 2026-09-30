# Kịch bản trình diễn đồ án

1. Đăng nhập tài khoản quản trị viên hệ thống, tạo trường. Quản trị viên trường nhận email
   kích hoạt và đặt mật khẩu.
2. Đăng nhập tài khoản trường, mở **Tổng quan**. Thống kê lấy từ dữ liệu thực của trường.
3. Tạo chuyên ngành, nhập danh sách sinh viên từ Excel, chuẩn bị mẫu bằng cho chuyên ngành.
4. Trong thẻ **Khóa ký**, tạo khóa ML-DSA-65 (hoặc ML-DSA-44, ML-DSA-87) và kích hoạt.
   Khóa cũ được giữ lại để xác minh các văn bằng đã ký.
5. Trong thẻ **Văn bằng**, bấm **Tải Excel lên**, chọn đợt cấp có sẵn hoặc tạo đợt mới, rồi
   tải danh sách tốt nghiệp.
6. Lọc chuyên ngành, đợt cấp, trạng thái **Chưa cấp**, bấm **Cấp & ký** và chọn mẫu bằng.
   Hộp thoại hiện tiến độ x/y; có thể đóng hộp thoại, tiến độ chuyển sang khu thông báo.
7. Chuyển trạng thái **Đã cấp**, bấm **Ghi lên Blockchain** để neo lô văn bằng của đợt lên
   Hyperledger Fabric.
8. Mở mã QR ở một dòng văn bằng. Trang `/verify/<id>` hoạt động không cần đăng nhập và hiển
   thị riêng kết quả chữ ký ML-DSA, toàn vẹn PDF và bằng chứng Blockchain. Quét từ điện thoại
   cần địa chỉ HTTPS truy cập được, không dùng localhost. Có thể quét bằng ứng dụng di động.
9. Ở mục đối chiếu, chọn PDF nguyên bản: giá trị băm trùng. Chọn bản đã chỉnh sửa: giá trị
   băm khác. Tệp chỉ được xử lý trên thiết bị của người xác minh.
10. Thu hồi một văn bằng hoặc cả đợt, nhập lý do; sinh viên nhận email kèm lý do. Lọc
    trạng thái **Đã thu hồi**, bấm **Ghi thu hồi lên Blockchain**. Trang xác minh báo văn bằng
    đã thu hồi, kể cả khi dữ liệu trong cơ sở dữ liệu bị sửa.
11. Tạo văn bằng thay thế với số hiệu, số vào sổ, ngày cấp mới. Bản thay thế thuộc cùng đợt
    cấp, được cấp và ký lại như văn bằng thường; bản cũ vẫn được giữ.
12. Kiểm tra nhật ký ở Tổng quan và lịch sử trong chi tiết văn bằng.

## Phạm vi

- Các vai trò: quản trị viên hệ thống, quản trị viên trường, sinh viên và người xác minh
  công khai. Quản trị viên trường kiêm người ký; chưa có quy trình phê duyệt hai người.
- Chỉ văn bằng đã được ghi lên Blockchain mới ghi được thu hồi lên sổ cái; văn bằng chưa neo
  chỉ được đánh dấu thu hồi trong cơ sở dữ liệu.
- Nhật ký không có API sửa, xóa, nhưng chưa phải lưu trữ chống sửa bởi quản trị cơ sở dữ liệu.
- Trang xác minh công khai chỉ hiển thị họ tên, tên bằng, trường, số hiệu và ngày cấp. Không
  công khai email, số định danh hay khóa bí mật.
- Cấp và ký chạy trong trình duyệt của quản trị viên, mỗi văn bằng một yêu cầu; đóng hẳn tab
  thì lô dừng lại, bấm lại để ký tiếp phần còn lại.
