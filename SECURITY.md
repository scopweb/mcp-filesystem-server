# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x.x   | Yes       |
| < 1.0   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public issue.
2. Email **scopweb@gmail.com** with:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
3. You will receive a response within 72 hours.

## Security Model

- All file operations are restricted to explicitly allowed directories passed at startup.
- Symlinks are resolved and re-validated against the allow-list.
- Path traversal attempts (`../`) are blocked.
- No network access beyond stdio transport.

## Safe Configuration Recommendations

**Only allow the exact directories you intend to edit.** Do not add broad paths like `C:\`, `/home`, or your entire project root unless strictly necessary.

Recommended setup:
- **Project paths** — only the specific folders you want the AI to read or modify.
- **Temporary sandbox** — a dedicated folder (e.g., `C:\MCPs\sandbox` or `/tmp/mcp-sandbox`) for experimental operations, file generation, and disposable tests. Contents can be discarded at any time without risk.

Example (Claude Desktop `claude_desktop_config.json`):
```json
"args": [
  "C:\\Projects\\my-project\\src",
  "C:\\MCPs\\sandbox"
]
```

> See [DISCLAIMER.md](DISCLAIMER.md) for full liability and usage terms.
