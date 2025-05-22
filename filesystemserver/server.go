package filesystemserver

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "0.4.1"

func NewFilesystemServer(allowedDirs []string) (*server.MCPServer, error) {

	h, err := NewFilesystemHandler(allowedDirs)
	if err != nil {
		return nil, err
	}

	s := server.NewMCPServer(
		"secure-filesystem-server",
		Version,
		server.WithResourceCapabilities(true, true),
	)

	// Register resource handlers
	s.AddResource(mcp.NewResource(
		"file://",
		"File System",
		mcp.WithResourceDescription("Access to files and directories on the local file system"),
	), h.handleReadResource)

	// Register tool handlers
	s.AddTool(mcp.NewTool(
		"read_file",
		mcp.WithDescription("Read the complete contents of a file from the file system."),
		mcp.WithString("path",
			mcp.Description("Path to the file to read"),
			mcp.Required(),
		),
	), h.handleReadFile)

	s.AddTool(mcp.NewTool(
		"write_file",
		mcp.WithDescription("Create a new file or overwrite an existing file with new content."),
		mcp.WithString("path",
			mcp.Description("Path where to write the file"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Content to write to the file"),
			mcp.Required(),
		),
	), h.handleWriteFile)

	s.AddTool(mcp.NewTool(
		"list_directory",
		mcp.WithDescription("Get a detailed listing of all files and directories in a specified path."),
		mcp.WithString("path",
			mcp.Description("Path of the directory to list"),
			mcp.Required(),
		),
	), h.handleListDirectory)

	s.AddTool(mcp.NewTool(
		"create_directory",
		mcp.WithDescription("Create a new directory or ensure a directory exists."),
		mcp.WithString("path",
			mcp.Description("Path of the directory to create"),
			mcp.Required(),
		),
	), h.handleCreateDirectory)

	s.AddTool(mcp.NewTool(
		"copy_file",
		mcp.WithDescription("Copy files and directories."),
		mcp.WithString("source",
			mcp.Description("Source path of the file or directory"),
			mcp.Required(),
		),
		mcp.WithString("destination",
			mcp.Description("Destination path"),
			mcp.Required(),
		),
	), h.handleCopyFile)

	s.AddTool(mcp.NewTool(
		"move_file",
		mcp.WithDescription("Move or rename files and directories."),
		mcp.WithString("source",
			mcp.Description("Source path of the file or directory"),
			mcp.Required(),
		),
		mcp.WithString("destination",
			mcp.Description("Destination path"),
			mcp.Required(),
		),
	), h.handleMoveFile)

	s.AddTool(mcp.NewTool(
		"search_files",
		mcp.WithDescription("Recursively search for files and directories matching a pattern."),
		mcp.WithString("path",
			mcp.Description("Starting path for the search"),
			mcp.Required(),
		),
		mcp.WithString("pattern",
			mcp.Description("Search pattern to match against file names"),
			mcp.Required(),
		),
	), h.handleSearchFiles)

	s.AddTool(mcp.NewTool(
		"get_file_info",
		mcp.WithDescription("Retrieve detailed metadata about a file or directory."),
		mcp.WithString("path",
			mcp.Description("Path to the file or directory"),
			mcp.Required(),
		),
	), h.handleGetFileInfo)

	s.AddTool(mcp.NewTool(
		"list_allowed_directories",
		mcp.WithDescription("Returns the list of directories that this server is allowed to access."),
	), h.handleListAllowedDirectories)

	s.AddTool(mcp.NewTool(
		"read_multiple_files",
		mcp.WithDescription("Read the contents of multiple files in a single operation."),
		mcp.WithArray("paths",
			mcp.Description("List of file paths to read"),
			mcp.Required(),
		),
	), h.handleReadMultipleFiles)

	s.AddTool(mcp.NewTool(
		"tree",
		mcp.WithDescription("Returns a hierarchical JSON representation of a directory structure."),
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
	), h.handleTree)

	s.AddTool(mcp.NewTool(
		"delete_file",
		mcp.WithDescription("Delete a file or directory from the file system."),
		mcp.WithString("path",
			mcp.Description("Path to the file or directory to delete"),
			mcp.Required(),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Whether to recursively delete directories (default: false)"),
		),
	), h.handleDeleteFile)
	
	s.AddTool(mcp.NewTool(
		"edit_file",
		mcp.WithDescription("Modify file content by replacing specific text without rewriting the entire file."),
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
	), h.handleEditFile)

	// Herramienta de análisis profundo de archivos
	s.AddTool(mcp.NewTool(
		"analyze_file",
		mcp.WithDescription("Perform deep analysis of a file including complexity metrics, dependencies, and metadata optimized for Claude Desktop."),
		mcp.WithString("path",
			mcp.Description("Path to the file to analyze"),
			mcp.Required(),
		),
	), h.handleAnalyzeFile)

	// Búsqueda inteligente optimizada para Claude
	s.AddTool(mcp.NewTool(
		"smart_search",
		mcp.WithDescription("Intelligent search with regex support, content matching, and file type filtering - perfect for Claude's code analysis."),
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
	), h.handleSmartSearch)

	// Detección de archivos duplicados
	s.AddTool(mcp.NewTool(
		"find_duplicates",
		mcp.WithDescription("Find duplicate files by content hash - useful for cleanup and optimization tasks Claude might suggest."),
		mcp.WithString("path",
			mcp.Description("Directory to scan for duplicates"),
			mcp.Required(),
		),
	), h.handleFindDuplicates)

	// Análisis de estructura de proyecto
	s.AddTool(mcp.NewTool(
		"analyze_project",
		mcp.WithDescription("Comprehensive project structure analysis with language detection and metrics - gives Claude full project context."),
		mcp.WithString("path",
			mcp.Description("Project root directory"),
			mcp.Required(),
		),
	), h.handleAnalyzeProject)

	// Operaciones en lote
	s.AddTool(mcp.NewTool(
		"batch_operations",
		mcp.WithDescription("Execute multiple file operations in a single call - efficient for Claude's bulk suggestions."),
		mcp.WithArray("operations",
			mcp.Description("Array of operations to execute: [{type: 'rename|delete|copy', from: 'path', to: 'path'}]"),
			mcp.Required(),
		),
	), h.handleBatchEdit)


	// Comparación de archivos avanzada
	s.AddTool(mcp.NewTool(
		"compare_files",
		mcp.WithDescription("Advanced file comparison with diff generation and similarity analysis for Claude's code review tasks."),
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
	), h.handleCompareFiles)

	// Monitoreeo de cambios en tiempo real
	s.AddTool(mcp.NewTool(
		"watch_file",
		mcp.WithDescription("Monitor file changes and return modification events - helps Claude track development progress."),
		mcp.WithString("path",
			mcp.Description("File or directory to monitor"),
			mcp.Required(),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Maximum time to watch in seconds (default: 30)"),
		),
	), h.handleWatchFile)

	// Validación de sintaxis específica por lenguaje
	s.AddTool(mcp.NewTool(
		"validate_syntax",
		mcp.WithDescription("Validate syntax for various programming languages - essential for Claude's code generation validation."),
		mcp.WithString("path",
			mcp.Description("Path to file to validate"),
			mcp.Required(),
		),
		mcp.WithString("language",
			mcp.Description("Language to validate (auto-detect if not specified)"),
		),
	), h.handleValidateSyntax)

	// Extracción de metadatos específicos
	s.AddTool(mcp.NewTool(
		"extract_metadata",
		mcp.WithDescription("Extract comprehensive metadata from files including EXIF, document properties, and code metrics."),
		mcp.WithString("path",
			mcp.Description("Path to file"),
			mcp.Required(),
		),
		mcp.WithString("type",
			mcp.Description("Metadata type: 'exif', 'document', 'code', 'all' (default: all)"),
		),
	), h.handleExtractMetadata)

	// Generación de checksums y verificación de integridad
	s.AddTool(mcp.NewTool(
		"generate_checksum",
		mcp.WithDescription("Generate various checksums for file integrity verification - useful for Claude's security recommendations."),
		mcp.WithString("path",
			mcp.Description("Path to file or directory"),
			mcp.Required(),
		),
		mcp.WithArray("algorithms",
			mcp.Description("Hash algorithms: ['md5', 'sha1', 'sha256', 'sha512']"),
		),
	), h.handleGenerateChecksum)

	// Análisis de dependencias de código
	s.AddTool(mcp.NewTool(
		"analyze_dependencies",
		mcp.WithDescription("Analyze code dependencies and imports - gives Claude full context of project relationships."),
		mcp.WithString("path",
			mcp.Description("Path to file or project root"),
			mcp.Required(),
		),
		mcp.WithString("language",
			mcp.Description("Programming language (auto-detect if not specified)"),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Analyze dependencies recursively (default: true)"),
		),
	), h.handleAnalyzeDependencies)


	// Limpieza inteligente de archivos
	s.AddTool(mcp.NewTool(
		"smart_cleanup",
		mcp.WithDescription("Intelligent cleanup of temporary files, logs, and build artifacts - helps Claude suggest project maintenance."),
		mcp.WithString("path",
			mcp.Description("Directory to clean"),
			mcp.Required(),
		),
		mcp.WithArray("patterns",
			mcp.Description("File patterns to clean (default: common temp files)"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Show what would be deleted without actually deleting (default: true)"),
		),
	), h.handleSmartCleanup)


	// Conversión de formato de archivos
	s.AddTool(mcp.NewTool(
		"convert_file",
		mcp.WithDescription("Convert files between different formats - supports text encodings, line endings, and basic format conversions."),
		mcp.WithString("source",
			mcp.Description("Source file path"),
			mcp.Required(),
		),
		mcp.WithString("target",
			mcp.Description("Target file path"),
			mcp.Required(),
		),
		mcp.WithString("from_format",
			mcp.Description("Source format"),
		),
		mcp.WithString("to_format",
			mcp.Description("Target format"),
		),
		mcp.WithString("encoding",
			mcp.Description("Target encoding (utf-8, ascii, etc.)"),
		),
	), h.handleConvertFile)


	// Template de archivos y scaffolding
	s.AddTool(mcp.NewTool(
		"create_from_template", 
		mcp.WithDescription("Create files from templates with variable substitution - perfect for Claude's code generation workflows."),
		mcp.WithString("template_path",
			mcp.Description("Path to template file"),
			mcp.Required(),
		),
		mcp.WithString("output_path",
			mcp.Description("Output file path"),
			mcp.Required(),
		),
		mcp.WithObject("variables",
			mcp.Description("Template variables as key-value pairs"),
		),
	), h.handleCreateFromTemplate)


	// Análisis de calidad de código  
	s.AddTool(mcp.NewTool(
		"code_quality_check",
		mcp.WithDescription("Comprehensive code quality analysis including complexity, maintainability, and best practices compliance."),
		mcp.WithString("path",
			mcp.Description("File or directory to analyze"),
			mcp.Required(),
		),
		mcp.WithString("language",
			mcp.Description("Programming language (auto-detect if not specified)"),
		),
		mcp.WithArray("metrics",
			mcp.Description("Specific metrics to calculate: ['complexity', 'maintainability', 'readability', 'security']"),
		),
	), h.handleCodeQualityCheck)


	// Estadísticas de directorio
	s.AddTool(mcp.NewTool(
		"directory_stats",
		mcp.WithDescription("Calculate comprehensive directory statistics including file counts, sizes, and language distribution."),
		mcp.WithString("path",
			mcp.Description("Directory to analyze"),
			mcp.Required(),
		),
	), h.handleDirectoryStats)


	// Búsqueda avanzada de texto
	s.AddTool(mcp.NewTool(
		"advanced_text_search",
		mcp.WithDescription("Advanced text search with regex, context, and precise matching options - perfect for Claude's code analysis needs."),
		mcp.WithString("path",
			mcp.Description("Directory to search in"),
			mcp.Required(),
		),
		mcp.WithString("pattern",
			mcp.Description("Search pattern (regex supported)"),
			mcp.Required(),
		),
		mcp.WithBoolean("case_sensitive",
			mcp.Description("Case sensitive search (default: false)"),
		),
		mcp.WithBoolean("whole_word",
			mcp.Description("Match whole words only (default: false)"),
		),
		mcp.WithBoolean("include_context",
			mcp.Description("Include surrounding lines for context (default: false)"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Number of context lines to include (default: 3)"),
		),
	), h.handleAdvancedTextSearch)

	// Funciones adicionales específicas para Claude Desktop
	
	// Análisis de rendimiento de archivos
	s.AddTool(mcp.NewTool(
		"performance_analysis",
		mcp.WithDescription("Analyze file system performance metrics and identify bottlenecks."),
		mcp.WithString("path",
			mcp.Description("Path to analyze"),
			mcp.Required(),
		),
		mcp.WithString("operation",
			mcp.Description("Operation to benchmark: 'read', 'write', 'list' (default: all)"),
		),
	), h.handlePerformanceAnalysis)

	// Generador de reportes
	s.AddTool(mcp.NewTool(
		"generate_report",
		mcp.WithDescription("Generate comprehensive reports in various formats (JSON, HTML, Markdown) for Claude's analysis."),
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
	), h.handleGenerateReport)

	// Sincronización inteligente
	s.AddTool(mcp.NewTool(
		"smart_sync",
		mcp.WithDescription("Intelligent file synchronization with conflict detection and resolution suggestions."),
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
	), h.handleSmartSync)

	// Herramienta de refactoring asistido
	s.AddTool(mcp.NewTool(
		"assist_refactor",
		mcp.WithDescription("Assist with code refactoring by analyzing dependencies and suggesting safe changes."),
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
	), h.handleAssistRefactor)

	return s, nil
}
