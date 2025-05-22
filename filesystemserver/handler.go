package filesystemserver

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"slices"

	"github.com/gabriel-vasile/mimetype"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// Maximum size for inline content (5MB)
	MAX_INLINE_SIZE = 5 * 1024 * 1024
	// Maximum size for base64 encoding (1MB)
	MAX_BASE64_SIZE = 1 * 1024 * 1024
)

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

type FilesystemHandler struct {
	allowedDirs []string
}

// Estructuras adicionales
type FileDiff struct {
	File1     string   `json:"file1"`
	File2     string   `json:"file2"`
	Similar   float64  `json:"similarity"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Unchanged int      `json:"unchanged"`
}

type FileWatchEvent struct {
	Path      string    `json:"path"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

type SyntaxValidation struct {
	Valid    bool     `json:"valid"`
	Language string   `json:"language"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type DependencyAnalysis struct {
	File         string              `json:"file"`
	Language     string              `json:"language"`
	Imports      []string            `json:"imports"`
	Dependencies map[string][]string `json:"dependencies"`
	Circular     []string            `json:"circular"`
	Missing      []string            `json:"missing"`
}

type CleanupResult struct {
	TotalFiles   int      `json:"totalFiles"`
	TotalSize    int64    `json:"totalSize"`
	FilesRemoved []string `json:"filesRemoved"`
	SizeFreed    int64    `json:"sizeFreed"`
	DryRun       bool     `json:"dryRun"`
}

// Nuevas estructuras para funcionalidades avanzadas
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

type FileHashes struct {
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

type CodeComplexity struct {
	CyclomaticComplexity int `json:"cyclomaticComplexity"`
	FunctionCount        int `json:"functionCount"`
	ClassCount           int `json:"classCount"`
	ImportCount          int `json:"importCount"`
}

type DuplicateFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

type ProjectStructure struct {
	Root        string              `json:"root"`
	Languages   map[string]int      `json:"languages"`
	FileTypes   map[string]int      `json:"fileTypes"`
	TotalFiles  int                 `json:"totalFiles"`
	TotalSize   int64               `json:"totalSize"`
	Directories []string            `json:"directories"`
	Structure   map[string][]string `json:"structure"`
}

// Nueva función: Análisis profundo de archivos para Claude
func (fs *FilesystemHandler) handleAnalyzeFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf("path error: %v", err)
	}

	analysis, err := fs.performDeepFileAnalysis(validPath)
	if err != nil {
		return nil, fmt.Errorf("analysis error: %v", err)
	}

	jsonData, _ := json.MarshalIndent(analysis, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("📊 Deep File Analysis for: %s\n\n%s", path, string(jsonData)),
			},
		},
	}, nil
}

// Función auxiliar para análisis profundo
func (fs *FilesystemHandler) performDeepFileAnalysis(path string) (*FileAnalysis, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	// Calcular hashes
	md5Hash := md5.Sum(content)
	sha256Hash := sha256.Sum256(content)

	analysis := &FileAnalysis{
		Path:         path,
		Size:         info.Size(),
		Lines:        strings.Count(contentStr, "\n") + 1,
		Words:        len(strings.Fields(contentStr)),
		Characters:   len(contentStr),
		MimeType:     detectMimeType(path),
		LastModified: info.ModTime(),
		Permissions:  fmt.Sprintf("%o", info.Mode().Perm()),
		Hash: FileHashes{
			MD5:    hex.EncodeToString(md5Hash[:]),
			SHA256: hex.EncodeToString(sha256Hash[:]),
		},
		Language: fs.detectLanguage(contentStr),
	}

	// Detectar encoding
	if utf8.ValidString(contentStr) {
		analysis.Encoding = "UTF-8"
	} else {
		analysis.Encoding = "unknown"
	}

	// Detectar terminaciones de línea
	if strings.Contains(contentStr, "\r\n") {
		analysis.LineEndings = "CRLF"
	} else if strings.Contains(contentStr, "\r") {
		analysis.LineEndings = "CR"
	} else {
		analysis.LineEndings = "LF"
	}

	// Análisis de complejidad para archivos de código
	if isTextFile(analysis.MimeType) {
		complexity := fs.calculateCodeComplexity(contentStr, analysis.Language)
		analysis.Complexity = &complexity

		// Extraer dependencias
		deps := fs.extractDependencies(contentStr, analysis.Language)
		analysis.Dependencies = deps
	}

	return analysis, nil
}

// Función auxiliar para calcular complejidad de código
func (fs *FilesystemHandler) calculateCodeComplexity(content, language string) CodeComplexity {
	complexity := CodeComplexity{
		CyclomaticComplexity: 1, // Complejidad base
	}

	// Contar funciones, clases e imports según el lenguaje
	switch language {
	case "go":
		complexity.FunctionCount = len(regexp.MustCompile(`func\s+\w+`).FindAllString(content, -1))
		complexity.ClassCount = len(regexp.MustCompile(`type\s+\w+\s+struct`).FindAllString(content, -1))
		complexity.ImportCount = len(regexp.MustCompile(`import\s+`).FindAllString(content, -1))

		// Complejidad ciclomática para Go
		patterns := []string{`\bif\b`, `\bfor\b`, `\bswitch\b`, `\bcase\b`, `\bselect\b`}
		for _, pattern := range patterns {
			complexity.CyclomaticComplexity += len(regexp.MustCompile(pattern).FindAllString(content, -1))
		}

	case "javascript":
		complexity.FunctionCount = len(regexp.MustCompile(`function\s+\w+|=>\s*{|\w+\s*:\s*function`).FindAllString(content, -1))
		complexity.ClassCount = len(regexp.MustCompile(`class\s+\w+`).FindAllString(content, -1))
		complexity.ImportCount = len(regexp.MustCompile(`import\s+.*from|require\s*\(`).FindAllString(content, -1))

		patterns := []string{`\bif\b`, `\bfor\b`, `\bwhile\b`, `\bswitch\b`, `\bcase\b`, `\btry\b`, `\bcatch\b`}
		for _, pattern := range patterns {
			complexity.CyclomaticComplexity += len(regexp.MustCompile(pattern).FindAllString(content, -1))
		}

	case "python":
		complexity.FunctionCount = len(regexp.MustCompile(`def\s+\w+`).FindAllString(content, -1))
		complexity.ClassCount = len(regexp.MustCompile(`class\s+\w+`).FindAllString(content, -1))
		complexity.ImportCount = len(regexp.MustCompile(`import\s+|from\s+.*import`).FindAllString(content, -1))

		patterns := []string{`\bif\b`, `\belif\b`, `\bfor\b`, `\bwhile\b`, `\btry\b`, `\bexcept\b`}
		for _, pattern := range patterns {
			complexity.CyclomaticComplexity += len(regexp.MustCompile(pattern).FindAllString(content, -1))
		}
	}

	return complexity
}

// Función auxiliar para extraer dependencias
func (fs *FilesystemHandler) extractDependencies(content, language string) []string {
	var dependencies []string

	switch language {
	case "go":
		// Extraer imports de Go
		re := regexp.MustCompile(`import\s+(?:"([^"]+)"|([a-zA-Z_][a-zA-Z0-9_]*)\s+"([^"]+)")`)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if match[1] != "" {
				dependencies = append(dependencies, match[1])
			} else if match[3] != "" {
				dependencies = append(dependencies, match[3])
			}
		}

	case "javascript":
		// Extraer imports/requires de JavaScript
		importRe := regexp.MustCompile(`import.*from\s+['"]([^'"]+)['"]|require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
		matches := importRe.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if match[1] != "" {
				dependencies = append(dependencies, match[1])
			} else if match[2] != "" {
				dependencies = append(dependencies, match[2])
			}
		}

	case "python":
		// Extraer imports de Python
		importRe := regexp.MustCompile(`(?:from\s+(\S+)\s+)?import\s+([^#\n]+)`)
		matches := importRe.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if match[1] != "" {
				dependencies = append(dependencies, match[1])
			}
			// Procesar múltiples imports en una línea
			imports := strings.Split(match[2], ",")
			for _, imp := range imports {
				dep := strings.TrimSpace(strings.Split(imp, " as ")[0])
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
		}
	}

	// Eliminar duplicados
	seen := make(map[string]bool)
	uniqueDeps := []string{}
	for _, dep := range dependencies {
		if !seen[dep] {
			seen[dep] = true
			uniqueDeps = append(uniqueDeps, dep)
		}
	}

	return uniqueDeps
}

// Comparación avanzada de archivos
func (fs *FilesystemHandler) handleCompareFiles(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	file1, _ := request.Params.Arguments["file1"].(string)
	file2, _ := request.Params.Arguments["file2"].(string)
	format, _ := request.Params.Arguments["format"].(string)

	if file1 == "" || file2 == "" {
		return nil, errors.New("both file1 and file2 are required")
	}

	if format == "" {
		format = "unified"
	}

	validPath1, err := fs.validatePath(file1)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Error with file1: %v", err))
	}

	validPath2, err := fs.validatePath(file2)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Error with file2: %v", err))
	}

	diff, err := fs.compareFiles(validPath1, validPath2, format)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Comparison error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(diff, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔍 File Comparison Results:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Validación de sintaxis
func (fs *FilesystemHandler) handleValidateSyntax(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	language, _ := request.Params.Arguments["language"].(string)

	if path == "" {
		return nil, errors.New("path is required")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	validation, err := fs.validateSyntax(validPath, language)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Validation error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(validation, "", "  ")

	status := "✅ Valid"
	if !validation.Valid {
		status = "❌ Invalid"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔍 Syntax Validation: %s\n\n%s", status, string(jsonData)),
			},
		},
	}, nil
}

