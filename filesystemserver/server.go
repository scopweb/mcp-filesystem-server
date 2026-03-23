package filesystemserver

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "0.6.3"

func NewFilesystemServer(allowedDirs []string) (*server.MCPServer, error) {

	h, err := NewFilesystemHandler(allowedDirs)
	if err != nil {
		return nil, err
	}

	s := server.NewMCPServer(
		"secure-filesystem-server",
		Version,
		server.WithResourceCapabilities(true, true),
		server.WithRoots(),
	)

	// Wire roots handler so the handler can request allowed dirs from the client.
	h.requestRootsFn = func(ctx context.Context) ([]string, error) {
		result, err := s.RequestRoots(ctx, mcp.ListRootsRequest{})
		if err != nil {
			return nil, err
		}
		dirs := make([]string, 0, len(result.Roots))
		for _, root := range result.Roots {
			path := strings.TrimPrefix(root.URI, "file://")
			dirs = append(dirs, path)
		}
		return dirs, nil
	}

	// Register resource handlers
	s.AddResource(mcp.NewResource(
		"file://",
		"File System",
		mcp.WithResourceDescription("Access to files and directories on the local file system"),
	), h.handleReadResource)

	// Register tool handlers — each wrapped with withNormalize for parameter
	// aliasing, type coercion, and JSON flexibility (ported from ultra).
	s.AddTool(mcp.NewTool(
		"read_file",
		mcp.WithDescription("Read the complete contents of a file from the file system. Use start_line/end_line to read a specific range without loading the entire file."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to the file to read"),
			mcp.Required(),
		),
		mcp.WithNumber("start_line",
			mcp.Description("First line to read, 1-based inclusive (optional)"),
		),
		mcp.WithNumber("end_line",
			mcp.Description("Last line to read, 1-based inclusive (optional). Defaults to end of file."),
		),
	), h.withNormalize("read_file", h.handleReadFile))

	s.AddTool(mcp.NewTool(
		"write_file",
		mcp.WithDescription("Create a new file or overwrite an existing file with new content."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("path",
			mcp.Description("Path where to write the file"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Content to write to the file"),
			mcp.Required(),
		),
	), h.withNormalize("write_file", h.handleWriteFile))

	s.AddTool(mcp.NewTool(
		"list_directory",
		mcp.WithDescription("Get a detailed listing of all files and directories in a specified path."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path of the directory to list"),
			mcp.Required(),
		),
	), h.withNormalize("list_directory", h.handleListDirectory))

	s.AddTool(mcp.NewTool(
		"create_directory",
		mcp.WithDescription("Create a new directory or ensure a directory exists."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path of the directory to create"),
			mcp.Required(),
		),
	), h.withNormalize("create_directory", h.handleCreateDirectory))

	s.AddTool(mcp.NewTool(
		"copy_file",
		mcp.WithDescription("Copy files and directories."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("source",
			mcp.Description("Source path of the file or directory"),
			mcp.Required(),
		),
		mcp.WithString("destination",
			mcp.Description("Destination path"),
			mcp.Required(),
		),
	), h.withNormalize("copy_file", h.handleCopyFile))

	s.AddTool(mcp.NewTool(
		"move_file",
		mcp.WithDescription("Move or rename files and directories."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("source",
			mcp.Description("Source path of the file or directory"),
			mcp.Required(),
		),
		mcp.WithString("destination",
			mcp.Description("Destination path"),
			mcp.Required(),
		),
	), h.withNormalize("move_file", h.handleMoveFile))

	s.AddTool(mcp.NewTool(
		"search_files",
		mcp.WithDescription("Recursively search for files and directories matching a pattern."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Starting path for the search"),
			mcp.Required(),
		),
		mcp.WithString("pattern",
			mcp.Description("Search pattern to match against file names"),
			mcp.Required(),
		),
	), h.withNormalize("search_files", h.handleSearchFiles))

	s.AddTool(mcp.NewTool(
		"get_file_info",
		mcp.WithDescription("Retrieve detailed metadata about a file or directory."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to the file or directory"),
			mcp.Required(),
		),
	), h.withNormalize("get_file_info", h.handleGetFileInfo))

	s.AddTool(mcp.NewTool(
		"list_allowed_directories",
		mcp.WithDescription("Returns the list of directories that this server is allowed to access."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), h.handleListAllowedDirectories)

	s.AddTool(mcp.NewTool(
		"read_multiple_files",
		mcp.WithDescription("Read the contents of multiple files in a single operation."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithArray("paths",
			mcp.Description("List of file paths to read"),
			mcp.Required(),
		),
	), h.withNormalize("read_multiple_files", h.handleReadMultipleFiles))

	s.AddTool(mcp.NewTool(
		"tree",
		mcp.WithDescription("Returns a hierarchical JSON representation of a directory structure."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path of the directory to traverse"),
			mcp.Required(),
		),
		mcp.WithNumber("depth",
			mcp.Description("Maximum depth to traverse (default: 3)"),
		),
		mcp.WithBoolean("follow_symlinks",
			mcp.Description("Whether to follow symbolic links (default: false)"),
		),
	), h.withNormalize("tree", h.handleTree))

	s.AddTool(mcp.NewTool(
		"delete_file",
		mcp.WithDescription("Delete a file or directory from the file system."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("path",
			mcp.Description("Path to the file or directory to delete"),
			mcp.Required(),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Whether to recursively delete directories (default: false)"),
		),
	), h.withNormalize("delete_file", h.handleDeleteFile))

	s.AddTool(mcp.NewTool(
		"edit_file",
		mcp.WithDescription("Modify file content by replacing specific text without rewriting the entire file."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("path",
			mcp.Description("Path to the file to edit"),
			mcp.Required(),
		),
		mcp.WithString("old_text",
			mcp.Description("Text to be replaced"),
			mcp.Required(),
		),
		mcp.WithString("new_text",
			mcp.Description("New text to replace with"),
			mcp.Required(),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Preview changes without writing (returns unified diff)"),
		),
	), h.withNormalize("edit_file", h.handleEditFile))

	s.AddTool(mcp.NewTool(
		"analyze_file",
		mcp.WithDescription("Perform deep analysis of a file including complexity metrics, dependencies, and metadata optimized for Claude Desktop."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to the file to analyze"),
			mcp.Required(),
		),
	), h.withNormalize("analyze_file", h.handleAnalyzeFile))

	s.AddTool(mcp.NewTool(
		"smart_search",
		mcp.WithDescription("Intelligent search with regex support, content matching, and file type filtering - perfect for Claude's code analysis."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Starting path for the search"),
			mcp.Required(),
		),
		mcp.WithString("pattern",
			mcp.Description("Search pattern (supports regex)"),
			mcp.Required(),
		),
		mcp.WithBoolean("include_content",
			mcp.Description("Search within file contents (default: false)"),
		),
		mcp.WithArray("file_types",
			mcp.Description("Filter by file extensions (e.g., ['.js', '.py', '.go'])"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Number of lines to show before and after each content match (like grep -C N). Default: 0."),
		),
	), h.withNormalize("smart_search", h.handleSmartSearch))

	s.AddTool(mcp.NewTool(
		"find_duplicates",
		mcp.WithDescription("Find duplicate files by content hash - useful for cleanup and optimization tasks Claude might suggest."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Directory to scan for duplicates"),
			mcp.Required(),
		),
	), h.withNormalize("find_duplicates", h.handleFindDuplicates))

	s.AddTool(mcp.NewTool(
		"analyze_project",
		mcp.WithDescription("Comprehensive project structure analysis with language detection and metrics - gives Claude full project context."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Project root directory"),
			mcp.Required(),
		),
	), h.withNormalize("analyze_project", h.handleAnalyzeProject))

	s.AddTool(mcp.NewTool(
		"batch_operations",
		mcp.WithDescription("Execute multiple file operations in a single call - efficient for Claude's bulk suggestions."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithArray("operations",
			mcp.Description("Array of operations to execute: [{type: 'rename|delete|copy', from: 'path', to: 'path'}]"),
			mcp.Required(),
		),
	), h.withNormalize("batch_operations", h.handleBatchEdit))

	s.AddTool(mcp.NewTool(
		"compare_files",
		mcp.WithDescription("Advanced file comparison with diff generation and similarity analysis for Claude's code review tasks."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("file1",
			mcp.Description("First file to compare"),
			mcp.Required(),
		),
		mcp.WithString("file2",
			mcp.Description("Second file to compare"),
			mcp.Required(),
		),
		mcp.WithString("format",
			mcp.Description("Output format: 'unified', 'context', 'side-by-side' (default: unified)"),
		),
	), h.withNormalize("compare_files", h.handleCompareFiles))

	s.AddTool(mcp.NewTool(
		"performance_analysis",
		mcp.WithDescription("Analyze file system performance metrics and identify bottlenecks."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to analyze"),
			mcp.Required(),
		),
		mcp.WithString("operation",
			mcp.Description("Operation to benchmark: 'read', 'write', 'list' (default: all)"),
		),
	), h.withNormalize("performance_analysis", h.handlePerformanceAnalysis))

	s.AddTool(mcp.NewTool(
		"generate_report",
		mcp.WithDescription("Generate comprehensive reports in various formats (JSON, HTML, Markdown) for Claude's analysis."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to analyze for report"),
			mcp.Required(),
		),
		mcp.WithString("format",
			mcp.Description("Report format: 'json', 'html', 'markdown' (default: json)"),
		),
		mcp.WithString("output",
			mcp.Description("Output file path (optional)"),
		),
		mcp.WithArray("sections",
			mcp.Description("Report sections to include: ['overview', 'files', 'quality', 'dependencies', 'security']"),
		),
	), h.withNormalize("generate_report", h.handleGenerateReport))

	s.AddTool(mcp.NewTool(
		"smart_sync",
		mcp.WithDescription("Intelligent file synchronization with conflict detection and resolution suggestions."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("source",
			mcp.Description("Source directory"),
			mcp.Required(),
		),
		mcp.WithString("target",
			mcp.Description("Target directory"),
			mcp.Required(),
		),
		mcp.WithString("mode",
			mcp.Description("Sync mode: 'preview', 'merge', 'overwrite' (default: preview)"),
		),
		mcp.WithArray("exclude_patterns",
			mcp.Description("Patterns to exclude from sync"),
		),
	), h.withNormalize("smart_sync", h.handleSmartSync))

	s.AddTool(mcp.NewTool(
		"assist_refactor",
		mcp.WithDescription("Assist with code refactoring by analyzing dependencies and suggesting safe changes."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("path",
			mcp.Description("File or directory to refactor"),
			mcp.Required(),
		),
		mcp.WithString("operation",
			mcp.Description("Refactor operation: 'rename', 'extract', 'inline', 'move'"),
			mcp.Required(),
		),
		mcp.WithString("target",
			mcp.Description("Target for refactoring (new name, extracted function name, etc.)"),
		),
		mcp.WithObject("options",
			mcp.Description("Refactoring options"),
		),
	), h.withNormalize("assist_refactor", h.handleAssistRefactor))

	s.AddTool(mcp.NewTool(
		"plan_task",
		mcp.WithDescription("Create step-by-step execution plan for complex file operations."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("description",
			mcp.Description("Task description"),
			mcp.Required(),
		),
		mcp.WithArray("target_files",
			mcp.Description("Files to modify"),
		),
		mcp.WithString("workspace",
			mcp.Description("Workspace path"),
		),
	), h.withNormalize("plan_task", h.handlePlanTask))

	// ARCHIVOS FRAGMENTADOS - Chunked Operations
	s.AddTool(mcp.NewTool(
		"chunked_write",
		mcp.WithDescription("Write large files in chunks to avoid memory limits."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("path",
			mcp.Description("Path to write the file"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Content chunk to write"),
			mcp.Required(),
		),
		mcp.WithNumber("chunk_index",
			mcp.Description("Current chunk index (0-based)"),
			mcp.Required(),
		),
		mcp.WithNumber("total_chunks",
			mcp.Description("Total number of chunks"),
			mcp.Required(),
		),
	), h.withNormalize("chunked_write", h.handleChunkedWrite))

	s.AddTool(mcp.NewTool(
		"split_file",
		mcp.WithDescription("Split large file into smaller chunks."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to file to split"),
			mcp.Required(),
		),
		mcp.WithNumber("chunk_size",
			mcp.Description("Size of each chunk in bytes (default: 1MB)"),
		),
	), h.withNormalize("split_file", h.handleSplitFile))

	s.AddTool(mcp.NewTool(
		"join_files",
		mcp.WithDescription("Join multiple file chunks into single file."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("target_path",
			mcp.Description("Path for the joined file"),
			mcp.Required(),
		),
		mcp.WithArray("source_files",
			mcp.Description("List of chunk files to join"),
			mcp.Required(),
		),
	), h.withNormalize("join_files", h.handleJoinFiles))

	s.AddTool(mcp.NewTool(
		"write_file_safe",
		mcp.WithDescription("Safe file write with atomic operation and optional backup."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("path",
			mcp.Description("Path to write the file"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Content to write"),
			mcp.Required(),
		),
		mcp.WithBoolean("create_backup",
			mcp.Description("Create backup before writing (default: false)"),
		),
	), h.withNormalize("write_file_safe", h.handleWriteFileSafe))

	s.AddTool(mcp.NewTool(
		"read_media_file",
		mcp.WithDescription("Read a media file (image or binary) returning base64 content or ImageContent for visual files."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to the media file"),
			mcp.Required(),
		),
	), h.withNormalize("read_media_file", h.handleReadMediaFile))

	return s, nil
}
