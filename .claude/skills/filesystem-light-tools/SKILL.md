---
name: filesystem-light-tools
description: Load the full catalog of 18+3 filesystem-light tools in one call. Use at the start of any conversation in Claude Desktop where lazy tool loading would otherwise miss most tools.
---

# Filesystem Light - Tool Discovery

## How to activate

Call `filesystem-light:help()` directly — no arguments needed.

This loads the full catalog of **18 tools + 3 compatibility aliases** in one call, bypassing Claude Desktop's lazy loading (which only discovers 4-5 tools per semantic search).

> **Note:** If `filesystem-light:help` isn't visible yet, run `tool_search("filesystem help")` first to surface it, then call it.

## Auto-execute in Claude Desktop

To load tools automatically at the start of every conversation, add this to your **project system prompt**:

```
At the start of every conversation, call filesystem-light:help() to load the full tool catalog.
```

Memory entries alone won't trigger tool calls — the system prompt is the right place.

## Recommended workflow

1. **Navigate** → `read_file(outline=true)` — symbol map with line numbers
2. **Locate** → `search(mode=content, context_lines=3)` — grep-like
3. **Read range** → `read_file(start_line, end_line)` — precise context
4. **Edit** → `edit_file` (NOT `write_file`) — supports `dry_run` preview
5. **Verify** → `compare_files` after edits >10 lines

## Tool reference

| Task | Tool |
|------|------|
| Modify existing file | `edit_file` — never `write_file` |
| Create new file | `write_file` |
| Navigate large file | `read_file(outline=true)` then range |
| Search content (grep) | `search(mode=content)` |
| Search by filename | `search(mode=files)` |
| Find duplicates | `search(mode=duplicates)` |
| Bulk ops | `batch_operations` |
| Diff two files | `compare_files` |
| Project overview | `analyze_project` |
| Recursive listing | `list_directory(depth=N)` |
| Visual tree | `tree` |
| Images / binaries | `read_media_file` |
| Large file writes | `write_file(chunk_index, total_chunks)` |

## Compatibility aliases (official MCP filesystem server)

- `read_text_file` → `read_file`
- `search_files` → `search`
- `directory_tree` → `tree`