// Generación de checksums
func (fs *FilesystemHandler) handleGenerateChecksum(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	algorithmsParam, _ := request.Params.Arguments["algorithms"].([]interface{})

	if path == "" {
		return nil, errors.New("path is required")
	}

	algorithms := []string{"md5", "sha256"} // default
	if len(algorithmsParam) > 0 {
		algorithms = []string{}
		for _, alg := range algorithmsParam {
			if str, ok := alg.(string); ok {
				algorithms = append(algorithms, str)
			}
		}
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	checksums, err := fs.generateChecksums(validPath, algorithms)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Checksum error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(checksums, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔐 File Checksums:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Análisis de dependencias
func (fs *FilesystemHandler) handleAnalyzeDependencies(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	language, _ := request.Params.Arguments["language"].(string)
	recursive, _ := request.Params.Arguments["recursive"].(bool)

	if path == "" {
		return nil, errors.New("path is required")
	}

	if recursive == false { // default to true if not specified
		recursive = true
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	dependencies, err := fs.analyzeDependencies(validPath, language, recursive)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Dependency analysis error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(dependencies, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("📦 Dependency Analysis:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Limpieza inteligente
func (fs *FilesystemHandler) handleSmartCleanup(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	patternsParam, _ := request.Params.Arguments["patterns"].([]interface{})
	dryRun, _ := request.Params.Arguments["dry_run"].(bool)

	if path == "" {
		return nil, errors.New("path is required")
	}

	// Default cleanup patterns
	patterns := []string{
		"*.tmp", "*.temp", "*.log", "*.bak", "*.backup",
		"node_modules", ".git", "*.pyc", "__pycache__",
		"*.class", "*.o", "*.obj", "Thumbs.db", ".DS_Store",
	}

	if len(patternsParam) > 0 {
		patterns = []string{}
		for _, p := range patternsParam {
			if str, ok := p.(string); ok {
				patterns = append(patterns, str)
			}
		}
	}

	if dryRun == false { // default to true for safety
		dryRun = true
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	result, err := fs.performSmartCleanup(validPath, patterns, dryRun)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Cleanup error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")

	mode := "🧹 Cleanup Complete"
	if result.DryRun {
		mode = "👀 Cleanup Preview (Dry Run)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%s\n\n%s", mode, string(jsonData)),
			},
		},
	}, nil
}

// Conversión de archivos
func (fs *FilesystemHandler) handleConvertFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	source, _ := request.Params.Arguments["source"].(string)
	target, _ := request.Params.Arguments["target"].(string)
	encoding, _ := request.Params.Arguments["encoding"].(string)

	if source == "" || target == "" {
		return nil, errors.New("both source and target are required")
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Source error: %v", err))
	}

	validTarget, err := fs.validatePath(target)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Target error: %v", err))
	}

	err = fs.convertFile(validSource, validTarget, encoding)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Conversion error: %v", err))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("✅ Successfully converted %s to %s", source, target),
			},
		},
	}, nil
}

// Análisis de calidad de código
func (fs *FilesystemHandler) handleCodeQualityCheck(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	language, _ := request.Params.Arguments["language"].(string)

	if path == "" {
		return nil, errors.New("path is required")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	quality, err := fs.analyzeCodeQuality(validPath, language)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Quality analysis error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(quality, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("📊 Code Quality Analysis:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Implementaciones de funciones auxiliares

func (fs *FilesystemHandler) compareFiles(path1, path2, format string) (*FileDiff, error) {
	content1, err := os.ReadFile(path1)
	if err != nil {
		return nil, err
	}

	content2, err := os.ReadFile(path2)
	if err != nil {
		return nil, err
	}

	lines1 := strings.Split(string(content1), "\n")
	lines2 := strings.Split(string(content2), "\n")

	diff := &FileDiff{
		File1: path1,
		File2: path2,
	}

	// Simple diff algorithm
	maxLen := len(lines1)
	if len(lines2) > maxLen {
		maxLen = len(lines2)
	}

	for i := 0; i < maxLen; i++ {
		var line1, line2 string
		if i < len(lines1) {
			line1 = lines1[i]
		}
		if i < len(lines2) {
			line2 = lines2[i]
		}

		if line1 == line2 {
			diff.Unchanged++
		} else if line1 != "" && line2 != "" {
			diff.Modified = append(diff.Modified, fmt.Sprintf("Line %d: '%s' -> '%s'", i+1, line1, line2))
		} else if line1 != "" {
			diff.Removed = append(diff.Removed, fmt.Sprintf("Line %d: '%s'", i+1, line1))
		} else if line2 != "" {
			diff.Added = append(diff.Added, fmt.Sprintf("Line %d: '%s'", i+1, line2))
		}
	}

	// Calculate similarity
	totalLines := float64(len(lines1) + len(lines2))
	if totalLines > 0 {
		diff.Similar = float64(diff.Unchanged*2) / totalLines * 100
	}

	return diff, nil
}

func (fs *FilesystemHandler) validateSyntax(path, language string) (*SyntaxValidation, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if language == "" {
		language = fs.detectLanguage(string(content))
	}

	validation := &SyntaxValidation{
		Language: language,
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Basic syntax validation for Go
	if language == "go" {
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			validation.Valid = false
			validation.Errors = append(validation.Errors, err.Error())
		}
	}

	// Add more language validations as needed

	return validation, nil
}

func (fs *FilesystemHandler) generateChecksums(path string, algorithms []string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	checksums := make(map[string]string)

	for _, alg := range algorithms {
		switch strings.ToLower(alg) {
		case "md5":
			hash := md5.Sum(content)
			checksums["md5"] = hex.EncodeToString(hash[:])
		case "sha1":
			hash := sha1.Sum(content)
			checksums["sha1"] = hex.EncodeToString(hash[:])
		case "sha256":
			hash := sha256.Sum256(content)
			checksums["sha256"] = hex.EncodeToString(hash[:])
		case "sha512":
			hash := sha512.Sum512(content)
			checksums["sha512"] = hex.EncodeToString(hash[:])
		}
	}

	return checksums, nil
}

func (fs *FilesystemHandler) analyzeDependencies(path, language string, recursive bool) (*DependencyAnalysis, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if language == "" {
		language = fs.detectLanguage(string(content))
	}

	analysis := &DependencyAnalysis{
		File:         path,
		Language:     language,
		Imports:      []string{},
		Dependencies: make(map[string][]string),
	}

	contentStr := string(content)

	// Extract imports based on language
	switch language {
	case "go":
		re := regexp.MustCompile(`import\s+(?:"([^"]+)"|([a-zA-Z_][a-zA-Z0-9_]*)\s+"([^"]+)")`)
		matches := re.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if match[1] != "" {
				analysis.Imports = append(analysis.Imports, match[1])
			} else if match[3] != "" {
				analysis.Imports = append(analysis.Imports, match[3])
			}
		}

	case "javascript":
		re := regexp.MustCompile(`(?:import|require)\s*\(?['"](.*?)['"]`)
		matches := re.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			analysis.Imports = append(analysis.Imports, match[1])
		}

	case "python":
		re := regexp.MustCompile(`(?:from\s+(\S+)\s+)?import\s+([^#\n]+)`)
		matches := re.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if match[1] != "" {
				analysis.Imports = append(analysis.Imports, match[1])
			}
			imports := strings.Split(match[2], ",")
			for _, imp := range imports {
				analysis.Imports = append(analysis.Imports, strings.TrimSpace(imp))
			}
		}
	}

	return analysis, nil
}

func (fs *FilesystemHandler) performSmartCleanup(path string, patterns []string, dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		DryRun:       dryRun,
		FilesRemoved: []string{},
	}

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, filepath.Base(filePath))
			if matched || strings.Contains(filePath, pattern) {
				result.TotalFiles++
				result.TotalSize += info.Size()
				result.FilesRemoved = append(result.FilesRemoved, filePath)

				if !dryRun {
					if info.IsDir() {
						os.RemoveAll(filePath)
					} else {
						os.Remove(filePath)
					}
					result.SizeFreed += info.Size()
				}
				break
			}
		}

		return nil
	})

	if dryRun {
		result.SizeFreed = result.TotalSize
	}

	return result, err
}

func (fs *FilesystemHandler) convertFile(source, target, encoding string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	// Basic conversion - normalize line endings
	contentStr := string(content)
	contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")
	contentStr = strings.ReplaceAll(contentStr, "\r", "\n")

	// Convert to target line endings based on OS
	if strings.Contains(target, ".bat") || strings.Contains(target, ".cmd") {
		contentStr = strings.ReplaceAll(contentStr, "\n", "\r\n")
	}

	return os.WriteFile(target, []byte(contentStr), 0644)
}

func (fs *FilesystemHandler) analyzeCodeQuality(path, language string) (map[string]interface{}, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if language == "" {
		language = fs.detectLanguage(string(content))
	}

	contentStr := string(content)

	quality := map[string]interface{}{
		"language":         language,
		"lines_of_code":    strings.Count(contentStr, "\n") + 1,
		"blank_lines":      strings.Count(contentStr, "\n\n"),
		"comment_ratio":    fs.calculateCommentRatio(contentStr, language),
		"avg_line_length":  fs.calculateAvgLineLength(contentStr),
		"max_line_length":  fs.calculateMaxLineLength(contentStr),
		"complexity_score": fs.calculateComplexity(contentStr, language),
	}

	return quality, nil
}

func (fs *FilesystemHandler) calculateCommentRatio(content, language string) float64 {
	lines := strings.Split(content, "\n")
	commentLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch language {
		case "go", "javascript", "java":
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
				commentLines++
			}
		case "python":
			if strings.HasPrefix(trimmed, "#") {
				commentLines++
			}
		}
	}

	if len(lines) == 0 {
		return 0
	}

	return float64(commentLines) / float64(len(lines)) * 100
}

func (fs *FilesystemHandler) calculateAvgLineLength(content string) float64 {
	lines := strings.Split(content, "\n")
	totalLength := 0

	for _, line := range lines {
		totalLength += len(line)
	}

	if len(lines) == 0 {
		return 0
	}

	return float64(totalLength) / float64(len(lines))
}

func (fs *FilesystemHandler) calculateMaxLineLength(content string) int {
	lines := strings.Split(content, "\n")
	maxLength := 0

	for _, line := range lines {
		if len(line) > maxLength {
			maxLength = len(line)
		}
	}

	return maxLength
}

