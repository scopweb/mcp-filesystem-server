---
title: Tools Reference
description: All 18 tools available in MCP Filesystem Server.
---

## File Operations

### `read_file`

Read the complete contents of a file, or a specific line range.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to the file |
| `start_line` | number | No | First line to read (1-based, inclusive) |
| `end_line` | number | No | Last line to read (1-based, inclusive) |

:::tip
When `start_line`/`end_line` are set, the 5 MB inline limit is bypassed — ideal for reading a function or section from a large file without loading it entirely.
:::

### `write_file`

Create or overwrite a file with new content. Supports optional backup and chunked streaming for large files.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path where to write |
| `content` | string | Yes | Content to write |
| `create_backup` | boolean | No | Create a `.backup` copy before overwriting (default: false) |
| `chunk_index` | number | No | 0-based chunk index for streaming large files. Omit for single write. |
| `total_chunks` | number | No | Total chunks expected. Required when `chunk_index` is set. |

### `edit_file`

Replace specific text in a file without rewriting the whole file. Supports intelligent multi-phase matching, literal escape recovery, idempotent edits, and non-blocking no-match.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to the file |
| `old_text` | string | Yes | Text to replace |
| `new_text` | string | Yes | Replacement text |
| `dry_run` | boolean | No | Preview diff without writing (default: false) |

:::tip
The normalizer accepts `old_str`, `oldText`, and other aliases for `old_text`. You don't need to worry about the exact parameter name.
:::

### `read_media_file`

Read a media file (image, audio, or other binary) and return it base64-encoded with its MIME type. Returns `ImageContent` for images so clients can render them inline.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to the media file |

### `copy_file`

Copy a file or directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `source` | string | Yes | Source path |
| `destination` | string | Yes | Destination path |

### `move_file`

Move or rename a file or directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `source` | string | Yes | Source path |
| `destination` | string | Yes | Destination path |

### `delete_file`

Delete a file or directory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to delete |
| `recursive` | boolean | No | Recursively delete directories (default: false) |

### `read_multiple_files`

Read multiple files in a single call.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `paths` | array | Yes | List of file paths |

### `get_file_info`

Get file or directory metadata (size, permissions, timestamps).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to inspect |

## Directory Operations

### `list_directory`

List the immediate contents of a single directory (one level, no recursion). Prefer this over `tree` or `search_files` when you only need what is directly inside a folder — faster and token-efficient.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory path |

### `create_directory`

Create a directory (including parent directories).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory path |

### `tree`

Hierarchical JSON representation of a directory structure.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory to traverse |
| `depth` | number | No | Max depth (default: 3) |
| `follow_symlinks` | boolean | No | Follow symlinks (default: false) |

### `list_allowed_directories`

Returns the list of directories the server is allowed to access. No parameters.

## Search

### `search`

Unified file search with three modes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Starting directory |
| `mode` | string | No | `"files"` (default), `"content"`, or `"duplicates"` |
| `pattern` | string | Conditional | Filename glob or content regex. Required for `files` and `content` modes. |
| `include_content` | boolean | No | (content mode) Search inside files. Default: true. |
| `file_types` | array | No | (content mode) Filter by extension, e.g. `[".go", ".js"]` |
| `context_lines` | number | No | (content mode) Lines before/after each match, like `grep -C N`. Default: 0. |

:::tip
- `mode: "files"` with a glob pattern like `*.go` replaces the old `search_files` tool.
- `mode: "content"` with `context_lines: 3` shows surrounding code — avoids a separate `read_file` call.
- `mode: "duplicates"` scans by MD5 hash — replaces the old `find_duplicates` tool.
:::

## Analysis

### `analyze_project`

Comprehensive project analysis — language detection, structure, metrics.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Project root directory |

### `compare_files`

Diff-style comparison between two files.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file1` | string | Yes | First file |
| `file2` | string | Yes | Second file |
| `format` | string | No | `unified`, `context`, or `side-by-side` (default: unified) |

## Batch & Advanced

### `batch_operations`

Execute multiple file operations in one call.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operations` | array | Yes | Array of `{type, from, to}` objects |

Supported types: `rename`, `delete`, `copy`, `cp`, `rm`, `remove`.

Field aliases: `source`/`src`/`from`/`path`/`file` for source, `destination`/`dest`/`dst`/`to`/`target` for destination, `action`/`type` for operation type.

### `plan_task`

Create step-by-step execution plans for complex operations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | string | Yes | Task description |
| `target_files` | array | No | Files to modify |
| `workspace` | string | No | Workspace path |
