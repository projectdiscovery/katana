FROM alpine:latest

LABEL org.opencontainers.image.authors="ProjectDiscovery"
LABEL org.opencontainers.image.description="A next-generation crawling and spidering framework."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.title="katana"
LABEL org.opencontainers.image.url="https://github.com/projectdiscovery/katana"

RUN apk add --no-cache bind-tools ca-certificates chromium

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/katana /usr/local/bin/

ENTRYPOINT ["katana"]