func (fs *FilesystemHandler) calculateComplexity(content, language string) int {
	// Simple cyclomatic complexity calculation
	complexity := 1 // Base complexity

	// Count decision points based on language
	switch language {
	case "go", "javascript", "java":
		patterns := []string{
			`\bif\b`, `\belse\b`, `\bfor\b`, `\bwhile\b`,
			`\bswitch\b`, `\bcase\b`, `\btry\b`, `\bcatch\b`,
			`\b&&\b`, `\b\|\|\b`, `\?.*:`,
		}
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			complexity += len(re.FindAllString(content, -1))
		}

	case "python":
		patterns := []string{
			`\bif\b`, `\belif\b`, `\belse\b`, `\bfor\b`, `\bwhile\b`,
			`\btry\b`, `\bexcept\b`, `\band\b`, `\bor\b`,
		}
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			complexity += len(re.FindAllString(content, -1))
		}
	}

	return complexity
}

// Handler adicional para monitoreo de archivos
func (fs *FilesystemHandler) handleWatchFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	timeoutParam, _ := request.Params.Arguments["timeout"].(float64)

	if path == "" {
		return nil, errors.New("path is required")
	}

	timeout := 30 // default 30 seconds
	if timeoutParam > 0 {
		timeout = int(timeoutParam)
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	// Simple file monitoring implementation
	initialStat, err := os.Stat(validPath)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Cannot stat file: %v", err))
	}

	events := []FileWatchEvent{}
	startTime := time.Now()

	for time.Since(startTime).Seconds() < float64(timeout) {
		currentStat, err := os.Stat(validPath)
		if err != nil {
			events = append(events, FileWatchEvent{
				Path:      validPath,
				Event:     "deleted",
				Timestamp: time.Now(),
			})
			break
		}

		if currentStat.ModTime() != initialStat.ModTime() {
			events = append(events, FileWatchEvent{
				Path:      validPath,
				Event:     "modified",
				Timestamp: currentStat.ModTime(),
			})
			initialStat = currentStat
		}

		time.Sleep(time.Second)
	}

	jsonData, _ := json.MarshalIndent(events, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("👁️ File Watch Results (monitored for %ds):\n\n%s", timeout, string(jsonData)),
			},
		},
	}, nil
}

// Handler para extraer metadatos
func (fs *FilesystemHandler) handleExtractMetadata(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	metadataType, _ := request.Params.Arguments["type"].(string)

	if path == "" {
		return nil, errors.New("path is required")
	}

	if metadataType == "" {
		metadataType = "all"
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	metadata, err := fs.extractFileMetadata(validPath, metadataType)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Metadata extraction error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(metadata, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("📋 File Metadata:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Handler para crear archivos desde templates
func (fs *FilesystemHandler) handleCreateFromTemplate(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	templatePath, _ := request.Params.Arguments["template_path"].(string)
	outputPath, _ := request.Params.Arguments["output_path"].(string)
	variablesParam, _ := request.Params.Arguments["variables"].(map[string]interface{})

	if templatePath == "" || outputPath == "" {
		return nil, errors.New("both template_path and output_path are required")
	}

	validTemplatePath, err := fs.validatePath(templatePath)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Template path error: %v", err))
	}

	validOutputPath, err := fs.validatePath(outputPath)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Output path error: %v", err))
	}

	err = fs.createFromTemplate(validTemplatePath, validOutputPath, variablesParam)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Template processing error: %v", err))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("✅ Successfully created %s from template %s", outputPath, templatePath),
			},
		},
	}, nil
}

// Implementaciones de funciones auxiliares adicionales

func (fs *FilesystemHandler) extractFileMetadata(path, metadataType string) (map[string]interface{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"path":         path,
		"name":         filepath.Base(path),
		"extension":    filepath.Ext(path),
		"size":         info.Size(),
		"modified":     info.ModTime(),
		"permissions":  fmt.Sprintf("%o", info.Mode().Perm()),
		"is_directory": info.IsDir(),
	}

	if !info.IsDir() {
		// Detectar MIME type
		mimeType := detectMimeType(path)
		metadata["mime_type"] = mimeType

		// Para archivos de texto, agregar información adicional
		if isTextFile(mimeType) {
			content, err := os.ReadFile(path)
			if err == nil {
				contentStr := string(content)
				metadata["encoding"] = "UTF-8"
				if utf8.ValidString(contentStr) {
					metadata["encoding"] = "UTF-8"
				} else {
					metadata["encoding"] = "unknown"
				}

				metadata["lines"] = strings.Count(contentStr, "\n") + 1
				metadata["words"] = len(strings.Fields(contentStr))
				metadata["characters"] = len(contentStr)
				metadata["language"] = fs.detectLanguage(contentStr)

				// Detectar terminaciones de línea
				if strings.Contains(contentStr, "\r\n") {
					metadata["line_endings"] = "CRLF"
				} else if strings.Contains(contentStr, "\r") {
					metadata["line_endings"] = "CR"
				} else {
					metadata["line_endings"] = "LF"
				}
			}
		}

		// Calcular hashes si el archivo no es muy grande
		if info.Size() < 50*1024*1024 { // 50MB limit
			content, err := os.ReadFile(path)
			if err == nil {
				md5Hash := md5.Sum(content)
				sha256Hash := sha256.Sum256(content)
				metadata["hashes"] = map[string]string{
					"md5":    hex.EncodeToString(md5Hash[:]),
					"sha256": hex.EncodeToString(sha256Hash[:]),
				}
			}
		}
	}

	return metadata, nil
}

func (fs *FilesystemHandler) createFromTemplate(templatePath, outputPath string, variables map[string]interface{}) error {
	// Leer template
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}

	content := string(templateContent)

	// Reemplazar variables
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		valueStr := fmt.Sprintf("%v", value)
		content = strings.ReplaceAll(content, placeholder, valueStr)
	}

	// También soportar variables con formato ${VAR}
	for key, value := range variables {
		placeholder := fmt.Sprintf("${%s}", key)
		valueStr := fmt.Sprintf("%v", value)
		content = strings.ReplaceAll(content, placeholder, valueStr)
	}

	// Crear directorios padre si no existen
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Escribir archivo de salida
	return os.WriteFile(outputPath, []byte(content), 0644)
}

// Función para obtener estadísticas de directorio optimizada para Claude
func (fs *FilesystemHandler) handleDirectoryStats(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	if path == "" {
		return nil, errors.New("path is required")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	stats, err := fs.calculateDirectoryStats(validPath)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Stats calculation error: %v", err))
	}

	jsonData, _ := json.MarshalIndent(stats, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("📊 Directory Statistics:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

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

func (fs *FilesystemHandler) calculateDirectoryStats(path string) (*DirectoryStats, error) {
	stats := &DirectoryStats{
		Path:      path,
		FileTypes: make(map[string]int),
		Languages: make(map[string]int),
	}

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			stats.TotalDirectories++
		} else {
			stats.TotalFiles++
			stats.TotalSize += info.Size()

			// Encontrar el archivo más grande
			if info.Size() > stats.LargestFileSize {
				stats.LargestFileSize = info.Size()
				stats.LargestFile = filePath
			}

			// Última modificación más reciente
			if info.ModTime().After(stats.LastModified) {
				stats.LastModified = info.ModTime()
			}

			// Contar tipos de archivo
			ext := strings.ToLower(filepath.Ext(filePath))
			if ext == "" {
				ext = "[no extension]"
			}
			stats.FileTypes[ext]++

			// Detectar lenguaje para archivos de texto
			if isTextFile(detectMimeType(filePath)) && info.Size() < 1024*1024 { // 1MB limit
				content, err := os.ReadFile(filePath)
				if err == nil {
					language := fs.detectLanguage(string(content))
					stats.Languages[language]++
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Calcular tamaño promedio de archivo
	if stats.TotalFiles > 0 {
		stats.AverageFileSize = stats.TotalSize / int64(stats.TotalFiles)
	}

	return stats, nil
}

// Función para búsqueda de texto avanzada
func (fs *FilesystemHandler) handleAdvancedTextSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	pattern, _ := request.Params.Arguments["pattern"].(string)
	caseSensitive, _ := request.Params.Arguments["case_sensitive"].(bool)
	wholeWord, _ := request.Params.Arguments["whole_word"].(bool)
	includeContext, _ := request.Params.Arguments["include_context"].(bool)
	contextLines, _ := request.Params.Arguments["context_lines"].(float64)

	if path == "" || pattern == "" {
		return nil, errors.New("both path and pattern are required")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	contextLinesInt := 3
	if contextLines > 0 {
		contextLinesInt = int(contextLines)
	}

	results, err := fs.performAdvancedTextSearch(validPath, pattern, caseSensitive, wholeWord, includeContext, contextLinesInt)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Search error: %v", err))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔍 Advanced Text Search Results:\n\n%s", results),
			},
		},
	}, nil
}

type SearchMatch struct {
	File       string   `json:"file"`
	LineNumber int      `json:"line_number"`
	Line       string   `json:"line"`
	Context    []string `json:"context,omitempty"`
	MatchStart int      `json:"match_start"`
	MatchEnd   int      `json:"match_end"`
}

func (fs *FilesystemHandler) performAdvancedTextSearch(basePath, pattern string, caseSensitive, wholeWord, includeContext bool, contextLines int) (string, error) {
	var matches []SearchMatch
	var totalMatches int

	// Preparar regex
	regexPattern := pattern
	if !caseSensitive {
		regexPattern = "(?i)" + regexPattern
	}
	if wholeWord {
		regexPattern = `\b` + regexPattern + `\b`
	}

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %v", err)
	}

	err = filepath.Walk(basePath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Solo buscar en archivos de texto
		if !isTextFile(detectMimeType(filePath)) {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")

		for i, line := range lines {
			if regex.MatchString(line) {
				match := SearchMatch{
					File:       filePath,
					LineNumber: i + 1,
					Line:       line,
				}

				// Encontrar posición de la coincidencia
				loc := regex.FindStringIndex(line)
				if len(loc) >= 2 {
					match.MatchStart = loc[0]
					match.MatchEnd = loc[1]
				}

				// Agregar contexto si se solicita
				if includeContext && contextLines > 0 {
					start := i - contextLines
					if start < 0 {
						start = 0
					}
					end := i + contextLines + 1
					if end > len(lines) {
						end = len(lines)
					}

					for j := start; j < end; j++ {
						if j != i {
							match.Context = append(match.Context, fmt.Sprintf("%d: %s", j+1, lines[j]))
						}
					}
				}

				matches = append(matches, match)
				totalMatches++
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// Formatear resultados
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d matches for pattern '%s'\n\n", totalMatches, pattern))

	for _, match := range matches {
		result.WriteString(fmt.Sprintf("📄 %s:%d\n", match.File, match.LineNumber))
		result.WriteString(fmt.Sprintf("   %s\n", match.Line))

		if len(match.Context) > 0 {
			result.WriteString("   Context:\n")
			for _, ctx := range match.Context {
				result.WriteString(fmt.Sprintf("   %s\n", ctx))
			}
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

// Función handleEditFile optimizada específicamente para Claude Desktop
func (fs *FilesystemHandler) handleEditFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	// Validación ultra-robusta de parámetros para Claude Desktop
	params := make(map[string]string)
	requiredParams := []string{"path", "old_text", "new_text"}

	for _, param := range requiredParams {
		if value, exists := request.Params.Arguments[param]; exists {
			switch v := value.(type) {
			case string:
				params[param] = v
			case nil:
				return nil, fmt.Errorf(fmt.Sprintf("Parameter %s is null", param))
			default:
				// Claude Desktop a veces envía tipos wrapped - intentar conversión
				if str, ok := convertToString(v); ok {
					params[param] = str
				} else {
					return nil, fmt.Errorf(fmt.Sprintf("Parameter %s must be string, got %T: %v", param, v, v))
				}
			}
		} else {
			return nil, fmt.Errorf(fmt.Sprintf("Missing required parameter: %s", param))
		}
	}

	path := params["path"]
	oldText := params["old_text"]
	newText := params["new_text"]

	// Resolver path relativo para Claude Desktop
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Path error: %v", err))
	}

	// Verificar que es un archivo editable
	if err := fs.validateEditableFile(validPath); err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	// Crear backup automático para seguridad con Claude
	backupPath, err := fs.createBackup(validPath)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Could not create backup: %v", err))
	}
	defer func() {
		if backupPath != "" {
			os.Remove(backupPath) // Limpiar backup en caso de éxito
		}
	}()

	// Leer y procesar contenido
	content, err := os.ReadFile(validPath)
	if err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Error reading file: %v", err))
	}

	// Análisis inteligente de contenido para Claude
	analysis := fs.analyzeContent(string(content), oldText)

	// Aplicar edición con algoritmo mejorado
	result, err := fs.performIntelligentEdit(string(content), oldText, newText, analysis)
	if err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	// Escribir resultado
	if err := os.WriteFile(validPath, []byte(result.ModifiedContent), 0644); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("Error writing file: %v", err))
	}

	// Éxito - limpiar backup
	if backupPath != "" {
		os.Remove(backupPath)
		backupPath = ""
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("✅ Successfully edited %s\n📝 Changes: %d replacement(s)\n🔍 Match confidence: %s\n📊 Lines affected: %d",
					path, result.ReplacementCount, result.MatchConfidence, result.LinesAffected),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      pathToResourceURI(validPath),
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Edited: %s", validPath),
				},
			},
		},
	}, nil
}

