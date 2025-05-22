# MCP Filesystem Server - Enhanced for Claude Desktop

This MCP server provides secure access to the local filesystem via the Model Context Protocol (MCP). This version has been **optimized and tested specifically for Claude Desktop** with advanced analysis capabilities.

## 🚀 Enhanced Features for Claude Desktop

This enhanced version includes powerful tools designed specifically for Claude's code analysis and development workflows:

### 🔍 Advanced Analysis Tools
- **analyze_file** - Deep file analysis with complexity metrics, dependencies, and metadata
- **analyze_project** - Comprehensive project structure analysis with language detection
- **code_quality_check** - Code quality analysis including complexity and maintainability metrics

### 🔎 Intelligent Search & Discovery
- **smart_search** - Regex-enabled search with content matching and file type filtering
- **advanced_text_search** - Precise text search with context and matching options
- **find_duplicates** - Detect duplicate files by content hash for cleanup optimization

### 📊 Dependency & Relationship Analysis
- **analyze_dependencies** - Code dependency analysis with import/export mapping
- **compare_files** - Advanced file comparison with diff generation and similarity analysis

### 🛠️ Development Workflow Tools
- **validate_syntax** - Multi-language syntax validation
- **watch_file** - Real-time file monitoring for development tracking
- **assist_refactor** - Intelligent refactoring assistance with dependency analysis

### 🧹 Project Maintenance
- **smart_cleanup** - Intelligent cleanup of temporary files and build artifacts
- **smart_sync** - Advanced file synchronization with conflict detection
- **batch_operations** - Execute multiple file operations efficiently

### 📈 Performance & Reporting
- **performance_analysis** - File system performance metrics and bottleneck identification
- **generate_report** - Comprehensive reports in JSON, HTML, or Markdown formats
- **directory_stats** - Detailed directory statistics with language distribution

### 🔧 Advanced File Operations
- **create_from_template** - Template-based file generation with variable substitution
- **convert_file** - File format conversion with encoding support
- **extract_metadata** - Comprehensive metadata extraction including EXIF and code metrics
- **generate_checksum** - Multiple hash algorithm support for integrity verification

## Components

### Tools

#### File Operations

- **read_file**
  - Read the complete contents of a file from the file system
  - Parameters: `path` (required): Path to the file to read

- **read_multiple_files**
  - Read the contents of multiple files in a single operation
  - Parameters: `paths` (required): List of file paths to read

- **write_file**
  - Create a new file or overwrite an existing file with new content
  - Parameters: `path` (required): Path where to write the file, `content` (required): Content to write to the file

- **edit_file**
  - Modify file content by replacing specific text without rewriting the entire file
  - Parameters: `path` (required): Path to the file to edit, `old_text` (required): Text to be replaced, `new_text` (required): New text to replace with

- **copy_file**
  - Copy files and directories
  - Parameters: `source` (required): Source path of the file or directory, `destination` (required): Destination path

- **move_file**
  - Move or rename files and directories
  - Parameters: `source` (required): Source path of the file or directory, `destination` (required): Destination path

- **delete_file**
  - Delete a file or directory from the file system
  - Parameters: `path` (required): Path to the file or directory to delete, `recursive` (optional): Whether to recursively delete directories (default: false)

#### Directory Operations

- **list_directory**
  - Get a detailed listing of all files and directories in a specified path
  - Parameters: `path` (required): Path of the directory to list

- **create_directory**
  - Create a new directory or ensure a directory exists
  - Parameters: `path` (required): Path of the directory to create

- **tree**
  - Returns a hierarchical JSON representation of a directory structure
  - Parameters: `path` (required): Path of the directory to traverse, `depth` (optional): Maximum depth to traverse (default: 3), `follow_symlinks` (optional): Whether to follow symbolic links (default: false)

- **directory_stats**
  - Calculate comprehensive directory statistics including file counts, sizes, and language distribution
  - Parameters: `path` (required): Directory to analyze

#### Search and Information

- **search_files**
  - Recursively search for files and directories matching a pattern
  - Parameters: `path` (required): Starting path for the search, `pattern` (required): Search pattern to match against file names

- **smart_search**
  - Intelligent search with regex support, content matching, and file type filtering - perfect for Claude's code analysis
  - Parameters: `path` (required): Starting path for the search, `pattern` (required): Search pattern (supports regex), `include_content` (optional): Search within file contents (default: false), `file_types` (optional): Filter by file extensions

- **advanced_text_search**
  - Advanced text search with regex, context, and precise matching options - perfect for Claude's code analysis needs
  - Parameters: `path` (required): Directory to search in, `pattern` (required): Search pattern (regex supported), `case_sensitive` (optional): Case sensitive search (default: false), `whole_word` (optional): Match whole words only (default: false), `include_context` (optional): Include surrounding lines for context (default: false), `context_lines` (optional): Number of context lines to include (default: 3)

- **get_file_info**
  - Retrieve detailed metadata about a file or directory
  - Parameters: `path` (required): Path to the file or directory

