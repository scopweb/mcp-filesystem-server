# Changelog

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
- **Official MCP compatibility aliases** — 3 tool name aliases for clients trained on the official `modelcontextprotocol/servers` filesystem server: `read_text_file` → `read_file`, `search_files` → `search`, `directory_tree` → `tree`. Full parameter schemas, same handlers.
- **Recommended workflow in instructions** — `serverInstructions` now includes a 5-step workflow (navigate → locate → read range → edit → verify) sent during MCP initialize.
- **`edit_file` large-edit tip** — success message includes a TIP to use `compare_files` when edits affect >10 lines.
- **`/filesystem-light-tools` skill** — Claude Code skill (`.claude/skills/filesystem-light-tools/`) that instructs the AI to call the `help` tool at the start of a conversation. Solves Claude Desktop's lazy tool loading problem: without this, only ~5 of 18 tools are discovered per query. The skill ships with the repo so anyone who clones it gets it automatically.
- **`help` tool** — returns the full catalog of 18 tools with usage rules, parameters, and best practices. Description is keyword-rich so Claude Desktop's semantic search picks it up for virtually any filesystem query.
- **`server.WithInstructions()`** — sends tool catalog during MCP initialize handshake (spec 2025-11-25 compliant). Works with clients that support the `instructions` field; Claude Desktop currently ignores it.
- **`read_file` outline mode** — new `outline` boolean param. Returns a symbol index (functions, classes, types, interfaces with line numbers) using regex extraction for 14 languages: Go, JS, TS, Python, C#, Java, Rust, PHP, Ruby, Swift, Kotlin, C, C++, CSS.
- **`list_directory` depth** — new `depth` param (1-10). Enables multi-level recursive directory listing without using `tree`.
- **MCP logging capability** — `server.WithLogging()` enabled. Handler can send `notifications/message` log events to clients.
- **Progress notifications** — `batch_operations` and `read_multiple_files` send progress updates via `notifications/progress`.
- **Content annotations** — read tools annotate output with `audience: ["assistant"]`, write tools with `audience: ["user", "assistant"]`.
- **Tool title annotations** — all tools have `WithTitleAnnotation()` for better client UI display.
- **Roots change notifications** — handler listens for `notifications/roots/list_changed` and refreshes allowed directories mid-session.
- **Context cancellation** — all `filepath.Walk` operations, line scanners, and batch loops check `ctx.Done()` for cooperative cancellation.
- **Development audit logging** — new `--dev --log-dir <dir>` mode writes `operations.jsonl` and `metrics.json` for local observability without affecting normal stdio MCP transport.
- **Log inspection tools** — new `cmd/logview`, `cmd/logdashboard`, shared `internal/logview`, and `internal/dashboardapi` make it easier to inspect recent operations, errors, timings, request parameters, and internal sub-operations.
- **Optional proxy correlation** — new `cmd/proxy` can inject and persist correlated `request_id` traces in `proxy.jsonl` for end-to-end debugging.

### Testing
- **40 security and edge-case tests** — new `security_test.go` covering directory traversal (`../`), symlink attacks (outside dirs, nested chains), Windows paths (backslash, UNC), unicode/spaces/deep nesting, null/missing params, roots lifecycle (refresh, error, once-only), concurrent access (read, validatePath, refreshRoots), handleReadResource, copy/move destination validation, and edit_file dry_run/already_present/large-edit-tip.

### Changed
- **Tool descriptions rewritten for discoverability** — descriptions now include action keywords (TEXT REPLACEMENT, FIND AND REPLACE, PATCH), recommended workflows, and reduced noise in cross-references. Optimized for Claude Desktop's semantic tool search.
- **Tool execution errors** — converted `return nil, fmt.Errorf(...)` to `toolError(...)` / `toolErrorf(...)` across all handlers. Tool errors now use `IsError: true` (tool-level) instead of protocol-level errors, per MCP spec.
- **Resource capabilities** — changed from `WithResourceCapabilities(true, true)` to `(false, false)` to avoid overcommitting capabilities the server doesn't implement.
- **Output sanitization** — `sanitizeOutput` (1MB truncation, UTF-8 validation) integrated into `withNormalize` post-processing.
- **Rate limiting** — integrated into both `withNormalize` and `withAudit` middleware for consistent enforcement.
- Development logging now records both raw and normalized tool arguments, plus internal action summaries and sub-operation traces, so Claude/Desktop requests can be audited more precisely.

## [1.0.0] - 2026-03-23

### Added
- **`search` tool** — unified search replacing `search_files`, `smart_search`, and `find_duplicates`. Use `mode: "files"` (glob/substring), `mode: "content"` (regex + `context_lines`), or `mode: "duplicates"` (MD5 hash scan).
- **`write_file` backup** — new optional `create_backup` boolean param. Creates a `.backup` copy of the existing file before overwriting.
- **`write_file` chunked streaming** — new optional `chunk_index` / `total_chunks` params. Replaces `chunked_write`: send chunks sequentially; first chunk (index 0) truncates the file, subsequent chunks append.

### Removed (breaking)
- `search_files` → use `search` with `mode: "files"`
- `smart_search` → use `search` with `mode: "content"`
- `find_duplicates` → use `search` with `mode: "duplicates"`
- `write_file_safe` → use `write_file` with `create_backup: true`
- `chunked_write` → use `write_file` with `chunk_index` + `total_chunks`
- `split_file` — removed (no direct replacement)
- `join_files` — removed (no direct replacement)
- `analyze_file` — removed (was stub, never implemented)
- `performance_analysis` — removed (was stub, never implemented)
- `generate_report` — removed (was stub, never implemented)
- `smart_sync` — removed (was stub, never implemented)
- `assist_refactor` — removed (was stub, never implemented)

---

## [0.6.4] - 2026-03-23

### Fixed
- **`read_file` line range memory** — now streams with `bufio.Scanner` instead of `os.ReadFile`. Large files are no longer fully loaded into memory when only a line range is requested.
- **Dead code removed** — `handleAdvancedTextSearch`, `performAdvancedTextSearch`, and `maxInt`/`minInt` helpers were never registered or used; deleted.

---

## [0.6.3] - 2026-03-23

### Added
- **`smart_search` `context_lines`** — new optional number param. When `include_content` is true and `context_lines` is set, each content match includes N lines before and after (equivalent to `grep -C N`). Default: 0 (no context).

---

## [0.6.2] - 2026-03-23

### Added
- **`read_file` line range** — new optional `start_line` / `end_line` params (1-based, inclusive). Reads a section of a large file without loading it entirely, reducing context token usage. The 5 MB inline limit is bypassed when a line range is requested.

---

## [0.6.1] - 2026-03-23

### Changed
- Go upgraded to **1.26.1** (patch release with stdlib security fixes).
- `golang.org/x/net` upgraded `v0.21.0` → `v0.38.0` (fixes [GO-2025-3595](https://pkg.go.dev/vuln/GO-2025-3595) XSS in HTML tokenizer).

### Fixed
- `go vet` warnings in `handler.go`: replaced `fmt.Errorf(err.Error())` with direct `return nil, err`.

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
  - Parameter aliasing: `old_str`->`old_text`, `src`->`source`, `action`->`type`, `filepath`->`path`, etc.
  - Type coercion: string `"true"` -> bool, string `"3"` -> float64.
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
