VERSION ?= $(shell cat VERSION)
IMAGE ?= kmasc-chaincode
PACKAGE := ./contractapi
OUTPUT := dist/kmasc-chaincode

.PHONY: fmt verify test build image release clean

fmt:
	gofmt -w contractapi/*.go

verify: fmt
	go mod tidy
	go mod verify
	go vet ./...
	go build ./...
	go test ./...

test:
	go test ./...

build: verify
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -buildvcs=false -trimpath -ldflags="-s -w" -o $(OUTPUT) $(PACKAGE)

image: verify
	docker build --build-arg CHAINCODE_VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

release: build image
	@echo "Built $(OUTPUT) and $(IMAGE):$(VERSION)"

clean:
	rm -rf dist