- **list_allowed_directories**
  - Returns the list of directories that this server is allowed to access
  - Parameters: None

#### Advanced Analysis Tools

- **analyze_file**
  - Perform deep analysis of a file including complexity metrics, dependencies, and metadata optimized for Claude Desktop
  - Parameters: `path` (required): Path to the file to analyze

- **analyze_project**
  - Comprehensive project structure analysis with language detection and metrics - gives Claude full project context
  - Parameters: `path` (required): Project root directory

- **analyze_dependencies**
  - Analyze code dependencies and imports - gives Claude full context of project relationships
  - Parameters: `path` (required): Path to file or project root, `language` (optional): Programming language (auto-detect if not specified), `recursive` (optional): Analyze dependencies recursively (default: true)

- **code_quality_check**
  - Comprehensive code quality analysis including complexity, maintainability, and best practices compliance
  - Parameters: `path` (required): File or directory to analyze, `language` (optional): Programming language (auto-detect if not specified), `metrics` (optional): Specific metrics to calculate

#### File Management and Utilities

- **find_duplicates**
  - Find duplicate files by content hash - useful for cleanup and optimization tasks Claude might suggest
  - Parameters: `path` (required): Directory to scan for duplicates

- **batch_operations**
  - Execute multiple file operations in a single call - efficient for Claude's bulk suggestions
  - Parameters: `operations` (required): Array of operations to execute: [{type: 'rename|delete|copy', from: 'path', to: 'path'}]

- **compare_files**
  - Advanced file comparison with diff generation and similarity analysis for Claude's code review tasks
  - Parameters: `file1` (required): First file to compare, `file2` (required): Second file to compare, `format` (optional): Output format: 'unified', 'context', 'side-by-side' (default: unified)

- **watch_file**
  - Monitor file changes and return modification events - helps Claude track development progress
  - Parameters: `path` (required): File or directory to monitor, `timeout` (optional): Maximum time to watch in seconds (default: 30)

#### Validation and Security

- **validate_syntax**
  - Validate syntax for various programming languages - essential for Claude's code generation validation
  - Parameters: `path` (required): Path to file to validate, `language` (optional): Language to validate (auto-detect if not specified)

- **extract_metadata**
  - Extract comprehensive metadata from files including EXIF, document properties, and code metrics
  - Parameters: `path` (required): Path to file, `type` (optional): Metadata type: 'exif', 'document', 'code', 'all' (default: all)

- **generate_checksum**
  - Generate various checksums for file integrity verification - useful for Claude's security recommendations
  - Parameters: `path` (required): Path to file or directory, `algorithms` (optional): Hash algorithms: ['md5', 'sha1', 'sha256', 'sha512']

#### Maintenance and Cleanup

- **smart_cleanup**
  - Intelligent cleanup of temporary files, logs, and build artifacts - helps Claude suggest project maintenance
  - Parameters: `path` (required): Directory to clean, `patterns` (optional): File patterns to clean (default: common temp files), `dry_run` (optional): Show what would be deleted without actually deleting (default: true)

- **convert_file**
  - Convert files between different formats - supports text encodings, line endings, and basic format conversions
  - Parameters: `source` (required): Source file path, `target` (required): Target file path, `from_format` (optional): Source format, `to_format` (optional): Target format, `encoding` (optional): Target encoding (utf-8, ascii, etc.)

#### Templates and Code Generation

- **create_from_template**
  - Create files from templates with variable substitution - perfect for Claude's code generation workflows
  - Parameters: `template_path` (required): Path to template file, `output_path` (required): Output file path, `variables` (optional): Template variables as key-value pairs

#### Advanced Features

- **performance_analysis**
  - Analyze file system performance metrics and identify bottlenecks
  - Parameters: `path` (required): Path to analyze, `operation` (optional): Operation to benchmark: 'read', 'write', 'list' (default: all)

- **generate_report**
  - Generate comprehensive reports in various formats (JSON, HTML, Markdown) for Claude's analysis
  - Parameters: `path` (required): Path to analyze for report, `format` (optional): Report format: 'json', 'html', 'markdown' (default: json), `output` (optional): Output file path, `sections` (optional): Report sections to include

- **smart_sync**
  - Intelligent file synchronization with conflict detection and resolution suggestions
  - Parameters: `source` (required): Source directory, `target` (required): Target directory, `mode` (optional): Sync mode: 'preview', 'merge', 'overwrite' (default: preview), `exclude_patterns` (optional): Patterns to exclude from sync

- **assist_refactor**
  - Assist with code refactoring by analyzing dependencies and suggesting safe changes
  - Parameters: `path` (required): File or directory to refactor, `operation` (required): Refactor operation: 'rename', 'extract', 'inline', 'move', `target` (optional): Target for refactoring, `options` (optional): Refactoring options

## Features