func NewFilesystemHandler(allowedDirs []string) (*FilesystemHandler, error) {
	// Normalize and validate directories
	normalized := make([]string, 0, len(allowedDirs))
	for _, dir := range allowedDirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", dir, err)
		}

		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to access directory %s: %w",
				abs,
				err,
			)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("path is not a directory: %s", abs)
		}

		// Ensure the path ends with a separator to prevent prefix matching issues
		// For example, /tmp/foo should not match /tmp/foobar
		normalized = append(normalized, filepath.Clean(abs)+string(filepath.Separator))
	}
	return &FilesystemHandler{
		allowedDirs: normalized,
	}, nil
}

// isPathInAllowedDirs checks if a path is within any of the allowed directories
func (fs *FilesystemHandler) isPathInAllowedDirs(path string) bool {
	// Ensure path is absolute and clean
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Add trailing separator to ensure we're checking a directory or a file within a directory
	// and not a prefix match (e.g., /tmp/foo should not match /tmp/foobar)
	if !strings.HasSuffix(absPath, string(filepath.Separator)) {
		// If it's a file, we need to check its directory
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			absPath = filepath.Dir(absPath) + string(filepath.Separator)
		} else {
			absPath = absPath + string(filepath.Separator)
		}
	}

	// Check if the path is within any of the allowed directories
	for _, dir := range fs.allowedDirs {
		if strings.HasPrefix(absPath, dir) {
			return true
		}
	}
	return false
}

// buildTree builds a tree representation of the filesystem starting at the given path
func (fs *FilesystemHandler) buildTree(path string, maxDepth int, currentDepth int, followSymlinks bool) (*FileNode, error) {
	// Validate the path
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Get file info
	info, err := os.Stat(validPath)
	if err != nil {
		return nil, err
	}

	// Create the node
	node := &FileNode{
		Name:     filepath.Base(validPath),
		Path:     validPath,
		Modified: info.ModTime(),
	}

	// Set type and size
	if info.IsDir() {
		node.Type = "directory"

		// If we haven't reached the max depth, process children
		if currentDepth < maxDepth {
			// Read directory entries
			entries, err := os.ReadDir(validPath)
			if err != nil {
				return nil, err
			}

			// Process each entry
			for _, entry := range entries {
				entryPath := filepath.Join(validPath, entry.Name())

				// Handle symlinks
				if entry.Type()&os.ModeSymlink != 0 {
					if !followSymlinks {
						// Skip symlinks if not following them
						continue
					}

					// Resolve symlink
					linkDest, err := filepath.EvalSymlinks(entryPath)
					if err != nil {
						// Skip invalid symlinks
						continue
					}

					// Validate the symlink destination is within allowed directories
					if !fs.isPathInAllowedDirs(linkDest) {
						// Skip symlinks pointing outside allowed directories
						continue
					}

					entryPath = linkDest
				}

				// Recursively build child node
				childNode, err := fs.buildTree(entryPath, maxDepth, currentDepth+1, followSymlinks)
				if err != nil {
					// Skip entries with errors
					continue
				}

				// Add child to the current node
				node.Children = append(node.Children, childNode)
			}
		}
	} else {
		node.Type = "file"
		node.Size = info.Size()
	}

	return node, nil
}

func (fs *FilesystemHandler) validatePath(requestedPath string) (string, error) {
	// Always convert to absolute path first
	abs, err := filepath.Abs(requestedPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check if path is within allowed directories
	if !fs.isPathInAllowedDirs(abs) {
		return "", fmt.Errorf(
			"access denied - path outside allowed directories: %s",
			abs,
		)
	}

	// Handle symlinks
	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		// For new files, check parent directory
		parent := filepath.Dir(abs)
		realParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", fmt.Errorf("parent directory does not exist: %s", parent)
		}

		if !fs.isPathInAllowedDirs(realParent) {
			return "", fmt.Errorf(
				"access denied - parent directory outside allowed directories",
			)
		}
		return abs, nil
	}

	// Check if the real path (after resolving symlinks) is still within allowed directories
	if !fs.isPathInAllowedDirs(realPath) {
		return "", fmt.Errorf(
			"access denied - symlink target outside allowed directories",
		)
	}

	return realPath, nil
}

func (fs *FilesystemHandler) getFileStats(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		Size:        info.Size(),
		Created:     info.ModTime(), // Note: ModTime used as birth time isn't always available
		Modified:    info.ModTime(),
		Accessed:    info.ModTime(), // Note: Access time isn't always available
		IsDirectory: info.IsDir(),
		IsFile:      !info.IsDir(),
		Permissions: fmt.Sprintf("%o", info.Mode().Perm()),
	}, nil
}

func (fs *FilesystemHandler) searchFiles(
	rootPath, pattern string,
) ([]string, error) {
	var results []string
	pattern = strings.ToLower(pattern)

	err := filepath.Walk(
		rootPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors and continue
			}

			// Try to validate path
			if _, err := fs.validatePath(path); err != nil {
				return nil // Skip invalid paths
			}

			if strings.Contains(strings.ToLower(info.Name()), pattern) {
				results = append(results, path)
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// detectMimeType tries to determine the MIME type of a file
func detectMimeType(path string) string {
	// Use mimetype library for more accurate detection
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		// Fallback to extension-based detection if file can't be read
		ext := filepath.Ext(path)
		if ext != "" {
			mimeType := mime.TypeByExtension(ext)
			if mimeType != "" {
				return mimeType
			}
		}
		return "application/octet-stream" // Default
	}

	return mtype.String()
}

// isTextFile determines if a file is likely a text file based on MIME type
func isTextFile(mimeType string) bool {
	// Check for common text MIME types
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	// Common application types that are text-based
	textApplicationTypes := []string{
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/typescript",
		"application/x-typescript",
		"application/x-yaml",
		"application/yaml",
		"application/toml",
		"application/x-sh",
		"application/x-shellscript",
	}

	if slices.Contains(textApplicationTypes, mimeType) {
		return true
	}

	// Check for +format types
	if strings.Contains(mimeType, "+xml") ||
		strings.Contains(mimeType, "+json") ||
		strings.Contains(mimeType, "+yaml") {
		return true
	}

	// Common code file types that might be misidentified
	if strings.HasPrefix(mimeType, "text/x-") {
		return true
	}

	if strings.HasPrefix(mimeType, "application/x-") &&
		(strings.Contains(mimeType, "script") ||
			strings.Contains(mimeType, "source") ||
			strings.Contains(mimeType, "code")) {
		return true
	}

	return false
}

// isImageFile determines if a file is an image based on MIME type
func isImageFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") ||
		(mimeType == "application/xml" && strings.HasSuffix(strings.ToLower(mimeType), ".svg"))
}

