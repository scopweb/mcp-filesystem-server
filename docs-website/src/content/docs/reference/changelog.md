---
title: Changelog
description: Version history for MCP Filesystem Server.
---

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
- **Official MCP compatibility aliases** — 3 tool name aliases for clients trained on the official MCP filesystem server: `read_text_file` → `read_file`, `search_files` → `search`, `directory_tree` → `tree`.
- **Recommended workflow in instructions** — 5-step workflow (navigate → locate → read range → edit → verify) sent during MCP initialize.
- **`edit_file` large-edit tip** — success message includes a TIP to use `compare_files` when edits affect >10 lines.
- **`/filesystem-light-tools` skill** — Claude Code skill for tool discovery. Solves Claude Desktop's lazy tool loading.
- **`help` tool** — returns the full tool catalog with usage rules and best practices.
- **`server.WithInstructions()`** — sends tool catalog during MCP initialize handshake.
- **`read_file` outline mode** — symbol index for 14 languages with line numbers.
- **`list_directory` depth** — multi-level recursive listing (1-10).
- **MCP logging, progress notifications, content annotations, tool titles** — full MCP spec compliance.
- **Roots change notifications** — refreshes allowed directories mid-session.
- **Context cancellation** — cooperative cancellation across all operations.
- **Development audit logging** — `--dev --log-dir` mode with `operations.jsonl` and `metrics.json`.
- **Log inspection tools** — `cmd/logview`, `cmd/logdashboard`, and local dashboard.

### Testing
- **40 security and edge-case tests** — directory traversal, symlink attacks, Windows paths, roots lifecycle, concurrency, resource handler, copy/move destination validation, edit_file features.

### Changed
- **Tool descriptions rewritten for discoverability** — action keywords, recommended workflows, reduced cross-reference noise. Optimized for Claude Desktop's semantic tool search.
- **Tool errors** — `IsError: true` (tool-level) instead of protocol-level errors.
- **Output sanitization** — 1MB truncation and UTF-8 validation in post-processing.

---

## [1.0.0] - 2026-03-23

### Added
- **`search` tool** — unified search replacing three separate tools. Use `mode: "files"` (name/glob), `mode: "content"` (regex + `context_lines`), or `mode: "duplicates"` (MD5 hash scan).
- **`write_file` `create_backup`** — optional boolean. Creates a `.backup` copy of the existing file before overwriting.
- **`write_file` chunked streaming** — optional `chunk_index` / `total_chunks` params replace the old `chunked_write` tool.

### Removed (breaking)

| Old tool | Migration |
|----------|-----------|
| `search_files` | `search` with `mode: "files"` |
| `smart_search` | `search` with `mode: "content"` |
| `find_duplicates` | `search` with `mode: "duplicates"` |
| `write_file_safe` | `write_file` with `create_backup: true` |
| `chunked_write` | `write_file` with `chunk_index` + `total_chunks` |
| `split_file` | removed |
| `join_files` | removed |
| `analyze_file` | removed (was stub) |
| `performance_analysis` | removed (was stub) |
| `generate_report` | removed (was stub) |
| `smart_sync` | removed (was stub) |
| `assist_refactor` | removed (was stub) |

---

## [0.6.4] - 2026-03-23

### Fixed
- **`read_file` line range memory** — now streams with `bufio.Scanner`. Large files no longer fully loaded when only a line range is requested.
- **Dead code removed** — `handleAdvancedTextSearch` and helpers deleted.

---

## [0.6.3] - 2026-03-23

### Added
- **`smart_search` `context_lines`** — new optional number param. When `include_content` is true, each match includes N surrounding lines (before + after), equivalent to `grep -C N`. Default: 0.

### Changed
- `list_directory` tool description clarified: explicitly states it lists one level only, and when to prefer it over `tree` or `search_files`.

---

## [0.6.2] - 2026-03-23

### Added
- **`read_file` line range** — new optional `start_line` / `end_line` params (1-based, inclusive). Reads a section of a large file without loading it entirely. The 5 MB inline limit is bypassed when a line range is requested.

---

## [0.6.1] - 2026-03-23

### Changed
- Go upgraded to **1.26.1** (patch release with stdlib security fixes: GO-2026-4599..4602).
- `golang.org/x/net` upgraded `v0.21.0` → `v0.38.0` (fixes GO-2025-3595 XSS in HTML tokenizer).

### Fixed
- `go vet` warnings in `handler.go`: replaced `fmt.Errorf(err.Error())` with `return nil, err`.

---

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
