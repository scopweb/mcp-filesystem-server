---
title: Installation
description: Install and configure MCP Filesystem Server.
---

## Prerequisites

| Requirement | Minimum | Check |
|-------------|---------|-------|
| Go          | 1.26.1+ | `go version` |

## Install

### From source

```bash
go install github.com/scopweb/mcp-filesystem-server@latest
```

### Docker

```bash
docker pull ghcr.io/scopweb/mcp-filesystem-server:latest
```

### Build locally (Linux / macOS)

```bash
git clone https://github.com/scopweb/mcp-filesystem-server.git
cd mcp-filesystem-server
go build -o mcp-filesystem-server .
```

### Build locally (Windows)

```powershell
git clone https://github.com/scopweb/mcp-filesystem-server.git
cd mcp-filesystem-server
go build -ldflags="-s -w" -o mcp-filesystem-server.exe .
```

The resulting `mcp-filesystem-server.exe` can be placed anywhere and referenced by its full path in `claude_desktop_config.json`.

## Verify

```bash
mcp-filesystem-server --help
```

## Run tests

```bash
go test ./filesystemserver -v
```
