package filesystemserver

import "time"

const (
	// Maximum size for inline content (5MB)
	MAX_INLINE_SIZE = 5 * 1024 * 1024
	// Maximum size for base64 encoding (1MB)
	MAX_BASE64_SIZE = 1 * 1024 * 1024
	// Maximum size for chunked write (1MB)
	MAX_CHUNK_SIZE = 1 * 1024 * 1024
)

// FileInfo represents basic file information
type FileInfo struct {
	Size        int64     `json:"size"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	Accessed    time.Time `json:"accessed"`
	IsDirectory bool      `json:"isDirectory"`
	IsFile      bool      `json:"isFile"`
	Permissions string    `json:"permissions"`
}

// FileNode represents a node in the file tree
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" or "directory"
	Size     int64       `json:"size,omitempty"`
	Modified time.Time   `json:"modified,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// FilesystemHandler manages file system operations
type FilesystemHandler struct {
	allowedDirs []string
}

// FileDiff represents the result of file comparison
type FileDiff struct {
	File1     string   `json:"file1"`
	File2     string   `json:"file2"`
	Similar   float64  `json:"similarity"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Unchanged int      `json:"unchanged"`
}

// FileWatchEvent represents a file system event
type FileWatchEvent struct {
	Path      string    `json:"path"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// SyntaxValidation represents syntax validation results
type SyntaxValidation struct {
	Valid    bool     `json:"valid"`
	Language string   `json:"language"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// DependencyAnalysis represents code dependency analysis
type DependencyAnalysis struct {
	File         string              `json:"file"`
	Language     string              `json:"language"`
	Imports      []string            `json:"imports"`
	Dependencies map[string][]string `json:"dependencies"`
	Circular     []string            `json:"circular"`
	Missing      []string            `json:"missing"`
}

// CleanupResult represents cleanup operation results
type CleanupResult struct {
	TotalFiles   int      `json:"totalFiles"`
	TotalSize    int64    `json:"totalSize"`
	FilesRemoved []string `json:"filesRemoved"`
	SizeFreed    int64    `json:"sizeFreed"`
	DryRun       bool     `json:"dryRun"`
}

// FileAnalysis represents comprehensive file analysis
type FileAnalysis struct {
	Path         string          `json:"path"`
	Size         int64           `json:"size"`
	Lines        int             `json:"lines"`
	Words        int             `json:"words"`
	Characters   int             `json:"characters"`
	MimeType     string          `json:"mimeType"`
	Encoding     string          `json:"encoding"`
	LineEndings  string          `json:"lineEndings"`
	LastModified time.Time       `json:"lastModified"`
	Permissions  string          `json:"permissions"`
	Hash         FileHashes      `json:"hashes"`
	Language     string          `json:"language,omitempty"`
	Complexity   *CodeComplexity `json:"complexity,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
}

// FileHashes contains file hash information
type FileHashes struct {
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// CodeComplexity represents code complexity metrics
type CodeComplexity struct {
	CyclomaticComplexity int `json:"cyclomaticComplexity"`
	FunctionCount        int `json:"functionCount"`
	ClassCount           int `json:"classCount"`
	ImportCount          int `json:"importCount"`
}

// DuplicateFile represents a duplicate file entry
type DuplicateFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// ProjectStructure represents project analysis results
type ProjectStructure struct {
	Root        string              `json:"root"`
	Languages   map[string]int      `json:"languages"`
	FileTypes   map[string]int      `json:"fileTypes"`
	TotalFiles  int                 `json:"totalFiles"`
	TotalSize   int64               `json:"totalSize"`
	Directories []string            `json:"directories"`
	Structure   map[string][]string `json:"structure"`
}

// ChunkWriteResult represents chunked file write results
type ChunkWriteResult struct {
	Path      string `json:"path"`
	TotalSize int64  `json:"total_size"`
	Chunks    int    `json:"chunks"`
	Completed bool   `json:"completed"`
	Error     string `json:"error,omitempty"`
}

// SearchMatch represents a text search match
type SearchMatch struct {
	File       string   `json:"file"`
	LineNumber int      `json:"line_number"`
	Line       string   `json:"line"`
	Context    []string `json:"context,omitempty"`
	MatchStart int      `json:"match_start"`
	MatchEnd   int      `json:"match_end"`
}

// DirectoryStats represents directory statistics
type DirectoryStats struct {
	Path             string         `json:"path"`
	TotalFiles       int            `json:"total_files"`
	TotalDirectories int            `json:"total_directories"`
	TotalSize        int64          `json:"total_size"`
	AverageFileSize  int64          `json:"average_file_size"`
	LargestFile      string         `json:"largest_file"`
	LargestFileSize  int64          `json:"largest_file_size"`
	FileTypes        map[string]int `json:"file_types"`
	Languages        map[string]int `json:"languages"`
	LastModified     time.Time      `json:"last_modified"`
}

// EditResult represents file edit operation results
type EditResult struct {
	ModifiedContent  string
	ReplacementCount int
	MatchConfidence  string
	LinesAffected    int
}

// SplitResult represents file split operation results
type SplitResult struct {
	SourceFile  string   `json:"source_file"`
	SourceSize  int64    `json:"source_size"`
	ChunkSize   int64    `json:"chunk_size"`
	TotalChunks int64    `json:"total_chunks"`
	ChunkFiles  []string `json:"chunk_files"`
}

// JoinResult represents file join operation results
type JoinResult struct {
	TargetFile  string   `json:"target_file"`
	SourceFiles []string `json:"source_files"`
	TotalSize   int64    `json:"total_size"`
}