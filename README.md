# MCP Filesystem Server

Lightweight edition of [mcp-filesystem-go-ultra](https://github.com/scopweb/mcp-filesystem-go-ultra). Same normalizer intelligence, fewer tools.

MCP server that provides filesystem access to Claude Desktop and other LLM clients.

## Install

```bash
go install github.com/scopweb/mcp-filesystem-server@latest
```

## Configure

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem-server",
      "args": ["/path/to/allowed/dir", "/another/dir"]
    }
  }
}
```

Docker:

```bash
docker pull ghcr.io/scopweb/mcp-filesystem-server:latest
```

## Tools

**File operations**: `read_file`, `write_file`, `edit_file` (supports `dry_run`), `copy_file`, `move_file`, `delete_file`, `list_directory`, `create_directory`, `tree`, `read_multiple_files`, `get_file_info`, `list_allowed_directories`

**Analysis**: `analyze_file`, `analyze_project`, `smart_search`, `find_duplicates`, `compare_files`, `performance_analysis`, `generate_report`

**Batch & advanced**: `batch_operations`, `assist_refactor`, `plan_task`, `smart_sync`

**Large files**: `chunked_write`, `split_file`, `join_files`, `write_file_safe`

**Media**: `read_media_file` — reads images as `ImageContent`, other binary files as base64 text with MIME type

## Normalizer

Ported from ultra. Runs before every handler. No new tools added — purely internal.

- **Parameter aliases**: `old_str` -> `old_text`, `src` -> `source`, `action` -> `type`, `filepath` -> `path`, etc. 50+ rules.
- **Type coercion**: `"true"` -> `true`, `"3"` -> `3.0`. Covers all boolean and numeric params.
- **JSON accept-both**: array params accept native JSON or a JSON string.
- **Literal escape fix**: `\n` as two chars -> real newline. Handles Claude Desktop quirk.
- **Idempotent edits**: if `new_text` already present and `old_text` absent, skips without error.
- **Non-blocking no-match**: `edit_file` returns 0 replacements instead of failing.
- **Flexible batch fields**: `from`/`source`/`src`, `to`/`destination`/`dest`/`target`, `type`/`action`.

## Security

All paths validated against an allow-list. Symlinks resolved and re-checked. Path traversal blocked.

## vs ultra

| | This server | [ultra](https://github.com/scopweb/mcp-filesystem-go-ultra) |
|---|---|---|
| Tools | 29 | 16 (consolidated) |
| Normalizer | Yes (ported) | Yes (original) |
| Backup/restore | No | Yes |
| Pipeline executor | No | Yes |
| Audit trail | No | Yes |
| Regex edit mode | No | Yes |
| WSL integration | No | Yes |

Choose this for simplicity. Choose ultra for full capability.

## Test

```bash
go test ./filesystemserver -v
```

## License

[MIT](LICENSE)
