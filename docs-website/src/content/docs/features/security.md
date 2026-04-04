---
title: Security
description: Path validation, allow-lists, and access control.
---

## Permission Model

| Access Type | Scope | Details |
|-------------|-------|---------|
| Filesystem | Read/Write | Limited to allowed directories (CLI args or MCP Roots) |
| Network | None | Stdio transport only, no outbound connections |
| System | None | No process execution, no system calls |

:::danger[Important]
Only pass directories you trust as arguments (or expose via MCP Roots). The server has full read/write access within allowed paths.
:::

## Path Validation

Every file operation goes through path validation:

1. The path is resolved to an absolute path.
2. Symlinks are resolved to their real target.
3. The resolved path is checked against the allow-list.
4. Path traversal attempts (`../`) are blocked.

If any check fails, the operation is rejected before touching the filesystem.

## Allow-list

### CLI arguments

Allowed directories are passed as command-line arguments at startup:

```bash
mcp-filesystem-go-light /home/user/projects /tmp/scratch
```

### MCP Roots

The server declares the `roots` capability. If the MCP client provides roots via `roots/list`, those directories are merged into the allow-list on the first tool call. This allows clients to dynamically configure access without restarting the server.

The server can start with no CLI arguments and rely entirely on roots from the client:

```bash
mcp-filesystem-go-light   # roots will be provided by the client
```

Only the resulting merged list of directories (and their subdirectories) is accessible. Everything else is denied.

## Atomic Writes

`write_file` uses an atomic write pattern:
1. Content is written to a temporary file in the same directory.
2. `os.Rename()` atomically replaces the target.
3. On any error, the temporary file is removed.

This prevents data loss from partial writes and eliminates race conditions. `write_file_safe` and `chunked_write` use the same pattern.

## Symlink Handling

Symlinks are followed and the real target path is validated. If a symlink points outside the allowed directories, the operation is rejected.

## Rate Limiting

A sliding-window rate limiter restricts operations to 60 calls per minute. This prevents runaway loops from consuming resources.

## Reporting Vulnerabilities

If you find a security issue, email **security@scopweb.dev**. Do not open a public issue.