// pathToResourceURI converts a file path to a resource URI
func pathToResourceURI(path string) string {
	return "file://" + path
}

// Resource handler
func (fs *FilesystemHandler) handleReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	// Check if it's a file:// URI
	if !strings.HasPrefix(uri, "file://") {
		return nil, fmt.Errorf("unsupported URI scheme: %s", uri)
	}

	// Extract the path from the URI
	path := strings.TrimPrefix(uri, "file://")

	// Validate the path
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Get file info
	fileInfo, err := os.Stat(validPath)
	if err != nil {
		return nil, err
	}

	// If it's a directory, return a listing
	if fileInfo.IsDir() {
		entries, err := os.ReadDir(validPath)
		if err != nil {
			return nil, err
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("Directory listing for: %s\n\n", validPath))

		for _, entry := range entries {
			entryPath := filepath.Join(validPath, entry.Name())
			entryURI := pathToResourceURI(entryPath)

			if entry.IsDir() {
				result.WriteString(fmt.Sprintf("[DIR]  %s (%s)\n", entry.Name(), entryURI))
			} else {
				info, err := entry.Info()
				if err == nil {
					result.WriteString(fmt.Sprintf("[FILE] %s (%s) - %d bytes\n",
						entry.Name(), entryURI, info.Size()))
				} else {
					result.WriteString(fmt.Sprintf("[FILE] %s (%s)\n", entry.Name(), entryURI))
				}
			}
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "text/plain",
				Text:     result.String(),
			},
		}, nil
	}

	// It's a file, determine how to handle it
	mimeType := detectMimeType(validPath)

	// Check file size
	if fileInfo.Size() > MAX_INLINE_SIZE {
		// File is too large to inline, return a reference instead
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "text/plain",
				Text:     fmt.Sprintf("File is too large to display inline (%d bytes). Use the read_file tool to access specific portions.", fileInfo.Size()),
			},
		}, nil
	}

	// Read the file content
	content, err := os.ReadFile(validPath)
	if err != nil {
		return nil, err
	}

	// Handle based on content type
	if isTextFile(mimeType) {
		// It's a text file, return as text
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: mimeType,
				Text:     string(content),
			},
		}, nil
	} else {
		// It's a binary file
		if fileInfo.Size() <= MAX_BASE64_SIZE {
			// Small enough for base64 encoding
			return []mcp.ResourceContents{
				mcp.BlobResourceContents{
					URI:      uri,
					MIMEType: mimeType,
					Blob:     base64.StdEncoding.EncodeToString(content),
				},
			}, nil
		} else {
			// Too large for base64, return a reference
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      uri,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Binary file (%s, %d bytes). Use the read_file tool to access specific portions.", mimeType, fileInfo.Size()),
				},
			}, nil
		}
	}
}

// Tool handlers

func (fs *FilesystemHandler) handleReadFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if info.IsDir() {
		// For directories, return a resource reference instead
		resourceURI := pathToResourceURI(validPath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("This is a directory. Use the resource URI to browse its contents: %s", resourceURI),
				},
				mcp.EmbeddedResource{
					Type: "resource",
					Resource: mcp.TextResourceContents{
						URI:      resourceURI,
						MIMEType: "text/plain",
						Text:     fmt.Sprintf("Directory: %s", validPath),
					},
				},
			},
		}, nil
	}

	// Determine MIME type
	mimeType := detectMimeType(validPath)

	// Check file size
	if info.Size() > MAX_INLINE_SIZE {
		// File is too large to inline, return a resource reference
		resourceURI := pathToResourceURI(validPath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("File is too large to display inline (%d bytes). Access it via resource URI: %s", info.Size(), resourceURI),
				},
				mcp.EmbeddedResource{
					Type: "resource",
					Resource: mcp.TextResourceContents{
						URI:      resourceURI,
						MIMEType: "text/plain",
						Text:     fmt.Sprintf("Large file: %s (%s, %d bytes)", validPath, mimeType, info.Size()),
					},
				},
			},
		}, nil
	}

	// Read file content
	content, err := os.ReadFile(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error reading file: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a text file
	if isTextFile(mimeType) {
		// It's a text file, return as text
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: string(content),
				},
			},
		}, nil
	} else if isImageFile(mimeType) {
		// It's an image file, return as image content
		if info.Size() <= MAX_BASE64_SIZE {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Image file: %s (%s, %d bytes)", validPath, mimeType, info.Size()),
					},
					mcp.ImageContent{
						Type:     "image",
						Data:     base64.StdEncoding.EncodeToString(content),
						MIMEType: mimeType,
					},
				},
			}, nil
		} else {
			// Too large for base64, return a reference
			resourceURI := pathToResourceURI(validPath)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Image file is too large to display inline (%d bytes). Access it via resource URI: %s", info.Size(), resourceURI),
					},
					mcp.EmbeddedResource{
						Type: "resource",
						Resource: mcp.TextResourceContents{
							URI:      resourceURI,
							MIMEType: "text/plain",
							Text:     fmt.Sprintf("Large image: %s (%s, %d bytes)", validPath, mimeType, info.Size()),
						},
					},
				},
			}, nil
		}
	} else {
		// It's another type of binary file
		resourceURI := pathToResourceURI(validPath)

		if info.Size() <= MAX_BASE64_SIZE {
			// Small enough for base64 encoding
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Binary file: %s (%s, %d bytes)", validPath, mimeType, info.Size()),
					},
					mcp.EmbeddedResource{
						Type: "resource",
						Resource: mcp.BlobResourceContents{
							URI:      resourceURI,
							MIMEType: mimeType,
							Blob:     base64.StdEncoding.EncodeToString(content),
						},
					},
				},
			}, nil
		} else {
			// Too large for base64, return a reference
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Binary file: %s (%s, %d bytes). Access it via resource URI: %s", validPath, mimeType, info.Size(), resourceURI),
					},
					mcp.EmbeddedResource{
						Type: "resource",
						Resource: mcp.TextResourceContents{
							URI:      resourceURI,
							MIMEType: "text/plain",
							Text:     fmt.Sprintf("Binary file: %s (%s, %d bytes)", validPath, mimeType, info.Size()),
						},
					},
				},
			}, nil
		}
	}
}

func (fs *FilesystemHandler) handleWriteFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}
	content, ok := request.Params.Arguments["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	if info, err := os.Stat(validPath); err == nil && info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: Cannot write to a directory",
				},
			},
			IsError: true,
		}, nil
	}

	// Create parent directories if they don't exist
	parentDir := filepath.Dir(validPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating parent directories: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if err := os.WriteFile(validPath, []byte(content), 0644); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error writing file: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Get file info for the response
	info, err := os.Stat(validPath)
	if err != nil {
		// File was written but we couldn't get info
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Successfully wrote to %s", path),
				},
			},
		}, nil
	}

	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully wrote %d bytes to %s", info.Size(), path),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("File: %s (%d bytes)", validPath, info.Size()),
				},
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleListDirectory(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: Path is not a directory",
				},
			},
			IsError: true,
		}, nil
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error reading directory: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory listing for: %s\n\n", validPath))

	for _, entry := range entries {
		entryPath := filepath.Join(validPath, entry.Name())
		resourceURI := pathToResourceURI(entryPath)

		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s (%s)\n", entry.Name(), resourceURI))
		} else {
			info, err := entry.Info()
			if err == nil {
				result.WriteString(fmt.Sprintf("[FILE] %s (%s) - %d bytes\n",
					entry.Name(), resourceURI, info.Size()))
			} else {
				result.WriteString(fmt.Sprintf("[FILE] %s (%s)\n", entry.Name(), resourceURI))
			}
		}
	}

	// Return both text content and embedded resource
	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result.String(),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Directory: %s", validPath),
				},
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleCreateDirectory(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if path already exists
	if info, err := os.Stat(validPath); err == nil {
		if info.IsDir() {
			resourceURI := pathToResourceURI(validPath)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Directory already exists: %s", path),
					},
					mcp.EmbeddedResource{
						Type: "resource",
						Resource: mcp.TextResourceContents{
							URI:      resourceURI,
							MIMEType: "text/plain",
							Text:     fmt.Sprintf("Directory: %s", validPath),
						},
					},
				},
			}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: Path exists but is not a directory: %s", path),
				},
			},
			IsError: true,
		}, nil
	}

	if err := os.MkdirAll(validPath, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating directory: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully created directory %s", path),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Directory: %s", validPath),
				},
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleCopyFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	source, ok := request.Params.Arguments["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source must be a string")
	}
	destination, ok := request.Params.Arguments["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination must be a string")
	}

	// Handle empty or relative paths for source
	if source == "." || source == "./" {
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		source = cwd
	}
	if destination == "." || destination == "./" {
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		destination = cwd
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error with source path: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if source exists
	srcInfo, err := os.Stat(validSource)
	if os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: Source does not exist: %s", source),
				},
			},
			IsError: true,
		}, nil
	} else if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error accessing source: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	validDest, err := fs.validatePath(destination)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error with destination path: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Create parent directory for destination if it doesn't exist
	destDir := filepath.Dir(validDest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating destination directory: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Perform the copy operation based on whether source is a file or directory
	if srcInfo.IsDir() {
		// It's a directory, copy recursively
		if err := copyDir(validSource, validDest); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error copying directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	} else {
		// It's a file, copy directly
		if err := copyFile(validSource, validDest); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error copying file: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
	}

	resourceURI := pathToResourceURI(validDest)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf(
					"Successfully copied %s to %s",
					source,
					destination,
				),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Copied file: %s", validDest),
				},
			},
		},
	}, nil
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy the contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Get source file mode
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Set the same file mode on destination
	return os.Chmod(dst, sourceInfo.Mode())
}

