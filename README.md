# MCP Filesystem Server - Enhanced for Claude Desktop

Secure filesystem access via Model Context Protocol with advanced analysis capabilities optimized for Claude Desktop.

## Core Features

- **File Operations**: Read, write, edit, copy, move, delete files and directories
- **Advanced Search**: Smart search with regex, content matching, and file type filtering
- **Code Analysis**: Project structure analysis, dependency mapping, complexity metrics
- **Batch Operations**: Execute multiple file operations efficiently
- **File Comparison**: Advanced diff generation and similarity analysis
- **Duplicate Detection**: Find duplicate files by content hash
- **Project Management**: Comprehensive reporting and maintenance tools

## Key Tools

### File Operations
- `read_file`, `write_file`, `edit_file` - Basic file operations
- `read_multiple_files` - Batch file reading
- `copy_file`, `move_file`, `delete_file` - File management
- `list_directory`, `create_directory`, `tree` - Directory operations

### Analysis & Search
- `analyze_project` - Comprehensive project structure analysis
- `analyze_file` - Deep file analysis with complexity metrics
- `smart_search` - Intelligent search with content matching
- `find_duplicates` - Duplicate file detection
- `compare_files` - Advanced file comparison

### Advanced Operations
- `batch_operations` - Execute multiple operations in one call
- `generate_report` - Create project reports in JSON/HTML/Markdown
- `performance_analysis` - File system performance metrics
- `assist_refactor` - Code refactoring assistance

### Chunked Operations 🚀
- `chunked_write` - Write large files in chunks (avoid memory limits)
- `split_file` - Split large files into smaller chunks
- `join_files` - Join multiple file chunks into single file
- `write_file_safe` - Atomic file write with optional backup

## Installation

```bash
go install github.com/scopweb/mcp-filesystem-server@latest
```

## Usage

### Standalone Server
```bash
mcp-filesystem-server /path/to/allowed/directory
```

### MCP Configuration
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem-server",
      "args": ["/path/to/allowed/directory"]
    }
  }
}
```

### Docker
```bash
docker run -i --rm ghcr.io/scopweb/mcp-filesystem-server:latest /path/to/directory
```

## Security

- Path validation prevents directory traversal attacks
- Symlink resolution with security checks
- Access restricted to specified directories only

## Testing

Run comprehensive test suite:
```bash
# Windows
run_tests.cmd

# Unix/Linux/Mac
go test ./filesystemserver -v
```

Test coverage: 34/34 functions (100%)

## License

See [LICENSE](LICENSE) file for details.