FROM golang:1.20.1-alpine AS builder
RUN apk add --no-cache git
RUN go install -v github.com/projectdiscovery/katana/cmd/katana@latest

FROM alpine:3.17.2
RUN apk -U upgrade --no-cache \
    && apk add --no-cache bind-tools ca-certificates chromium
COPY --from=builder /go/bin/katana /usr/local/bin/

ENTRYPOINT ["katana"]