// copyDir recursively copies a directory tree from src to dst
func copyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create the destination directory with the same permissions
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Handle symlinks
		if entry.Type()&os.ModeSymlink != 0 {
			// For simplicity, we'll skip symlinks in this implementation
			continue
		}

		// Recursively copy subdirectories or copy files
		if entry.IsDir() {
			if err = copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (fs *FilesystemHandler) handleMoveFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	source, ok := request.Params.Arguments["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source must be a string")
	}
	destination, ok := request.Params.Arguments["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination must be a string")
	}

	// Handle empty or relative paths for source
	if source == "." || source == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		source = cwd
	}

	// Handle empty or relative paths for destination
	if destination == "." || destination == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		destination = cwd
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error with source path: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if source exists
	if _, err := os.Stat(validSource); os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: Source does not exist: %s", source),
				},
			},
			IsError: true,
		}, nil
	}

	validDest, err := fs.validatePath(destination)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error with destination path: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Create parent directory for destination if it doesn't exist
	destDir := filepath.Dir(validDest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error creating destination directory: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if err := os.Rename(validSource, validDest); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error moving file: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	resourceURI := pathToResourceURI(validDest)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf(
					"Successfully moved %s to %s",
					source,
					destination,
				),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Moved file: %s", validDest),
				},
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleSearchFiles(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}
	pattern, ok := request.Params.Arguments["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: Search path must be a directory",
				},
			},
			IsError: true,
		}, nil
	}

	results, err := fs.searchFiles(validPath, pattern)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error searching files: %v",
						err),
				},
			},
			IsError: true,
		}, nil
	}

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("No files found matching pattern '%s' in %s", pattern, path),
				},
			},
		}, nil
	}

	// Format results with resource URIs
	var formattedResults strings.Builder
	formattedResults.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for _, result := range results {
		resourceURI := pathToResourceURI(result)
		info, err := os.Stat(result)
		if err == nil {
			if info.IsDir() {
				formattedResults.WriteString(fmt.Sprintf("[DIR]  %s (%s)\n", result, resourceURI))
			} else {
				formattedResults.WriteString(fmt.Sprintf("[FILE] %s (%s) - %d bytes\n",
					result, resourceURI, info.Size()))
			}
		} else {
			formattedResults.WriteString(fmt.Sprintf("%s (%s)\n", result, resourceURI))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: formattedResults.String(),
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleTree(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	// Extract depth parameter (optional, default: 3)
	depth := 3 // Default value
	if depthParam, ok := request.Params.Arguments["depth"]; ok {
		if d, ok := depthParam.(float64); ok {
			depth = int(d)
		}
	}

	// Extract follow_symlinks parameter (optional, default: false)
	followSymlinks := false // Default value
	if followParam, ok := request.Params.Arguments["follow_symlinks"]; ok {
		if f, ok := followParam.(bool); ok {
			followSymlinks = f
		}
	}

	// Validate the path is within allowed directories
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: The specified path is not a directory",
				},
			},
			IsError: true,
		}, nil
	}

	// Build the tree structure
	tree, err := fs.buildTree(validPath, depth, 0, followSymlinks)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error building directory tree: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error generating JSON: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Create resource URI for the directory
	resourceURI := pathToResourceURI(validPath)

	// Return the result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Directory tree for %s (max depth: %d):\n\n%s", validPath, depth, string(jsonData)),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "application/json",
					Text:     string(jsonData),
				},
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleGetFileInfo(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	info, err := fs.getFileStats(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error getting file info: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Get MIME type for files
	mimeType := "directory"
	if info.IsFile {
		mimeType = detectMimeType(validPath)
	}

	resourceURI := pathToResourceURI(validPath)

	// Determine file type text
	var fileTypeText string
	if info.IsDirectory {
		fileTypeText = "Directory"
	} else {
		fileTypeText = "File"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf(
					"File information for: %s\n\nSize: %d bytes\nCreated: %s\nModified: %s\nAccessed: %s\nIsDirectory: %v\nIsFile: %v\nPermissions: %s\nMIME Type: %s\nResource URI: %s",
					validPath,
					info.Size,
					info.Created.Format(time.RFC3339),
					info.Modified.Format(time.RFC3339),
					info.Accessed.Format(time.RFC3339),
					info.IsDirectory,
					info.IsFile,
					info.Permissions,
					mimeType,
					resourceURI,
				),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "text/plain",
					Text: fmt.Sprintf("%s: %s (%s, %d bytes)",
						fileTypeText,
						validPath,
						mimeType,
						info.Size),
				},
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleReadMultipleFiles(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	pathsParam, ok := request.Params.Arguments["paths"]
	if !ok {
		return nil, fmt.Errorf("paths parameter is required")
	}

	// Convert the paths parameter to a string slice
	pathsSlice, ok := pathsParam.([]any)
	if !ok {
		return nil, fmt.Errorf("paths must be an array of strings")
	}

	if len(pathsSlice) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "No files specified to read",
				},
			},
			IsError: true,
		}, nil
	}

	// Maximum number of files to read in a single request
	const maxFiles = 50
	if len(pathsSlice) > maxFiles {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Too many files requested. Maximum is %d files per request.", maxFiles),
				},
			},
			IsError: true,
		}, nil
	}

	// Process each file
	var results []mcp.Content
	for _, pathInterface := range pathsSlice {
		path, ok := pathInterface.(string)
		if !ok {
			return nil, fmt.Errorf("each path must be a string")
		}

		// Handle empty or relative paths like "." or "./" by converting to absolute path
		if path == "." || path == "./" {
			// Get current working directory
			cwd, err := os.Getwd()
			if err != nil {
				results = append(results, mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error resolving current directory for path '%s': %v", path, err),
				})
				continue
			}
			path = cwd
		}

		validPath, err := fs.validatePath(path)
		if err != nil {
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error with path '%s': %v", path, err),
			})
			continue
		}

		// Check if it's a directory
		info, err := os.Stat(validPath)
		if err != nil {
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error accessing '%s': %v", path, err),
			})
			continue
		}

		if info.IsDir() {
			// For directories, return a resource reference instead
			resourceURI := pathToResourceURI(validPath)
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("'%s' is a directory. Use list_directory tool or resource URI: %s", path, resourceURI),
			})
			continue
		}

		// Determine MIME type
		mimeType := detectMimeType(validPath)

		// Check file size
		if info.Size() > MAX_INLINE_SIZE {
			// File is too large to inline, return a resource reference
			resourceURI := pathToResourceURI(validPath)
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("File '%s' is too large to display inline (%d bytes). Access it via resource URI: %s",
					path, info.Size(), resourceURI),
			})
			continue
		}

		// Read file content
		content, err := os.ReadFile(validPath)
		if err != nil {
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error reading file '%s': %v", path, err),
			})
			continue
		}

		// Add file header
		results = append(results, mcp.TextContent{
			Type: "text",
			Text: fmt.Sprintf("--- File: %s ---", path),
		})

		// Check if it's a text file
		if isTextFile(mimeType) {
			// It's a text file, return as text
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: string(content),
			})
		} else if isImageFile(mimeType) {
			// It's an image file, return as image content
			if info.Size() <= MAX_BASE64_SIZE {
				results = append(results, mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Image file: %s (%s, %d bytes)", path, mimeType, info.Size()),
				})
				results = append(results, mcp.ImageContent{
					Type:     "image",
					Data:     base64.StdEncoding.EncodeToString(content),
					MIMEType: mimeType,
				})
			} else {
				// Too large for base64, return a reference
				resourceURI := pathToResourceURI(validPath)
				results = append(results, mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Image file '%s' is too large to display inline (%d bytes). Access it via resource URI: %s",
						path, info.Size(), resourceURI),
				})
			}
		} else {
			// It's another type of binary file
			resourceURI := pathToResourceURI(validPath)

			if info.Size() <= MAX_BASE64_SIZE {
				// Small enough for base64 encoding
				results = append(results, mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Binary file: %s (%s, %d bytes)", path, mimeType, info.Size()),
				})
				results = append(results, mcp.EmbeddedResource{
					Type: "resource",
					Resource: mcp.BlobResourceContents{
						URI:      resourceURI,
						MIMEType: mimeType,
						Blob:     base64.StdEncoding.EncodeToString(content),
					},
				})
			} else {
				// Too large for base64, return a reference
				results = append(results, mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Binary file '%s' (%s, %d bytes). Access it via resource URI: %s",
						path, mimeType, info.Size(), resourceURI),
				})
			}
		}
	}

	return &mcp.CallToolResult{
		Content: results,
	}, nil
}

func (fs *FilesystemHandler) handleDeleteFile(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := request.Params.Arguments["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if path exists
	info, err := os.Stat(validPath)
	if os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: Path does not exist: %s", path),
				},
			},
			IsError: true,
		}, nil
	} else if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error accessing path: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Extract recursive parameter (optional, default: false)
	recursive := false
	if recursiveParam, ok := request.Params.Arguments["recursive"]; ok {
		if r, ok := recursiveParam.(bool); ok {
			recursive = r
		}
	}

	// Check if it's a directory and handle accordingly
	if info.IsDir() {
		if !recursive {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error: %s is a directory. Use recursive=true to delete directories.", path),
					},
				},
				IsError: true,
			}, nil
		}

		// It's a directory and recursive is true, so remove it
		if err := os.RemoveAll(validPath); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error deleting directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Successfully deleted directory %s", path),
				},
			},
		}, nil
	}

	// It's a file, delete it
	if err := os.Remove(validPath); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error deleting file: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully deleted file %s", path),
			},
		},
	}, nil
}

func (fs *FilesystemHandler) handleListAllowedDirectories(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	// Remove the trailing separator for display purposes
	displayDirs := make([]string, len(fs.allowedDirs))
	for i, dir := range fs.allowedDirs {
		displayDirs[i] = strings.TrimSuffix(dir, string(filepath.Separator))
	}

	var result strings.Builder
	result.WriteString("Allowed directories:\n\n")

	for _, dir := range displayDirs {
		resourceURI := pathToResourceURI(dir)
		result.WriteString(fmt.Sprintf("%s (%s)\n", dir, resourceURI))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: result.String(),
			},
		},
	}, nil
}

