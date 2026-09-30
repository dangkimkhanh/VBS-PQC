# Kiến trúc ký số hậu lượng tử

## Phạm vi

Mỗi văn bằng được ký bằng ML-DSA theo FIPS 204. Trường chọn một trong ba bộ tham số
ML-DSA-44, ML-DSA-65 (mặc định) hoặc ML-DSA-87 khi tạo khóa; cài đặt dùng thư viện
Cloudflare CIRCL.

Không dùng JWT làm chữ ký văn bằng. JWT chỉ xác thực phiên và phân quyền API.
Không thay MSP của Hyperledger Fabric bằng PQC vì Fabric hiện vẫn dùng danh tính
X.509 cổ điển. Fabric dùng để neo hash của dữ liệu và bằng chứng chữ ký.

## Dữ liệu được ký

ML-DSA ký một manifest có schema cố định, bao gồm ID văn bằng, trường, khoa,
người nhận, thông tin cấp bằng, thời điểm ký UTC và SHA-256 của tệp PDF. Context
`PQC-EDIPLOMA-V1` ngăn chữ ký bị tái sử dụng ngoài miền văn bằng.

Proof lưu cùng văn bằng chứa:

- phiên bản proof;
- thuật toán và context;
- ID cùng dấu vân tay khóa công khai;
- hash manifest;
- chữ ký ML-DSA;
- thời điểm ký UTC.

Nếu văn bằng có PQC proof mới, leaf đưa vào Merkle tree ràng buộc ID văn bằng,
ID trường, phiên bản anchor, thuật toán, ID và fingerprint khóa công khai, hash
manifest, hash chữ ký và hash PDF. Fabric chỉ lưu Merkle root của batch, không
lưu PDF, chữ ký hay khóa công khai. Dữ liệu cũ tiếp tục dùng cách tính hash cũ
để giữ tương thích.

Mỗi batch có ID `EDIP-<university>-<merkle-root>` nên một đợt phát hành mới
không thể ghi đè root của đợt cũ. Chaincode ghi thêm transaction ID, thời điểm
giao dịch và danh tính Fabric của bên phát hành.

## Chữ ký giao dịch và thu hồi trên sổ cái

Phong bì giao dịch ghi lô được ký bằng khóa ML-DSA đang hoạt động của trường với
context `VBS-PQC-FABRIC-TX-V1`. Chaincode kiểm tra bộ tham số, dấu vân tay khóa
công khai đi kèm, giá trị băm phong bì và chữ ký trước khi ghi, nên mỗi lô có một
bằng chứng hậu lượng tử độc lập với chữ ký X.509 của Fabric.

Thu hồi văn bằng đã neo được ghi lên sổ cái bằng hàm `RevokeEDiplomaBatch`. Mỗi
mục gồm mã văn bằng, mã lô, anchor payload, Merkle proof, thời điểm thu hồi và
SHA-256 của lý do; lý do không được ghi ở dạng rõ. Phong bì được ký với context
`VBS-PQC-FABRIC-REVOKE-V1`, khác context ghi lô, nên chữ ký giao dịch cấp bằng
không thể dùng lại làm chữ ký thu hồi. Chaincode chỉ ghi khi anchor payload băm ra
đúng lá, Merkle proof dẫn tới root của một lô đã neo của cùng trường và văn bằng
chưa bị thu hồi trên sổ cái. Khi xác minh, văn bằng bị coi là đã thu hồi nếu cơ sở
dữ liệu hoặc sổ cái ghi nhận thu hồi.

## Vòng đời khóa

Một khóa có một trong bốn trạng thái:

1. `pending`: vừa tạo, chưa được phép ký.
2. `active`: khóa duy nhất được dùng để ký mới.
3. `retired`: được thay bằng khóa mới; vẫn dùng xác minh chữ ký cũ.
4. `revoked`: không bao giờ được kích hoạt/ký lại; khóa công khai vẫn được giữ.

Kích hoạt khóa mới tự chuyển khóa đang hoạt động sang `retired`. Không có API
xóa khóa vì xóa khóa công khai sẽ phá xác minh lịch sử. Khóa `retired` không
được kích hoạt lại; nếu cần quay vòng phải tạo một cặp khóa mới.

