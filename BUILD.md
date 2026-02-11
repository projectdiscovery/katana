# Building Katana Without CGO

This document describes how to build Katana without CGO dependencies for simplified cross-platform compilation.

## Background

Katana uses `github.com/smacker/go-tree-sitter` (via `github.com/BishopFox/jsluice`) which requires CGO for syntax tree parsing. This complicates cross-platform builds, especially for macOS/ARM64.

## Solution: Pure Go Build Tags

To build Katana without CGO:

```bash
# Disable CGO
export CGO_ENABLED=0

# Build with pure Go flags
go build -tags=purego ./cmd/katana
```

## Modified Dependencies

The following changes allow Katana to build without CGO:

1. **JavaScript Parsing**: Uses `github.com/BishopFox/jsluice` only when CGO is enabled
2. **Fallback Mode**: When CGO is disabled, JavaScript analysis falls back to regex-based parsing
3. **Build Tags**: Conditional compilation using `//go:build !purego` and `//go:build purego`

## Cross-Platform Build Examples

### Linux AMD64
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags=purego ./cmd/katana
```

### macOS ARM64 (Apple Silicon)
```bash
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -tags=purego ./cmd/katana
```

### Windows AMD64
```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags=purego ./cmd/katana
```

## Feature Parity

| Feature | With CGO | Without CGO |
|---------|----------|-------------|
| Basic Crawling | ✅ | ✅ |
| Headless Browser | ✅ | ✅ |
| JavaScript Parsing | Full (tree-sitter) | Basic (regex) |
| Form Analysis | ✅ | ✅ |
| Endpoint Extraction | ✅ | ✅ |

## CI/CD Integration

For GitHub Actions or other CI systems:

```yaml
- name: Build (No CGO)
  run: |
    export CGO_ENABLED=0
    go build -tags=purego -o katana-purego ./cmd/katana
```

## Notes

- The pure Go build excludes advanced JavaScript AST analysis features
- Core crawling functionality remains fully functional
- Consider maintaining two builds: full (with CGO) and lite (without CGO)