// Función para detectar lenguaje (básica)
func (fs *FilesystemHandler) detectLanguage(content string) string {
	if strings.Contains(content, "package ") && strings.Contains(content, "import (") {
		return "go"
	}
	if strings.Contains(content, "function ") || strings.Contains(content, "const ") {
		return "javascript"
	}
	if strings.Contains(content, "def ") || strings.Contains(content, "import ") {
		return "python"
	}
	return "unknown"
}

// Función para convertir a string
func convertToString(v interface{}) (string, bool) {
	if str, ok := v.(string); ok {
		return str, true
	}
	return "", false
}

// Función para validar archivo editable
func (fs *FilesystemHandler) validateEditableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("cannot edit directory")
	}
	return nil
}

// Función para crear backup
func (fs *FilesystemHandler) createBackup(path string) (string, error) {
	backupPath := path + ".backup"
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(backupPath, content, 0644)
	return backupPath, err
}

// Estructura para resultado de edición
type EditResult struct {
	ModifiedContent  string
	ReplacementCount int
	MatchConfidence  string
	LinesAffected    int
}

// Función para analizar contenido
func (fs *FilesystemHandler) analyzeContent(content, oldText string) interface{} {
	return nil // Implementación básica
}

// Función para edición inteligente
func (fs *FilesystemHandler) performIntelligentEdit(content, oldText, newText string, analysis interface{}) (*EditResult, error) {
	newContent := strings.ReplaceAll(content, oldText, newText)
	count := strings.Count(content, oldText)

	return &EditResult{
		ModifiedContent:  newContent,
		ReplacementCount: count,
		MatchConfidence:  "high",
		LinesAffected:    count,
	}, nil
}

// HANDLERS FALTANTES PARA COMPLETAR LA FUNCIONALIDAD

// Búsqueda inteligente de contenido
func (fs *FilesystemHandler) handleSmartSearch(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	pattern, _ := request.Params.Arguments["pattern"].(string)
	includeContent, _ := request.Params.Arguments["include_content"].(bool)
	fileTypesParam, _ := request.Params.Arguments["file_types"].([]interface{})

	if path == "" || pattern == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: path and pattern are required",
				},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Path error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Convertir tipos de archivo
	fileTypes := []string{}
	for _, ft := range fileTypesParam {
		if str, ok := ft.(string); ok {
			fileTypes = append(fileTypes, str)
		}
	}

	results, err := fs.performSmartSearch(validPath, pattern, includeContent, fileTypes)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Search error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: results,
			},
		},
	}, nil
}

// Detectar archivos duplicados
func (fs *FilesystemHandler) handleFindDuplicates(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	if path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: path is required",
				},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Path error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	duplicates, err := fs.findDuplicateFiles(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Duplicate detection error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(duplicates, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔍 Duplicate Files Found:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Análisis de estructura de proyecto
func (fs *FilesystemHandler) handleAnalyzeProject(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	if path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: path is required",
				},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Path error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	structure, err := fs.analyzeProjectStructure(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Project analysis error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(structure, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🏗️ Project Structure Analysis:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Operaciones en lote
func (fs *FilesystemHandler) handleBatchOperations(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	operationsParam, ok := request.Params.Arguments["operations"].([]interface{})
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: operations must be an array",
				},
			},
			IsError: true,
		}, nil
	}

	results := []string{}
	errors := []string{}

	for i, op := range operationsParam {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			errors = append(errors, fmt.Sprintf("Operation %d: invalid format", i))
			continue
		}

		result, err := fs.processBatchOperation(opMap)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Operation %d: %v", i, err))
		} else {
			results = append(results, result)
		}
	}

	response := fmt.Sprintf("🔄 Batch Operations Completed\n✅ Successful: %d\n❌ Failed: %d\n\nResults:\n%s",
		len(results), len(errors), strings.Join(results, "\n"))

	if len(errors) > 0 {
		response += fmt.Sprintf("\n\nErrors:\n%s", strings.Join(errors, "\n"))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: response,
			},
		},
	}, nil
}

// IMPLEMENTACIONES DE FUNCIONES AUXILIARES ADICIONALES

func (fs *FilesystemHandler) performSmartSearch(basePath, pattern string, includeContent bool, fileTypes []string) (string, error) {
	var results []string
	var totalMatches int

	// Crear mapa de tipos permitidos
	allowedTypes := make(map[string]bool)
	for _, ft := range fileTypes {
		allowedTypes[strings.ToLower(ft)] = true
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Filtrar por tipo de archivo si se especifica
		if len(allowedTypes) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if !allowedTypes[ext] {
				return nil
			}
		}

		// Buscar en nombre de archivo
		if matched, _ := regexp.MatchString("(?i)"+pattern, filepath.Base(path)); matched {
			results = append(results, fmt.Sprintf("📄 %s (filename match)", path))
			totalMatches++
		}

		// Buscar en contenido si es archivo de texto e includeContent está habilitado
		if includeContent && !info.IsDir() && isTextFile(detectMimeType(path)) {
			content, err := os.ReadFile(path)
			if err == nil {
				if matched, _ := regexp.MatchString("(?i)"+pattern, string(content)); matched {
					results = append(results, fmt.Sprintf("📝 %s (content match)", path))
					totalMatches++
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	response := fmt.Sprintf("🔍 Smart Search Results for '%s'\n📊 Total matches: %d\n\n", pattern, totalMatches)
	if len(results) > 0 {
		response += strings.Join(results, "\n")
	} else {
		response += "No matches found"
	}

	return response, nil
}

func (fs *FilesystemHandler) findDuplicateFiles(basePath string) (map[string][]DuplicateFile, error) {
	fileHashes := make(map[string][]DuplicateFile)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		hash := fmt.Sprintf("%x", md5.Sum(content))

		fileHashes[hash] = append(fileHashes[hash], DuplicateFile{
			Path: path,
			Hash: hash,
			Size: info.Size(),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Filtrar solo duplicados
	duplicates := make(map[string][]DuplicateFile)
	for hash, files := range fileHashes {
		if len(files) > 1 {
			duplicates[hash] = files
		}
	}

	return duplicates, nil
}

func (fs *FilesystemHandler) analyzeProjectStructure(basePath string) (*ProjectStructure, error) {
	structure := &ProjectStructure{
		Root:      basePath,
		Languages: make(map[string]int),
		FileTypes: make(map[string]int),
		Structure: make(map[string][]string),
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relativePath := strings.TrimPrefix(path, basePath)
		relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))

		if info.IsDir() {
			structure.Directories = append(structure.Directories, relativePath)
			structure.Structure[relativePath] = []string{}
		} else {
			structure.TotalFiles++
			structure.TotalSize += info.Size()

			// Analizar tipo de archivo
			ext := strings.ToLower(filepath.Ext(path))
			structure.FileTypes[ext]++

			// Detectar lenguaje
			if isTextFile(detectMimeType(path)) {
				content, err := os.ReadFile(path)
				if err == nil {
					lang := fs.detectLanguage(string(content))
					structure.Languages[lang]++
				}
			}

			// Agregar a estructura
			dir := filepath.Dir(relativePath)
			if dir == "." {
				dir = ""
			}
			structure.Structure[dir] = append(structure.Structure[dir], filepath.Base(path))
		}

		return nil
	})

	return structure, err
}

func (fs *FilesystemHandler) processBatchOperation(operation map[string]interface{}) (string, error) {
	opType, ok := operation["type"].(string)
	if !ok {
		return "", errors.New("operation type required")
	}

	switch opType {
	case "rename":
		from, _ := operation["from"].(string)
		to, _ := operation["to"].(string)
		if from == "" || to == "" {
			return "", errors.New("from and to paths required for rename")
		}

		validFrom, err := fs.validatePath(from)
		if err != nil {
			return "", err
		}

		validTo, err := fs.validatePath(to)
		if err != nil {
			return "", err
		}

		err = os.Rename(validFrom, validTo)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Renamed: %s → %s", from, to), nil

	case "delete":
		path, _ := operation["path"].(string)
		if path == "" {
			return "", errors.New("path required for delete")
		}

		validPath, err := fs.validatePath(path)
		if err != nil {
			return "", err
		}

		err = os.Remove(validPath)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Deleted: %s", path), nil

	case "copy":
		from, _ := operation["from"].(string)
		to, _ := operation["to"].(string)
		if from == "" || to == "" {
			return "", errors.New("from and to paths required for copy")
		}

		validFrom, err := fs.validatePath(from)
		if err != nil {
			return "", err
		}

		validTo, err := fs.validatePath(to)
		if err != nil {
			return "", err
		}

		err = copyFile(validFrom, validTo)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Copied: %s → %s", from, to), nil

	default:
		return "", fmt.Errorf("unsupported operation type: %s", opType)
	}
}

// Alias para handleBatchOperations (referenciado en server.go como handleBatchEdit)
func (fs *FilesystemHandler) handleBatchEdit(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return fs.handleBatchOperations(ctx, request)
}

// Análisis de rendimiento de archivos
func (fs *FilesystemHandler) handlePerformanceAnalysis(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	operation, _ := request.Params.Arguments["operation"].(string)

	if path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: path is required",
				},
			},
			IsError: true,
		}, nil
	}

	if operation == "" {
		operation = "all"
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Path error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	performance, err := fs.analyzePerformance(validPath, operation)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Performance analysis error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(performance, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🚀 Performance Analysis:\n\n%s", string(jsonData)),
			},
		},
	}, nil
}

