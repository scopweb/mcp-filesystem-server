---
title: Claude Desktop Setup
description: Configure MCP Filesystem Server for Claude Desktop.
---

## Configuration

Add to your `claude_desktop_config.json`:

### Binary

```json title="claude_desktop_config.json"
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem-server",
      "args": ["/path/to/allowed/dir", "/another/dir"]
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
        "ghcr.io/scopweb/mcp-filesystem-server:latest",
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

:::caution
Only allow directories you trust. The server has full read/write access within allowed paths.
:::

## Config file location

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |
