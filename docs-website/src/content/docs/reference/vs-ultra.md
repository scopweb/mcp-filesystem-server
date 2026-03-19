---
title: vs Ultra
description: Comparison between MCP Filesystem Server and the full-featured ultra edition.
---

## Feature Comparison

| Feature | This server | [Ultra](https://github.com/scopweb/mcp-filesystem-go-ultra) |
|---------|------------|------|
| Tools | 27 | 16 (consolidated) |
| Normalizer | Yes (ported) | Yes (original) |
| Backup/restore | No | Yes |
| Pipeline executor | No | Yes |
| Audit trail | No | Yes |
| Regex edit mode | No | Yes |
| WSL integration | No | Yes |
| Hooks system | No | Yes |
| Dashboard | No | Yes |
| Streaming | No | Yes |

## When to use this server

- You want a simple, lightweight MCP filesystem server.
- You don't need backup/restore, pipelines, or WSL.
- You want the normalizer intelligence without the extra features.
- You prefer more granular tools (27 individual) over consolidated ones (16 in ultra).

## When to use ultra

- You need backup/restore for safe editing workflows.
- You use WSL and need cross-environment file access.
- You want pipeline execution for complex multi-step operations.
- You need audit trails and dashboards for monitoring.
- You want streaming for large file operations.

## Migration

Both servers use the same MCP protocol and normalizer layer. Tool names differ — ultra consolidates operations into fewer, more powerful tools. There is no automated migration path; reconfigure your MCP client to point to the other server.
