---
name: filesystem-light-tools
description: Load and discover all available tools from the filesystem-light MCP server. Call this at the start of any conversation to ensure you know every tool available (read, write, edit, search, compare, batch, analyze, tree, media, chunks, duplicates, plan, and more). Essential because MCP clients may only load a subset of tools by default.
---

# Filesystem Light - Tool Discovery

## Purpose

MCP clients like Claude Desktop use **lazy tool loading** — they only discover a subset of tools based on semantic search of the current query. This means you may be missing powerful tools that could help the user.

**This skill ensures you know ALL 19 tools** from the filesystem-light MCP server.

## Instructions

When this skill is invoked:

1. **Call the `help` tool** from the filesystem-light MCP server (no arguments needed)
2. **Read the full response** — it contains the complete catalog of 19 tools with parameters, usage rules, and best practices
3. **Acknowledge to the user** what tools are available, organized by category
4. **Keep this knowledge active** for the rest of the conversation — use the right tool for each task

## When to use this skill

- At the **start of every conversation** involving filesystem operations
- When the user asks "what tools do you have?" or "what can you do with files?"
- When you suspect you're missing tools (e.g., you're about to use `write_file` to modify an existing file, but `edit_file` would be better)
- When the user explicitly invokes `/filesystem-light-tools`

## Expected behavior after loading

After calling `help`, you should be able to:
- Use `edit_file` instead of `write_file` for modifications
- Use `read_file` with `outline=true` for code navigation
- Use `search` with `mode=content` for grep-like searches
- Use `batch_operations` for bulk file operations
- Use `compare_files` for diffs
- Use `analyze_project` for project structure analysis
- Use `list_directory` with `depth=N` for recursive listings
- Use `tree` for hierarchical views
- Use `read_media_file` for images and binary files
