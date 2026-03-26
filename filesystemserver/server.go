package filesystemserver

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "1.0.0"

// serverInstructions is sent to the client during the MCP initialize handshake.
// It ensures the AI model knows ALL available tools from the start, not just the
// ones it happens to discover on its own.
const serverInstructions = `You have access to a full-featured filesystem server with 18 tools. Use the RIGHT tool for each task:

## Editing files (IMPORTANT)
- **edit_file**: Modify specific text in a file (search/replace). Supports dry_run preview. ALWAYS prefer this over write_file for existing files.
- **write_file**: Create new files or full rewrites only. Supports create_backup.

## Reading files
- **read_file**: Read file contents. Use start_line/end_line for ranges. Use outline=true to get a symbol index (functions, classes, types with line numbers) — ideal for navigating large files without reading everything.
- **read_multiple_files**: Read several files in one call.
- **read_media_file**: Read images/binary files (returns base64 or ImageContent).

## File operations
- **copy_file**: Copy files or directories.
- **move_file**: Move or rename files and directories.
- **delete_file**: Delete files or directories (supports recursive=true).

## Directory operations
- **list_directory**: List directory contents. Use depth=N (1-10) for multi-level listing.
- **create_directory**: Create new directories (recursive).
- **tree**: Hierarchical JSON tree with configurable depth.

## Search and analysis
- **search**: Unified search — modes: files, content, duplicates. Supports regex patterns and include_content.
- **get_file_info**: File metadata (size, permissions, dates).
- **compare_files**: Diff two files (unified, context, or side-by-side format).
- **analyze_project**: Full project structure analysis with language detection and metrics.

## Bulk and advanced
- **batch_operations**: Multiple file operations (rename, delete, copy) in one call.
- **plan_task**: Create step-by-step plans for complex file operations.

## System
- **list_allowed_directories**: Show which directories the server can access.

## Key rules
1. To modify existing files: use edit_file, NOT write_file
2. To explore large files: use read_file with outline=true first, then read_file with start_line/end_line for specific sections
3. To search file contents: use search with mode=content and include_content=true
4. For bulk renames/deletes: use batch_operations, not individual calls`

func NewFilesystemServer(allowedDirs []string) (*server.MCPServer, error) {
	s, _, err := NewFilesystemServerWithOptions(allowedDirs)
	if err != nil {
		return nil, err
	}
	return s, nil
}

type ServerOption func(*serverOptions)

type serverOptions struct {
	auditLogDir string
}

type multiCloser struct {
	closers []io.Closer
}

