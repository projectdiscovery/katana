FROM golang:1.24-alpine AS build-env
RUN apk add --no-cache git gcc musl-dev
WORKDIR /app
COPY . /app
RUN go mod download
RUN go build ./cmd/katana

FROM alpine:3.21.2
RUN apk add --no-cache bind-tools ca-certificates chromium
COPY --from=builder /app/katana /usr/local/bin/

ENTRYPOINT ["katana"]
