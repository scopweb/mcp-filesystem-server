---
title: Request Normalizer
description: How the normalizer layer prevents parameter errors.
---

The normalizer is an internal layer ported from [mcp-filesystem-go-ultra](https://github.com/scopweb/mcp-filesystem-go-ultra). It runs before every tool handler and fixes common parameter issues without adding any new tools.

## Why it exists

LLM clients (Claude Desktop, etc.) sometimes send parameters with slightly wrong names, wrong types, or inconsistent JSON encoding. Instead of returning errors, the normalizer silently corrects these before the handler sees them.

## Parameter Aliasing

50+ rules map common parameter name variants to the canonical name.

| Canonical | Accepted aliases |
|-----------|-----------------|
| `path` | `filepath`, `file_path`, `filename`, `file` |
| `old_text` | `old_str`, `oldText`, `old_string`, `search` |
| `new_text` | `new_str`, `newText`, `new_string`, `replace` |
| `source` | `src`, `from`, `source_path` |
| `destination` | `dest`, `dst`, `to`, `target`, `destination_path` |
| `content` | `text`, `data`, `body` |
| `pattern` | `query`, `search`, `regex` |
| `type` | `action`, `operation`, `op` |
| `recursive` | `recurse` |
| `include_content` | `search_content`, `content_search` |
| `file_types` | `extensions`, `ext`, `filetypes` |
| `chunk_index` | `index`, `chunk_number` |
| `total_chunks` | `chunks`, `num_chunks`, `total` |
| `chunk_size` | `size`, `bytes` |
| `create_backup` | `backup` |
| `follow_symlinks` | `symlinks`, `follow_links` |
| `exclude_patterns` | `exclude`, `ignore` |

## Type Coercion

String values are converted to the expected Go type:

| From | To | Example |
|------|-----|---------|
| `"true"`, `"false"` | `bool` | `"true"` → `true` |
| `"yes"`, `"no"` | `bool` | `"yes"` → `true` |
| `"1"`, `"0"` (for booleans) | `bool` | `"1"` → `true` |
| Numeric strings | `float64` | `"3"` → `3.0` |

## JSON Accept-Both

Array parameters accept either native JSON arrays or JSON-encoded strings:

```json
// Both work:
{ "paths": ["/a.txt", "/b.txt"] }
{ "paths": "[\"/a.txt\", \"/b.txt\"]" }
```

## Literal Escape Recovery

Claude Desktop sometimes sends `\n` as two literal characters (`\` + `n`) instead of a real newline. The normalizer detects and fixes this in `old_text` and `new_text` parameters.

## Idempotent Edits

If `edit_file` is called where `new_text` is already present in the file and `old_text` is absent, the edit is skipped gracefully instead of returning an error. This prevents retry loops.

## Non-blocking No-match

When `edit_file` can't find `old_text` in the file, it returns 0 replacements with a message instead of failing. The caller is told to re-read the file for current content.

## Flexible Batch Fields

`batch_operations` accepts multiple field names for the same concept:

| Concept | Accepted fields |
|---------|----------------|
| Source | `from`, `source`, `src`, `path`, `file` |
| Destination | `to`, `destination`, `dest`, `dst`, `target` |
| Operation | `type`, `action` |

Type aliases: `cp` → `copy`, `rm` / `remove` → `delete`.
