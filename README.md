# MCP Filesystem Server

Lightweight edition of [mcp-filesystem-go-ultra](https://github.com/scopweb/mcp-filesystem-go-ultra). Same normalizer intelligence, fewer tools.

MCP server that provides filesystem access to Claude Desktop and other LLM clients.

## Install

```bash
go install github.com/scopweb/mcp-filesystem-go-light@latest
```

## Configure

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem-go-light",
      "args": ["/path/to/allowed/dir", "/another/dir"]
    }
  }
}
```

On Windows, you can point directly to the `.exe`:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "C:\\path\\to\\mcp-filesystem-go-light.exe",
      "args": ["C:\\Users\\you\\projects", "C:\\another\\dir"]
    }
  }
}
```

Docker:

```bash
docker pull ghcr.io/scopweb/mcp-filesystem-go-light:latest
```

## Tool Discovery

Claude Desktop uses **lazy tool loading** — it only discovers ~5 tools per query via semantic search, missing most of the 19 available tools.

Three layers solve this:

| Layer | How it works | Client support |
|-------|-------------|----------------|
| **`/filesystem-light-tools` skill** | Claude Code skill that calls `help` on conversation start | Claude Code |
| **`help` tool** | Keyword-rich description; returns full 19-tool catalog | Any MCP client |
| **`server.WithInstructions()`** | Sends catalog during MCP initialize handshake | Spec-compliant clients |

### Using the skill

The skill ships in `.claude/skills/filesystem-light-tools/`. In Claude Code or Claude Desktop, invoke:

```
/filesystem-light-tools
```

This calls the `help` tool and loads the full catalog. You can also add this to your project instructions:

```
Al inicio de cada conversación, llama a la herramienta help del MCP filesystem-light
```

## Tools

**File operations**: `read_file` (supports `start_line`/`end_line`, `outline`), `write_file` (supports `create_backup`, `chunk_index`/`total_chunks`), `edit_file` (supports `dry_run`), `copy_file`, `move_file`, `delete_file`, `list_directory` (supports `depth`), `create_directory`, `tree`, `read_multiple_files`, `get_file_info`, `list_allowed_directories`

**Search**: `search` — unified: `mode: "files"` (glob/substring), `mode: "content"` (regex + `context_lines`), `mode: "duplicates"` (MD5 hash scan)

**Analysis**: `analyze_project`, `compare_files`

**Batch & advanced**: `batch_operations`, `plan_task`

**Media**: `read_media_file` — reads images as `ImageContent`, other binary files as base64 text with MIME type

**System**: `help` — full tool catalog with usage rules and best practices

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
| Tools | 19 | 16 (consolidated) |
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
