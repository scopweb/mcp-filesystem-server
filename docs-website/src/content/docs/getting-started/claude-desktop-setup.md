---
title: Claude Desktop Setup
description: Configure MCP Filesystem Server for Claude Desktop.
---

## Configuration

Add to your `claude_desktop_config.json`:

### Binary (in PATH)

```json title="claude_desktop_config.json"
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem-go-light",
      "args": ["/path/to/allowed/dir", "/another/dir"]
    }
  }
}
```

### Windows — direct path to .exe

```json title="claude_desktop_config.json"
{
  "mcpServers": {
    "filesystem": {
      "command": "C:\\path\\to\\mcp-filesystem-go-light.exe",
      "args": ["C:\\Users\\you\\projects", "C:\\another\\dir"]
    }
  }
}
```

### Docker

```json title="claude_desktop_config.json"
{
  "mcpServers": {
    "filesystem": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "/your/path:/data",
        "ghcr.io/scopweb/mcp-filesystem-go-light:latest",
        "/data"
      ]
    }
  }
}
```

## How it works

1. Claude Desktop starts the server as a subprocess via stdio.
2. The server receives MCP tool calls and executes filesystem operations.
3. All paths are validated against the allowed directories passed as arguments.
4. The normalizer layer corrects parameter mismatches before they reach the handler.

## Development logging

If you want local audit logs while testing with Claude Desktop, place flags before the allowed directories:

```json title="claude_desktop_config.json"
{
  "mcpServers": {
    "filesystem": {
      "command": "C:\\path\\to\\mcp-filesystem-go-light.exe",
      "args": [
        "--dev",
        "--log-dir",
        "C:\\mcp-logs",
        "C:\\Users\\you\\projects"
      ]
    }
  }
}
```

- `--dev --log-dir C:\\mcp-logs` writes `operations.jsonl` and `metrics.json`.
- Allowed directories must come after the flags because the server reads them from positional arguments.
- To inspect logs locally, run `go run ./cmd/logdashboard --log-dir C:\\mcp-logs --addr :8091` and open `http://127.0.0.1:8091`.

:::caution
Only allow directories you trust. The server has full read/write access within allowed paths.
:::

## Config file location

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |
