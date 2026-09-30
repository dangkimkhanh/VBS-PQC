# Chaincode văn bằng số (Chaincode-as-a-Service)

## Thông tin module

- Module trong repository: `chaincode/`
- Go module: `github.com/tuyenngduc/certificate-management-system/chaincode`
- Package chứa `main`: `./contractapi`
- Fabric mục tiêu: `3.1`, mạng quản lý bằng ChainLaunch
- Go: `1.24.4`
- MSP ứng dụng: `KMAUniverMSP`, `VNUUniverMSP`
- Channel: `ediploma-channel`
- Chaincode name: `ediploma-chaincode`
- Cổng CCAAS: `9999`

Chaincode chạy dưới dạng service gRPC độc lập. Container bắt buộc nhận `CHAINCODE_ID`; giá trị này phải trùng package ID mà peer/ChainLaunch dùng khi cài chaincode.

## Quy trình tạo phiên bản mới

Mỗi lần sửa chaincode:

1. Sửa source và unit test trong `contractapi/`.
2. Tăng image version trong file `VERSION`, ví dụ `1.0.0` thành `1.0.1`.
3. Chạy bộ build tự động.
4. Push image với tag mới; không ghi đè tag đã triển khai.
5. Trong ChainLaunch, thực hiện upgrade chaincode với image mới và tăng lifecycle `sequence` thêm 1.

Ba giá trị không nên nhầm lẫn:

- `VERSION`: tag Docker image của source CCAAS.
- Chaincode definition version: phiên bản khai báo trong lifecycle Fabric.
- Chaincode sequence: số nguyên phải tăng mỗi lần commit definition mới trên cùng channel.

## Build trên Windows

Từ thư mục `chaincode`:

```powershell
.\scripts\build.ps1
```

Chỉ kiểm tra Go và tạo binary Linux, không build Docker:

```powershell
.\scripts\build.ps1 -SkipDocker
```

Build một version chỉ định mà không sửa file `VERSION`:

```powershell
.\scripts\build.ps1 -Version 1.0.1
```

Script tự chạy `gofmt`, `go mod tidy`, `go mod verify`, `go vet`, `go build`, `go test`, tạo `dist/kmasc-chaincode`, rồi build Docker image.

## Build trên Linux/VPS

```bash
cd chaincode
chmod +x scripts/build.sh
./scripts/build.sh
```

Hoặc dùng Make:

```bash
make verify
make build
make image VERSION=1.0.0
make release VERSION=1.0.0
```

Các lệnh Go tương đương:

```bash
gofmt -w contractapi/*.go
go mod tidy
go mod verify
go vet ./...
go build ./...
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -mod=vendor -buildvcs=false -trimpath -ldflags="-s -w" \
  -o dist/kmasc-chaincode ./contractapi
```

## Build và kiểm tra Docker image

```bash
docker build \
  --build-arg CHAINCODE_VERSION=1.0.0 \
  -t kmasc-chaincode:1.0.0 \
  .
```

Kiểm tra image:

```bash
docker image inspect kmasc-chaincode:1.0.0
docker history kmasc-chaincode:1.0.0
```

Chạy thử:

```bash
docker run --rm \
  --name kmasc-chaincode-test \
  -e CHAINCODE_ID=test-package-id \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999 \
  -p 9999:9999 \
  kmasc-chaincode:1.0.0
```

Log hợp lệ phải có thông báo server bắt đầu lắng nghe trên `0.0.0.0:9999`. `test-package-id` chỉ dùng để kiểm tra container khởi động, không dùng khi triển khai thật.

Xem log container chạy nền:

```bash
docker logs -f kmasc-chaincode
```

## Push lên Docker Hub

Đăng nhập:

```bash
docker login
```

Tag và push:

```bash
docker tag kmasc-chaincode:1.0.0 \
  <DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0

docker push \
  <DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0
```

Không dùng lại một tag cho hai binary khác nhau. Nếu registry hỗ trợ digest, nên ghi lại digest sau khi push:

```bash
docker inspect --format='{{index .RepoDigests 0}}' \
  <DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0
```

## Pull và chạy trên VPS

```bash
docker login
docker pull docker.io/<DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0
docker image inspect docker.io/<DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0
```

Nếu ChainLaunch không tự quản lý container, ví dụ chạy trực tiếp:

```bash
docker run -d \
  --name kmasc-chaincode \
  --restart unless-stopped \
  -e CHAINCODE_ID='<PACKAGE_ID_FROM_CHAINLAUNCH>' \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999 \
  -p 9999:9999 \
  docker.io/<DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0
```

Không lưu package ID, private key hoặc credential vào Dockerfile/image.

## Giá trị nhập trong ChainLaunch

```text
MSP: KMAUniverMSP
Channel: ediploma-channel
Docker Image: docker.io/<DOCKERHUB_USERNAME>/kmasc-chaincode:1.0.0
Container Port: 9999
```

`Chaincode Address` phải là hostname hoặc địa chỉ mà peer Fabric có thể kết nối tới chaincode container trên cổng `9999`, dạng:

```text
<PEER_REACHABLE_CHAINCODE_HOST>:9999
```

Không dùng `0.0.0.0:9999` làm Chaincode Address phía peer: đó chỉ là địa chỉ bind bên trong container. Không mặc định dùng `localhost:9999` nếu peer và chaincode chạy ở container/máy khác. Giá trị cụ thể phụ thuộc Docker network, DNS hoặc cách ChainLaunch tạo service.

Khi upgrade cùng một chaincode trên `ediploma-channel`:

- dùng image tag mới;
- giữ nguyên chaincode name nếu muốn nâng cấp state hiện tại;
- tăng sequence;
- đảm bảo endorsement policy và collections không vô tình thay đổi;
- approve definition cho các organization cần thiết rồi commit;
- xác nhận package ID được truyền vào container qua `CHAINCODE_ID`.

## Biến môi trường runtime

| Biến | Bắt buộc | Giá trị |
|---|---:|---|
| `CHAINCODE_ID` | Có | Package ID do Fabric/ChainLaunch tạo |
| `CHAINCODE_SERVER_ADDRESS` | Không | Mặc định `0.0.0.0:9999` |

TLS tại chaincode server hiện đang tắt. Kết nối peer-to-chaincode phải nằm trên network riêng/được bảo vệ; nếu triển khai qua mạng không tin cậy thì cần bật TLS CCAAS trước khi production.
