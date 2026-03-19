---
title: Security
description: Path validation, allow-lists, and access control.
---

## Permission Model

| Access Type | Scope | Details |
|-------------|-------|---------|
| Filesystem | Read/Write | Limited to directories passed at startup |
| Network | None | Stdio transport only, no outbound connections |
| System | None | No process execution, no system calls |

:::danger[Important]
Only pass directories you trust as arguments. The server has full read/write access within allowed paths.
:::

## Path Validation

Every file operation goes through path validation:

1. The path is resolved to an absolute path.
2. Symlinks are resolved to their real target.
3. The resolved path is checked against the allow-list.
4. Path traversal attempts (`../`) are blocked.

If any check fails, the operation is rejected before touching the filesystem.

## Allow-list

Allowed directories are passed as command-line arguments at startup:

```bash
mcp-filesystem-server /home/user/projects /tmp/scratch
```

Only these directories (and their subdirectories) are accessible. Everything else is denied.

## Symlink Handling

Symlinks are followed and the real target path is validated. If a symlink points outside the allowed directories, the operation is rejected.

## Rate Limiting

A sliding-window rate limiter restricts operations to 60 calls per minute. This prevents runaway loops from consuming resources.

## Reporting Vulnerabilities

If you find a security issue, email **security@scopweb.dev**. Do not open a public issue.
