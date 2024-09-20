FROM golang:1.23.1-alpine3.20 AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app
COPY . /app
RUN go mod download
RUN GO111MODULE=on go build -o katana ./cmd/katana/main.go

FROM alpine:3.18.5
RUN apk add --no-cache bind-tools ca-certificates chromium
COPY --from=builder /app/katana /usr/local/bin/

ENTRYPOINT ["katana"]