Thu hồi thông thường sau khi luân chuyển không làm vô hiệu chữ ký đã tạo khi
khóa còn hợp lệ. Nếu khóa bị lộ, quản trị viên nhập
`compromise_effective_at`; chữ ký từ thời điểm đó trở đi bị đánh dấu không hợp
lệ. `signed_at` nằm trong manifest nên không thể sửa mà không phá chữ ký. Tuy
nhiên, kẻ đã lấy được khóa riêng vẫn có thể tạo chữ ký mới và khai thời gian cũ;
vì vậy timestamp giao dịch Fabric hoặc TSA phải được dùng làm mốc thời gian tin
cậy trong triển khai production.

## Bảo vệ khóa riêng

Khóa riêng ML-DSA được mã hóa AES-256-GCM trước khi lưu MongoDB. AAD ràng
buộc ciphertext với ID khóa và ID trường. API không serialize khóa riêng.

Biến `PQC_MASTER_KEY` phải là base64 của đúng 32 byte ngẫu nhiên. Không dùng
JWT secret làm khóa mã hóa và không commit master key. Production nên thay lớp
lưu khóa bằng KMS/HSM; interface service/repository cho phép thay mà không đổi
manifest hoặc dữ liệu proof.

Đổi `PQC_MASTER_KEY` cần quy trình re-wrap khóa riêng. Khóa công khai và việc
xác minh lịch sử không phụ thuộc master key nên vẫn hoạt động nếu master key cũ
không còn, nhưng không thể dùng khóa riêng cũ để ký mới.

## API

Các API sau yêu cầu vai trò `university_admin` và luôn giới hạn theo
`university_id` trong JWT:

- `GET /api/v1/pqc/keys`
- `POST /api/v1/pqc/keys`
- `POST /api/v1/pqc/keys/:id/activate`
- `POST /api/v1/pqc/keys/:id/revoke`
- `POST /api/v1/pqc/ediplomas/:id/sign`
- `POST /api/v1/ediplomas/issue-pqc-batch` (cấp và ký theo đợt)
- `POST /api/v1/blockchain/push-ediploma` (ghi lô lên Fabric)
- `POST /api/v1/blockchain/push-revocations` (ghi thu hồi lên Fabric)

API xác minh công khai:

- `GET /api/v1/pqc/ediplomas/:id/verify`
- `GET /api/v1/public/degrees/:id`

API này thực hiện một luồng thống nhất:

1. dựng lại manifest và xác minh ML-DSA;
2. kiểm tra khóa có hiệu lực tại thời điểm ký;
3. tải PDF thực tế từ MinIO và so sánh SHA-256;
4. kiểm tra Merkle proof với root đọc trực tiếp từ Fabric và đọc trạng thái thu
   hồi trên sổ cái.

`valid=true` chỉ khi cả bốn lớp đều hợp lệ. Nếu chữ ký và PDF đúng nhưng chưa
đưa lên Fabric, API trả `assurance_level=signature_only` thay vì gọi văn bằng
không hợp lệ về mặt mật mã.

Không tạo registry khóa thứ hai trong chaincode ở phiên bản này. Fingerprint
khóa đã được ràng buộc trong leaf phát hành; trạng thái vận hành của khóa nằm ở
key repository. Cách này tránh hai nguồn trạng thái dễ lệch nhau. Hướng phát
triển là đăng ký khóa công khai của từng trường trên sổ cái để chaincode tự kiểm
tra khóa ký giao dịch thuộc đúng trường.

## Chính sách vận hành khuyến nghị

- Luân chuyển khóa định kỳ 12 tháng hoặc theo chính sách của đơn vị.
- Luân chuyển ngay khi thay người/quyền quản trị hoặc nghi ngờ sự cố.
- Sau khi ký, đưa batch lên Fabric trước khi công bố văn bằng.
- Không cho ký lại văn bằng đã neo blockchain.
- Sao lưu master key tách biệt, có kiểm soát truy cập và nhật ký khôi phục.
- Production nên yêu cầu xác thực lại hoặc phê duyệt hai người khi kích hoạt và
  thu hồi khóa.
