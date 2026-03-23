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

### Build locally

```bash
git clone https://github.com/scopweb/mcp-filesystem-server.git
cd mcp-filesystem-server
go build -o mcp-filesystem-server .
```

## Verify

```bash
mcp-filesystem-server --help
```

## Run tests

```bash
go test ./filesystemserver -v
```
