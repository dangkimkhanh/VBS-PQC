#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
MODULE_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
VERSION=${VERSION:-$(tr -d '[:space:]' < "$MODULE_ROOT/VERSION")}
IMAGE=${IMAGE:-kmasc-chaincode}

case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "Invalid VERSION: $VERSION" >&2; exit 1 ;;
esac

cd "$MODULE_ROOT"
gofmt -w contractapi/*.go
go mod tidy
go mod verify
go vet ./...
go build ./...
go test ./...
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -mod=vendor -buildvcs=false -trimpath -ldflags="-s -w" \
  -o dist/kmasc-chaincode ./contractapi
docker build --build-arg "CHAINCODE_VERSION=$VERSION" -t "$IMAGE:$VERSION" .

echo "Built dist/kmasc-chaincode and $IMAGE:$VERSION"