- Secure access to specified directories
- Path validation to prevent directory traversal attacks
- Symlink resolution with security checks
- MIME type detection
- Support for text, binary, and image files
- Size limits for inline content and base64 encoding
- **Advanced code analysis and project structure understanding**
- **Intelligent file operations optimized for AI assistants**
- Comprehensive metadata extraction and file validation
- Project management and maintenance tools
- **Optimized for Claude Desktop workflows**

## Getting Started

### Installation

#### Using Go Install

```bash
go install github.com/TU_USUARIO/mcp-filesystem-server@latest
```

### Usage

#### As a standalone server

Start the MCP server with allowed directories:

```bash
mcp-filesystem-server /path/to/allowed/directory [/another/allowed/directory ...]
```

#### As a library in your Go project

```go
package main

import (
	"log"
	"os"

	"github.com/scopweb/mcp-filesystem-server/filesystemserver"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new filesystem server with allowed directories
	allowedDirs := []string{"/path/to/allowed/directory", "/another/allowed/directory"}
	fs, err := filesystemserver.NewFilesystemServer(allowedDirs)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Serve requests
	if err := server.ServeStdio(fs); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

### Usage with Model Context Protocol

To integrate this server with apps that support MCP:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-filesystem-server",
      "args": ["/path/to/allowed/directory", "/another/allowed/directory"]
    }
  }
}
```

### Docker

#### Running with Docker

You can run the Filesystem MCP server using Docker:

```bash
docker run -i --rm ghcr.io/TU_USUARIO/mcp-filesystem-server:latest /path/to/allowed/directory
```

#### Docker Configuration with MCP

To integrate the Docker image with apps that support MCP:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "ghcr.io/TU_USUARIO/mcp-filesystem-server:latest",
        "/path/to/allowed/directory"
      ]
    }
  }
}
```

## Advanced Usage Examples

### Code Analysis Workflow

The server provides powerful tools for code analysis that work seamlessly with AI assistants:

```bash
# Analyze entire project structure
mcp-filesystem-server analyze_project /path/to/project

# Deep file analysis with complexity metrics
mcp-filesystem-server analyze_file /path/to/source.go

# Smart search with content matching
mcp-filesystem-server smart_search /path/to/project "TODO" --include-content true

# Validate syntax across multiple files
mcp-filesystem-server validate_syntax /path/to/source.py --language python
```

### Project Maintenance

```bash
# Find duplicate files for cleanup
mcp-filesystem-server find_duplicates /path/to/project

# Intelligent cleanup of temporary files
mcp-filesystem-server smart_cleanup /path/to/project --dry-run true

# Generate comprehensive project report
mcp-filesystem-server generate_report /path/to/project --format markdown
```


## Testing

This project includes comprehensive tests covering all 34+ implemented functions with 100% coverage.

### Running Tests

#### Quick Test Execution

```bash
# Windows - Automated script
run_tests.cmd

# Unix/Linux/Mac - Manual execution
cd /path/to/mcp-filesystem-server
go test ./filesystemserver -v -timeout=60s
```

#### Advanced Testing Options

```bash
# Run all tests with verbose output
go test ./filesystemserver -v

# Run specific test
go test ./filesystemserver -v -run TestAnalyzeFile_Valid

# Run tests matching a pattern
go test ./filesystemserver -v -run "TestAnalyze*"

# Run tests with coverage report
go test ./filesystemserver -v -cover

# Run tests with race detection
go test ./filesystemserver -v -race
```

#### Test Categories

The test suite is organized into logical sections:

- **Basic File Operations** - Core file/directory operations
- **Advanced Analysis** - Code analysis and project structure tools  
- **Intelligent Search** - Smart search and duplicate detection
- **Advanced Operations** - Batch operations, file comparison, validation
- **Metadata & Reports** - Statistics, metadata extraction, reporting
- **Utilities & Edge Cases** - Error handling, parameter validation, edge cases

#### Project Validation

```bash
# Windows - Complete project validation
validate_project.cmd

# Unix/Linux/Mac - Complete project validation  
./validate_project.sh
```

The validation script checks:
- ✅ File existence and structure
- ✅ Go syntax validation (`go vet`)
- ✅ Successful compilation
- ✅ Test execution
- ✅ Function coverage verification

### Test Coverage

- **34/34 functions** tested (100%)
- **40+ test cases** covering valid/invalid scenarios
- **Edge cases** handled (large files, symlinks, invalid paths)
- **Error conditions** validated
- **Parameter validation** comprehensive

### Continuous Integration

For CI/CD pipelines, use:

```yaml
# Example GitHub Actions step
- name: Run Tests
  run: |
    cd mcp-filesystem-server
    go test ./filesystemserver -v -timeout=60s -race -cover
```

### Test Files

- `filesystemserver/handler_test.go` - Main test suite (comprehensive)
- `filesystemserver/inprocess_test.go` - Integration tests
- `run_tests.cmd` - Windows test runner
- `validate_project.cmd` - Project validation (Windows)
- `validate_project.sh` - Project validation (Unix/Linux)


## License

See the [LICENSE](LICENSE) file for details.