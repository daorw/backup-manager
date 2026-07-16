# GitHub CI/CD Release Pipeline

## Workflow Overview

Multi-platform build + release workflow triggered by Git tag pushes. Three
independent platform build jobs run in parallel, each producing binaries for
multiple architectures. A final release job collects all artifacts and creates
a GitHub Release with auto-generated notes.

## Trigger

```yaml
on:
  push:
    tags:
      - '*'
```

## Permissions

```yaml
permissions:
  contents: write  # Required for creating releases
```

## Build Jobs

### Pattern

Each platform job follows this pattern:

1. `actions/checkout@v4` with `fetch-depth: 0`
2. `actions/setup-go@v5` reading version from `go.mod` + caching
3. `actions/setup-node@v4` with Node 22 + npm cache from `frontend/package-lock.json`
4. Build frontend: `cd frontend && npm ci && npm run build`
5. Build Go binary: `go build -ldflags="-s -w" -o $OUT .`
6. `actions/upload-artifact@v4` with platform-specific name

### Linux Build

```yaml
build-linux:
  runs-on: ubuntu-latest
  strategy:
    matrix:
      goarch: [amd64, arm64]
  env:
    GOOS: linux
    GOARCH: ${{ matrix.goarch }}
    CGO_ENABLED: 0    # Pure Go, no systray dependency
```

### macOS Build

```yaml
build-darwin:
  runs-on: macos-latest    # Must be macOS for CGO + Cocoa
  strategy:
    matrix:
      goarch: [amd64, arm64]
  env:
    GOOS: darwin
    GOARCH: ${{ matrix.goarch }}
    CGO_ENABLED: 1         # Required for getlantern/systray (Cocoa)
```

### Windows Build

```yaml
build-windows:
  runs-on: ubuntu-latest   # Cross-compile from Linux
  strategy:
    matrix:
      goarch: [amd64]
  env:
    GOOS: windows
    GOARCH: ${{ matrix.goarch }}
    CGO_ENABLED: 0         # Pure Go systray via Windows syscall
```

Binary output appends `.exe` for Windows:

```yaml
out="backup-manager-${GOOS}-${GOARCH}.exe"  # Windows only
# vs
out="backup-manager-${GOOS}-${GOARCH}"      # Linux/macOS
```

## Release Job

```yaml
release:
  needs: [build-linux, build-darwin, build-windows]
  runs-on: ubuntu-latest
  steps:
    - uses: actions/download-artifact@v4
      with:
        path: artifacts
        merge-multiple: true

    # Generate release notes from git log
    - run: |
        git log --oneline $PREV_TAG..$TAG >> release-notes.md
        # Append file size table

    - uses: softprops/action-gh-release@v2
      with:
        files: artifacts/*
        body_path: release-notes.md
```

## Naming Convention

Binaries follow: `<app>-<os>-<arch>[.exe]`

Examples:
- `backup-manager-linux-amd64`
- `backup-manager-darwin-arm64`
- `backup-manager-windows-amd64.exe`

## Build Optimization

- `-ldflags="-s -w"` strips debug info and symbol table, reducing binary size
- `go build` runs AFTER frontend build so embed.FS picks up `frontend/dist/`
- Use `npm ci` instead of `npm install` for deterministic installs in CI
