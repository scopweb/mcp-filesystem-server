---
name: filesystem-light-tools
description: Load and discover all available tools from the filesystem-light MCP server. Call this at the start of any conversation to ensure you know every tool available (read, write, edit, search, compare, batch, analyze, tree, media, chunks, duplicates, plan, and more). Essential because MCP clients may only load a subset of tools by default.
---

# Filesystem Light - Tool Discovery

## MANDATORY ACTION

Call `filesystem-light:help` as the **absolute first action** of this conversation, **BEFORE any tool_search or other tool call**. This call directly loads the complete tool catalog with no intermediate steps.

Do NOT use tool_search to look for "help" or "filesystem". Call directly:

```
filesystem-light:help()
```

No arguments. No prior search. No intermediaries.

## Why

MCP clients like Claude Desktop use **lazy tool loading** — they only discover a subset of tools via semantic search of the current query. If you do tool_search first, you load 4-5 partial tools and the full catalog gets buried.

Calling `help` directly gives you all **19 tools** at once.

## After calling help

Use the right tool for each task:

- **Modify existing files** → `edit_file` (NOT `write_file`)
- **Explore large files** → `read_file` with `outline=true`, then `start_line/end_line`
- **Search content** → `search` with `mode=content`
- **Bulk operations** → `batch_operations`
- **Compare files** → `compare_files`
- **Analyze project** → `analyze_project`
- **Recursive listing** → `list_directory` with `depth=N`
- **Visual tree** → `tree`
- **Images/binaries** → `read_media_file`
