---
title: Installation
description: Install and configure MCP Filesystem Light.
---

## Prerequisites

| Requirement | Minimum | Check |
|-------------|---------|-------|
| Go          | 1.26.1+ | `go version` |

## Install

### From source

```bash
go install github.com/scopweb/mcp-filesystem-go-light@latest
```

### Docker

```bash
docker pull ghcr.io/scopweb/mcp-filesystem-go-light:latest
```

### Build locally (Linux / macOS)

```bash
git clone https://github.com/scopweb/mcp-filesystem-go-light.git
cd mcp-filesystem-go-light
go build -o mcp-filesystem-go-light .
```

### Build locally (Windows)

```powershell
git clone https://github.com/scopweb/mcp-filesystem-go-light.git
cd mcp-filesystem-go-light
go build -ldflags="-s -w" -o mcp-filesystem-go-light.exe .
```

The resulting `mcp-filesystem-go-light.exe` can be placed anywhere and referenced by its full path in `claude_desktop_config.json`.

## Verify

```bash
mcp-filesystem-go-light --help
```

## Run tests

```bash
go test ./filesystemserver -v
```
