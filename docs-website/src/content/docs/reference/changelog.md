---
title: Changelog
description: Version history for MCP Filesystem Server.
---

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [0.6.0] - 2026-03-21

### Added
- **Tool Annotations** — all 29 tools carry `readOnlyHint`, `destructiveHint`, and `idempotentHint` annotations. MCP clients use these to improve UX and gate destructive operations.
- **MCP Roots support** — server declares `roots` capability. On the first tool call the handler fetches allowed directories from the client via `roots/list` and merges them with any CLI-supplied paths. `FilesystemHandler` is now thread-safe (`sync.RWMutex`). New `UpdateAllowedDirs()` method for external updates.
- **`read_media_file` tool** — reads any binary file and returns it as `ImageContent` (images) or base64 text blob (audio/other binary) with MIME type detection.
- **`edit_file` `dry_run` mode** — new optional boolean parameter. When `true`, the diff is computed and returned as preview without writing the file.
- **Atomic `write_file`** — writes go through a temp file in the same directory, then `os.Rename()` to the target. Prevents data loss from partial writes.
- Server can start without CLI arguments — allowed directories may be provided entirely via MCP Roots.

### Fixed
- `search_files` now supports glob patterns (`*.go`, `test_*.txt`) in addition to plain substring matching.
- `file_edit_test.go` updated for mcp-go v0.45.0 (`mcp.CallToolParams` struct change).

## [0.5.0] - 2026-03-19

### Added
- Normalizer layer (`normalizer.go`) ported from mcp-filesystem-go-ultra. 50+ rules, zero new tools.
  - Parameter aliasing: `old_str`→`old_text`, `src`→`source`, `action`→`type`, `filepath`→`path`, etc.
  - Type coercion: string `"true"` → bool, string `"3"` → float64.
  - JSON accept-both: array params accept native JSON or JSON string.
- `edit_file` already-present detection — skips gracefully if new_text present and old_text absent.
- `edit_file` literal escape recovery — handles `\n` as two chars from Claude Desktop.
- `edit_file` non-blocking no-match — returns 0 replacements instead of error.
- `batch_operations` flexible field names: `source`/`src`/`from`, `dest`/`dst`/`to`, `action`/`type`.
- `batch_operations` extra type aliases: `cp`, `rm`, `remove`.
- Rate limiter (`ratelimit.go`) — 60 calls/min sliding window.
- `convertToString` handles float64, bool, int, int64, json.Number.

### Fixed
- `FilesystemHandler` struct missing `limiter` field.
- `inprocess_test.go` wrong import path.

## [0.4.1] - 2026-03-17

### Added
- `plan_task` tool for step-by-step execution plans.

## [0.4.0] - 2026-03-16

### Added
- Chunked operations: `chunked_write`, `split_file`, `join_files`, `write_file_safe`.

## [0.3.0] - 2026-03-15

### Added
- Intelligent multi-phase text replacement in `edit_file`.

## [0.2.0] - 2026-03-14

### Added
- Analysis tools: `analyze_file`, `analyze_project`, `smart_search`, `find_duplicates`.
- `batch_operations`, `compare_files`, `generate_report`, `performance_analysis`.
- `smart_sync`, `assist_refactor`.

## [0.1.0] - 2026-03-13

### Added
- Initial release. Core filesystem operations with path validation and security.
