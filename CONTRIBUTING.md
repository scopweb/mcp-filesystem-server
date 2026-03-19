# Contributing

## How to Contribute

1. Fork the repository
2. Create a branch: `git checkout -b feature/your-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Commit using [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`
6. Open a Pull Request against `main`

## Development

```bash
# Clone
git clone https://github.com/scopweb/mcp-filesystem-server.git
cd mcp-filesystem-server

# Test
go test ./filesystemserver -v

# Build
go build -o mcp-filesystem-server .
```

## Guidelines

- Keep changes focused and minimal.
- Add tests for new functionality.
- Do not add dependencies without discussion.
- Follow existing code style.

## Issues

Use GitHub Issues for bug reports and feature requests. Include steps to reproduce for bugs.
