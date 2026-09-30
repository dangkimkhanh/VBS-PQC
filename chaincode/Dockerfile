FROM golang:1.24.4-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=vendor -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/kmasc-chaincode ./contractapi

FROM alpine:3.22

ARG CHAINCODE_VERSION=1.0.0

RUN addgroup -S chaincode && adduser -S -G chaincode chaincode

WORKDIR /app

COPY --from=builder --chown=chaincode:chaincode /out/kmasc-chaincode /app/kmasc-chaincode

LABEL org.opencontainers.image.title="KMASC Certificate Chaincode" \
      org.opencontainers.image.version="${CHAINCODE_VERSION}"

USER chaincode

EXPOSE 9999

ENV CHAINCODE_SERVER_ADDRESS=0.0.0.0:9999

ENTRYPOINT ["/app/kmasc-chaincode"]
