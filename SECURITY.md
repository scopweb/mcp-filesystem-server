# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.5.x   | Yes       |
| < 0.5   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public issue.
2. Email **security@scopweb.dev** with:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
3. You will receive a response within 72 hours.

## Security Model

- All file operations are restricted to explicitly allowed directories passed at startup.
- Symlinks are resolved and re-validated against the allow-list.
- Path traversal attempts (`../`) are blocked.
- No network access beyond stdio transport.
