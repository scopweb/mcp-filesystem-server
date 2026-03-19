---
title: Tools Reference
description: All 27 tools available in MCP Filesystem Server.
---

## File Operations

### `read_file`

Read the complete contents of a file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to the file |

### `write_file`

Create or overwrite a file with new content.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path where to write |
| `content` | string | Yes | Content to write |

### `edit_file`

Replace specific text in a file without rewriting the whole file. Supports intelligent multi-phase matching, literal escape recovery, idempotent edits, and non-blocking no-match.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to the file |
| `old_text` | string | Yes | Text to replace |
| `new_text` | string | Yes | Replacement text |

:::tip
The normalizer accepts `old_str`, `oldText`, and other aliases for `old_text`. You don't need to worry about the exact parameter name.
:::

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

List all files and directories in a path.

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

### `search_files`

Recursively search for files matching a name pattern.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Starting directory |
| `pattern` | string | Yes | Name pattern to match |

### `smart_search`

Intelligent search with regex, content matching, and file type filtering.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Starting directory |
| `pattern` | string | Yes | Search pattern (regex supported) |
| `include_content` | boolean | No | Search inside files (default: false) |
| `file_types` | array | No | Filter by extension (e.g., `[".js", ".go"]`) |

### `find_duplicates`

Find duplicate files by content hash.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Directory to scan |

## Analysis

### `analyze_file`

Deep analysis of a single file — complexity, dependencies, metadata.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File to analyze |

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

### `performance_analysis`

Benchmark filesystem performance on a path.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to analyze |
| `operation` | string | No | `read`, `write`, `list`, or all (default: all) |

### `generate_report`

Generate reports in JSON, HTML, or Markdown.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | Path to analyze |
| `format` | string | No | `json`, `html`, `markdown` (default: json) |
| `output` | string | No | Output file path |
| `sections` | array | No | Sections: `overview`, `files`, `quality`, `dependencies`, `security` |

## Batch & Advanced

### `batch_operations`

Execute multiple file operations in one call.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operations` | array | Yes | Array of `{type, from, to}` objects |

Supported types: `rename`, `delete`, `copy`, `cp`, `rm`, `remove`.

Field aliases: `source`/`src`/`from`/`path`/`file` for source, `destination`/`dest`/`dst`/`to`/`target` for destination, `action`/`type` for operation type.

### `smart_sync`

Directory synchronization with conflict detection.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `source` | string | Yes | Source directory |
| `target` | string | Yes | Target directory |
| `mode` | string | No | `preview`, `merge`, `overwrite` (default: preview) |
| `exclude_patterns` | array | No | Patterns to exclude |

### `assist_refactor`

Analyze dependencies and suggest safe refactoring changes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File or directory |
| `operation` | string | Yes | `rename`, `extract`, `inline`, `move` |
| `target` | string | No | Target name |
| `options` | object | No | Additional options |

### `plan_task`

Create step-by-step execution plans for complex operations.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `description` | string | Yes | Task description |
| `target_files` | array | No | Files to modify |
| `workspace` | string | No | Workspace path |

## Large File Operations

### `chunked_write`

Write large files in chunks to avoid memory limits.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File path |
| `content` | string | Yes | Content chunk |
| `chunk_index` | number | Yes | Chunk index (0-based) |
| `total_chunks` | number | Yes | Total chunks |

### `split_file`

Split a large file into smaller chunks.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File to split |
| `chunk_size` | number | No | Chunk size in bytes (default: 1MB) |

### `join_files`

Join file chunks into a single file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_path` | string | Yes | Output file path |
| `source_files` | array | Yes | List of chunk files |

### `write_file_safe`

Atomic file write with optional backup.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | Yes | File path |
| `content` | string | Yes | Content to write |
| `create_backup` | boolean | No | Create backup first (default: false) |
