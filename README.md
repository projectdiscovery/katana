## Building

### Standard Build (No CGO required)

```bash
go build -o katana ./cmd/katana
```

This builds Katana without tree-sitter support, using pure-Go parsers only. This is recommended for cross-platform compilation.

### Build with Tree-Sitter Support (Requires CGO)

```bash
go build -tags tree_sitter -o katana ./cmd/katana
```

This enables enhanced JavaScript parsing using tree-sitter, but requires CGO and the appropriate C compiler toolchain for your platform.

### Cross-Platform Compilation

For cross-platform builds without CGO:

```bash
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o katana ./cmd/katana
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o katana ./cmd/katana
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o katana.exe ./cmd/katana
```