// Generador de reportes
func (fs *FilesystemHandler) handleGenerateReport(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	format, _ := request.Params.Arguments["format"].(string)
	output, _ := request.Params.Arguments["output"].(string)
	sectionsParam, _ := request.Params.Arguments["sections"].([]interface{})

	if path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: path is required",
				},
			},
			IsError: true,
		}, nil
	}

	if format == "" {
		format = "json"
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Path error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	sections := []string{"overview", "files", "quality", "dependencies"}
	if len(sectionsParam) > 0 {
		sections = make([]string, 0, len(sectionsParam))
		for _, s := range sectionsParam {
			if str, ok := s.(string); ok {
				sections = append(sections, str)
			}
		}
	}

	report, err := fs.generateComprehensiveReport(validPath, format, sections)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Report generation error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Guardar reporte si se especifica output
	if output != "" {
		validOutput, err := fs.validatePath(output)
		if err == nil {
			os.WriteFile(validOutput, []byte(report), 0644)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("📋 Comprehensive Report:\n\n%s", report),
			},
		},
	}, nil
}

// Sincronización inteligente
func (fs *FilesystemHandler) handleSmartSync(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	source, _ := request.Params.Arguments["source"].(string)
	target, _ := request.Params.Arguments["target"].(string)
	mode, _ := request.Params.Arguments["mode"].(string)
	excludePatternsParam, _ := request.Params.Arguments["exclude_patterns"].([]interface{})

	if source == "" || target == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: both source and target are required",
				},
			},
			IsError: true,
		}, nil
	}

	if mode == "" {
		mode = "preview"
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Source error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	validTarget, err := fs.validatePath(target)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Target error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	excludePatterns := []string{}
	for _, p := range excludePatternsParam {
		if str, ok := p.(string); ok {
			excludePatterns = append(excludePatterns, str)
		}
	}

	syncResult, err := fs.performSmartSync(validSource, validTarget, mode, excludePatterns)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Sync error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔄 Smart Sync Results:\n\n%s", syncResult),
			},
		},
	}, nil
}

// Herramienta de refactoring asistido
func (fs *FilesystemHandler) handleAssistRefactor(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {

	path, _ := request.Params.Arguments["path"].(string)
	operation, _ := request.Params.Arguments["operation"].(string)
	target, _ := request.Params.Arguments["target"].(string)
	optionsParam, _ := request.Params.Arguments["options"].(map[string]interface{})

	if path == "" || operation == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: both path and operation are required",
				},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Path error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	refactorResult, err := fs.performRefactoring(validPath, operation, target, optionsParam)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("❌ Error: Refactoring error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("🔧 Refactoring Assistant Results:\n\n%s", refactorResult),
			},
		},
	}, nil
}

// FUNCIONES AUXILIARES ADICIONALES para los nuevos handlers

func (fs *FilesystemHandler) analyzePerformance(path, operation string) (map[string]interface{}, error) {
	performance := make(map[string]interface{})

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if operation == "read" || operation == "all" {
		// Benchmark de lectura
		start := time.Now()
		if !info.IsDir() {
			_, err := os.ReadFile(path)
			if err == nil {
				readTime := time.Since(start)
				performance["read_time_ms"] = readTime.Milliseconds()
				if readTime.Nanoseconds() > 0 {
					performance["read_speed_mbps"] = float64(info.Size()) / float64(readTime.Nanoseconds()) * 1000
				}
			} else {
				return nil, err
			}
		}
	}

	if operation == "list" || operation == "all" {
		if info.IsDir() {
			start := time.Now()
			entries, err := os.ReadDir(path)
			listTime := time.Since(start)
			if err == nil {
				performance["list_time_ms"] = listTime.Milliseconds()
				performance["entries_count"] = len(entries)
				if listTime.Seconds() > 0 {
					performance["list_speed_entries_per_sec"] = float64(len(entries)) / listTime.Seconds()
				}
			} else {
				return nil, err
			}
		}
	}

	performance["file_size"] = info.Size()
	performance["path"] = path
	performance["timestamp"] = time.Now()

	return performance, nil
}

func (fs *FilesystemHandler) generateComprehensiveReport(path, format string, sections []string) (string, error) {
	report := make(map[string]interface{})

	for _, section := range sections {
		switch section {
		case "overview":
			stats, err := fs.calculateDirectoryStats(path)
			if err == nil {
				report["overview"] = stats
			}

		case "files":
			files := []map[string]interface{}{}
			filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					fileInfo := map[string]interface{}{
						"path":     filePath,
						"size":     info.Size(),
						"modified": info.ModTime(),
						"type":     detectMimeType(filePath),
					}
					files = append(files, fileInfo)
				}
				return nil
			})
			report["files"] = files

		case "quality":
			quality := map[string]interface{}{
				"total_lines": 0,
				"languages":   make(map[string]int),
			}
			report["quality"] = quality

		case "dependencies":
			deps := map[string]interface{}{
				"imports": []string{},
				"exports": []string{},
			}
			report["dependencies"] = deps
		}
	}

	report["generated_at"] = time.Now()
	report["path"] = path
	report["format"] = format

	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		return string(data), err
	case "markdown":
		return fs.formatMarkdownReport(report), nil
	default:
		data, err := json.MarshalIndent(report, "", "  ")
		return string(data), err
	}
}

func (fs *FilesystemHandler) formatMarkdownReport(report map[string]interface{}) string {
	var md strings.Builder

	md.WriteString("# File System Analysis Report\n\n")
	md.WriteString(fmt.Sprintf("Generated: %v\n\n", report["generated_at"]))

	if overview, ok := report["overview"]; ok {
		md.WriteString("## Overview\n\n")
		if stats, ok := overview.(*DirectoryStats); ok {
			md.WriteString(fmt.Sprintf("- **Total Files:** %d\n", stats.TotalFiles))
			md.WriteString(fmt.Sprintf("- **Total Directories:** %d\n", stats.TotalDirectories))
			md.WriteString(fmt.Sprintf("- **Total Size:** %d bytes\n", stats.TotalSize))
			md.WriteString(fmt.Sprintf("- **Average File Size:** %d bytes\n", stats.AverageFileSize))
			md.WriteString("\n")
		}
	}

	if files, ok := report["files"].([]map[string]interface{}); ok {
		md.WriteString("## Files\n\n")
		maxFiles := 10
		if len(files) < maxFiles {
			maxFiles = len(files)
		}
		for i := 0; i < maxFiles; i++ {
			file := files[i]
			md.WriteString(fmt.Sprintf("- **%s** (%v bytes)\n", file["path"], file["size"]))
		}
		md.WriteString("\n")
	}

	return md.String()
}

func (fs *FilesystemHandler) performSmartSync(source, target, mode string, excludePatterns []string) (string, error) {
	syncActions := []string{}

	err := filepath.Walk(source, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Verificar patrones de exclusión
		for _, pattern := range excludePatterns {
			matched, _ := filepath.Match(pattern, filepath.Base(srcPath))
			if matched {
				return nil
			}
		}

		relPath, _ := filepath.Rel(source, srcPath)
		targetPath := filepath.Join(target, relPath)

		if info.IsDir() {
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				syncActions = append(syncActions, fmt.Sprintf("CREATE DIR: %s", targetPath))
				if mode != "preview" {
					os.MkdirAll(targetPath, info.Mode())
				}
			}
		} else {
			targetInfo, err := os.Stat(targetPath)
			if os.IsNotExist(err) {
				syncActions = append(syncActions, fmt.Sprintf("COPY: %s -> %s", srcPath, targetPath))
				if mode != "preview" {
					copyFile(srcPath, targetPath)
				}
			} else if targetInfo.ModTime().Before(info.ModTime()) {
				syncActions = append(syncActions, fmt.Sprintf("UPDATE: %s", targetPath))
				if mode != "preview" {
					copyFile(srcPath, targetPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	result := fmt.Sprintf("Sync Mode: %s\nActions planned: %d\n\n", mode, len(syncActions))
	for _, action := range syncActions {
		result += action + "\n"
	}

	return result, nil
}

func (fs *FilesystemHandler) performRefactoring(path, operation, target string, options map[string]interface{}) (string, error) {
	suggestions := []string{}

	switch operation {
	case "rename":
		if target == "" {
			return "", errors.New("target name required for rename operation")
		}

		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}

		newPath := filepath.Join(filepath.Dir(path), target)

		if info.IsDir() {
			suggestions = append(suggestions, fmt.Sprintf("RENAME DIRECTORY: %s -> %s", path, newPath))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("RENAME FILE: %s -> %s", path, newPath))

			// Buscar referencias al archivo
			if isTextFile(detectMimeType(path)) {
				refs, err := fs.findFileReferences(path)
				if err == nil && len(refs) > 0 {
					suggestions = append(suggestions, "REFERENCES FOUND:")
					for _, ref := range refs {
						suggestions = append(suggestions, fmt.Sprintf("  - %s", ref))
					}
				}
			}
		}

	case "extract":
		suggestions = append(suggestions, fmt.Sprintf("EXTRACT operation suggested for: %s", path))
		if target != "" {
			suggestions = append(suggestions, fmt.Sprintf("Extract to: %s", target))
		}

	case "inline":
		suggestions = append(suggestions, fmt.Sprintf("INLINE operation suggested for: %s", path))

	case "move":
		if target == "" {
			return "", errors.New("target directory required for move operation")
		}
		suggestions = append(suggestions, fmt.Sprintf("MOVE: %s -> %s", path, target))

	default:
		return "", fmt.Errorf("unsupported refactoring operation: %s", operation)
	}

	// Agregar sugerencias de seguridad
	suggestions = append(suggestions, "\nRECOMMENDATIONS:")
	suggestions = append(suggestions, "- Create backup before applying changes")
	suggestions = append(suggestions, "- Test changes in development environment")
	suggestions = append(suggestions, "- Update documentation if needed")

	return strings.Join(suggestions, "\n"), nil
}

func (fs *FilesystemHandler) findFileReferences(filePath string) ([]string, error) {
	references := []string{}
	fileName := filepath.Base(filePath)
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Buscar en el directorio padre
	parentDir := filepath.Dir(filePath)

	err := filepath.Walk(parentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || path == filePath {
			return nil
		}

		if !isTextFile(detectMimeType(path)) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		// Buscar referencias por nombre de archivo
		if strings.Contains(contentStr, fileName) || strings.Contains(contentStr, baseName) {
			references = append(references, path)
		}

		return nil
	})

	return references, err
}
