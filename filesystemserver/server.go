package filesystemserver

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "1.1.0"

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
- **delete_file**: Delete files or directories (supports recursive=true). Cannot delete allowed root directories.

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
		mcp.WithDescription("Read file contents. Use start_line/end_line for reading specific ranges. "+
			"Use outline=true to get a symbol index (functions, classes, types with line numbers) — "+
			"ideal for navigating large files before reading specific sections. To MODIFY files use edit_file."),
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
		mcp.WithDescription("Create NEW files or full overwrite. Supports chunked streaming for large files "+
			"(set chunk_index + total_chunks). Use create_backup:true to save a .backup copy before overwriting. "+
			"For modifying existing files ALWAYS use edit_file instead."),
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
		mcp.WithDescription("Modify existing files by replacing specific text (search → replace). "+
			"Use dry_run:true to preview changes as a unified diff before applying. "+
			"ALWAYS prefer this over write_file for existing files."),
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
		mcp.WithDescription("Copy files or directories to a new location. Supports recursive directory copy."),
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
		mcp.WithDescription("Move or rename files and directories. Works as both move and rename. Cannot move allowed root directories."),
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
		mcp.WithDescription("Delete files or directories. Use recursive:true for non-empty directories. Cannot delete allowed root directories."),
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
		mcp.WithDescription("List directory contents with file sizes and types. Use depth:N (1-10) for multi-level listing. "+
			"For hierarchical JSON output use the tree tool instead."),
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
		mcp.WithDescription("Create directories recursively (like mkdir -p). Creates all intermediate directories. Idempotent."),
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
		mcp.WithDescription("Hierarchical JSON tree of directory structure with configurable depth. "+
			"Use follow_symlinks:true to traverse symbolic links. For flat listing use list_directory."),
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
		mcp.WithDescription("Unified search with three modes. mode=files: find files by name/glob pattern. "+
			"mode=content: regex search inside file contents (like grep), use context_lines for surrounding lines "+
			"and file_types:[\".go\",\".ts\"] to filter. mode=duplicates: find duplicate files by content hash."),
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
		mcp.WithDescription("File/directory metadata: size, permissions, timestamps, type. Quick way to check existence and properties."),
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
		mcp.WithDescription("Show which directories this server is allowed to access. Call to verify permissions before operations."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), h.withAudit("list_allowed_directories", h.handleListAllowedDirectories))

	s.AddTool(mcp.NewTool(
		"read_multiple_files",
		mcp.WithTitleAnnotation("Read Multiple Files"),
		mcp.WithDescription("Read multiple files in one call. More efficient than sequential read_file calls when you need several files at once."),
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
		mcp.WithDescription("Read images and binary files. Returns base64 data or ImageContent for supported image formats (png, jpg, gif, webp)."),
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
		mcp.WithDescription("Full project structure analysis: language detection, file type distribution, "+
			"directory layout, code metrics and patterns. Scans entire project tree to identify "+
			"languages, frameworks, build systems, and file organization."),
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
		mcp.WithDescription("Diff two files. Formats: unified (default, like git diff), context (with surrounding lines), side-by-side (parallel comparison)."),
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
		mcp.WithDescription("Bulk file operations in one call. Pass an array of operations, each with type (rename, delete, copy), "+
			"from and to paths. More efficient than individual calls for bulk renames, deletes, or copies."),
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
		mcp.WithDescription("Create a step-by-step execution plan for complex file operations. "+
			"Analyzes target files and workspace to recommend which tools to use and in what order. "+
			"Use before batch_operations for complex multi-file tasks."),
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
		mcp.WithDescription("Call FIRST in every conversation. Returns the complete catalog of all filesystem tools "+
			"(read, write, edit, search, copy, move, delete, compare, batch, analyze, tree, plan, media) "+
			"with usage rules and best practices. Without calling help first you may miss available tools."),
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
