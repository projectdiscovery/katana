# Building Katana

## Standard Build (with CGO)

```bash
go build -v .
```

This will build Katana with full JavaScript parsing capabilities using jsluice (requires CGO).

## Pure Go Build (without CGO)

For cross-compilation or environments without CGO:

```bash
CGO_ENABLED=0 go build -tags=without_jsluice -v .
```

This builds Katana without jsluice JavaScript parsing. All other features remain functional.

## Cross-Compilation

### macOS (ARM64)

```bash
# With CGO (requires macOS SDK)
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -v .

# Without jsluice (pure Go, no SDK needed)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -tags=without_jsluice -v .
```

### Windows

```bash
# Pure Go build (recommended)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags=without_jsluice -v .
```

### Linux

```bash
# Static binary (pure Go)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags=without_jsluice -v .
```

## Build Tags

- `without_jsluice` - Disables jsluice JavaScript endpoint extraction (removes CGO dependency)

## Note on jsluice Dependency

Katana uses [jsluice](https://github.com/BishopFox/jsluice) for JavaScript endpoint extraction, which depends on `go-tree-sitter` and requires CGO.

If you need pure Go builds:
- Use the `without_jsluice` build tag
- JavaScript endpoint extraction will be disabled
- All other crawling features work normally

For more details, see issue #1367.