func (m multiCloser) Close() error {
	var firstErr error
	for _, closer := range m.closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func WithDevelopmentAuditLogging(logDir string) ServerOption {
	return func(opts *serverOptions) {
		opts.auditLogDir = strings.TrimSpace(logDir)
	}
}

func NewFilesystemServerWithOptions(allowedDirs []string, options ...ServerOption) (*server.MCPServer, io.Closer, error) {
	var opts serverOptions
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

	h, err := NewFilesystemHandler(allowedDirs)
	if err != nil {
		return nil, nil, err
	}

	closers := make([]io.Closer, 0, 1)
	if opts.auditLogDir != "" {
		logger, err := NewAuditLogger(opts.auditLogDir)
		if err != nil {
			return nil, nil, fmt.Errorf("initialize audit logger: %w", err)
		}
		h.SetAuditLogger(logger)
		closers = append(closers, logger)
	}

	s := server.NewMCPServer(
		"secure-filesystem-server",
		Version,
		server.WithResourceCapabilities(false, false),
		server.WithLogging(),
		server.WithRoots(),
		server.WithInstructions(serverInstructions),
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

	// Give the handler access to the server for sending notifications (progress, logging).
	h.mcpServer = s

	// Listen for roots/list_changed so we re-fetch allowed dirs mid-session.
	s.AddNotificationHandler("notifications/roots/list_changed", func(ctx context.Context, notification mcp.JSONRPCNotification) {
		h.refreshRoots(ctx)
	})

	// Register resource handlers
	s.AddResource(mcp.NewResource(
		"file://",
		"File System",
		mcp.WithResourceDescription("Access to files and directories on the local file system"),
	), h.withResourceAudit(h.handleReadResource))

	// Register tool handlers — each wrapped with withNormalize for parameter
	// aliasing, type coercion, and JSON flexibility (ported from ultra).
	s.AddTool(mcp.NewTool(
		"read_file",
		mcp.WithTitleAnnotation("Read File"),
		mcp.WithDescription("Read the complete contents of a file from the file system. Use start_line/end_line to read a specific range without loading the entire file. Use outline=true to get a symbol index (functions, classes, types) with line numbers instead of contents."),
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
		mcp.WithBoolean("outline",
			mcp.Description("Return a structured symbol outline (functions, classes, types, etc.) with line numbers instead of file contents"),
		),
	), h.withNormalize("read_file", h.handleReadFile))

	s.AddTool(mcp.NewTool(
		"write_file",
		mcp.WithTitleAnnotation("Write File"),
		mcp.WithDescription("Create a new file or overwrite an existing file with new content. Supports optional backup and chunked streaming for large files."),
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
		mcp.WithBoolean("create_backup",
			mcp.Description("Create a .backup copy of the existing file before writing. Default: false."),
		),
		mcp.WithNumber("chunk_index",
			mcp.Description("0-based chunk index for streaming large files. Omit for a single write."),
		),
		mcp.WithNumber("total_chunks",
			mcp.Description("Total number of chunks expected. Required when chunk_index is set."),
		),
	), h.withNormalize("write_file", h.handleWriteFile))

	s.AddTool(mcp.NewTool(
		"edit_file",
		mcp.WithTitleAnnotation("Edit File"),
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
		"copy_file",
		mcp.WithTitleAnnotation("Copy File"),
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
		mcp.WithTitleAnnotation("Move / Rename"),
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
		"delete_file",
		mcp.WithTitleAnnotation("Delete File"),
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
		"list_directory",
		mcp.WithTitleAnnotation("List Directory"),
		mcp.WithDescription("List contents of a directory. By default lists one level only. Use depth to recurse (e.g. depth=2 shows two levels). Prefer this over tree when you want a flat listing with sizes."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path of the directory to list"),
			mcp.Required(),
		),
		mcp.WithNumber("depth",
			mcp.Description("How many levels deep to list (default: 1, max: 10). Use 1 for immediate contents only."),
		),
	), h.withNormalize("list_directory", h.handleListDirectory))

	s.AddTool(mcp.NewTool(
		"create_directory",
		mcp.WithTitleAnnotation("Create Directory"),
		mcp.WithDescription("Create a new directory or ensure a directory exists."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path of the directory to create"),
			mcp.Required(),
		),
	), h.withNormalize("create_directory", h.handleCreateDirectory))

	s.AddTool(mcp.NewTool(
		"tree",
		mcp.WithTitleAnnotation("Directory Tree"),
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
		"search",
		mcp.WithTitleAnnotation("Search Files"),
		mcp.WithDescription("Unified file search. mode=files: find by name/glob pattern; mode=content: regex search inside file contents with optional context lines (like grep -C N); mode=duplicates: find duplicate files by content hash."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Starting directory for the search"),
			mcp.Required(),
		),
		mcp.WithString("mode",
			mcp.Description("Search mode: 'files' (default), 'content', or 'duplicates'"),
		),
		mcp.WithString("pattern",
			mcp.Description("Filename glob or content regex. Required for files and content modes."),
		),
		mcp.WithBoolean("include_content",
			mcp.Description("(content mode) Search within file contents. Default: true."),
		),
		mcp.WithArray("file_types",
			mcp.Description("(content mode) Filter by extension, e.g. ['.go', '.js']"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("(content mode) Lines before/after each match, like grep -C N. Default: 0."),
		),
	), h.withNormalize("search", h.handleSearch))

	s.AddTool(mcp.NewTool(
		"get_file_info",
		mcp.WithTitleAnnotation("File Info"),
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
		mcp.WithTitleAnnotation("Allowed Directories"),
		mcp.WithDescription("Returns the list of directories that this server is allowed to access."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), h.withAudit("list_allowed_directories", h.handleListAllowedDirectories))

	s.AddTool(mcp.NewTool(
		"read_multiple_files",
		mcp.WithTitleAnnotation("Read Multiple Files"),
		mcp.WithDescription("Read the contents of multiple files in a single operation."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithArray("paths",
			mcp.Description("List of file paths to read"),
			mcp.Required(),
		),
	), h.withNormalize("read_multiple_files", h.handleReadMultipleFiles))

	s.AddTool(mcp.NewTool(
		"read_media_file",
		mcp.WithTitleAnnotation("Read Media File"),
		mcp.WithDescription("Read a media file (image or binary) returning base64 content or ImageContent for visual files."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Path to the media file"),
			mcp.Required(),
		),
	), h.withNormalize("read_media_file", h.handleReadMediaFile))

	s.AddTool(mcp.NewTool(
		"analyze_project",
		mcp.WithTitleAnnotation("Analyze Project"),
		mcp.WithDescription("Comprehensive project structure analysis with language detection and metrics — gives Claude full project context."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("path",
			mcp.Description("Project root directory"),
			mcp.Required(),
		),
	), h.withNormalize("analyze_project", h.handleAnalyzeProject))

	s.AddTool(mcp.NewTool(
		"compare_files",
		mcp.WithTitleAnnotation("Compare Files"),
		mcp.WithDescription("File comparison with diff generation and similarity analysis."),
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
		"batch_operations",
		mcp.WithTitleAnnotation("Batch Operations"),
		mcp.WithDescription("Execute multiple file operations in a single call — efficient for bulk changes."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithArray("operations",
			mcp.Description("Array of operations: [{type: 'rename|delete|copy', from: 'path', to: 'path'}]"),
			mcp.Required(),
		),
	), h.withNormalize("batch_operations", h.handleBatchEdit))

	s.AddTool(mcp.NewTool(
		"plan_task",
		mcp.WithTitleAnnotation("Plan Task"),
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

	s.AddTool(mcp.NewTool(
		"help",
		mcp.WithTitleAnnotation("Server Help"),
		mcp.WithDescription("IMPORTANT: Call this tool FIRST in every conversation. Returns the complete catalog of 19 filesystem tools (read, write, edit, list, search, copy, move, delete, compare, batch, analyze, tree, stats, media, chunks, duplicates, plan) with usage rules, best practices and parameter details. Without calling help first you will miss most available tools."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), h.withAudit("help", func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: serverInstructions},
			},
		}, nil
	}))

	return s, multiCloser{closers: closers}, nil
}
