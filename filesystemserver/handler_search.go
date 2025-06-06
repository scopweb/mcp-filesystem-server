package filesystemserver

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleSmartSearch - Búsqueda inteligente con regex y filtros
func (fs *FilesystemHandler) handleSmartSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

// handleAdvancedTextSearch - Búsqueda avanzada de texto con contexto
func (fs *FilesystemHandler) handleAdvancedTextSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := request.Params.Arguments["path"].(string)
	pattern, _ := request.Params.Arguments["pattern"].(string)
	caseSensitive, _ := request.Params.Arguments["case_sensitive"].(bool)
	wholeWord, _ := request.Params.Arguments["whole_word"].(bool)
	includeContext, _ := request.Params.Arguments["include_context"].(bool)
	contextLines := 3
	if cl, ok := request.Params.Arguments["context_lines"].(float64); ok {
		contextLines = int(cl)
	}

	if path == "" || pattern == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: path and pattern are required"},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	matches, err := fs.performAdvancedTextSearch(validPath, pattern, caseSensitive, wholeWord, includeContext, contextLines)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if len(matches) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("🔍 No matches found for pattern '%s' in %s", pattern, path)},
			},
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("🔍 Found %d matches for pattern '%s':\n\n", len(matches), pattern))

	for _, match := range matches {
		result.WriteString(fmt.Sprintf("📁 %s:%d\n", match.File, match.LineNumber))
		result.WriteString(fmt.Sprintf("   %s\n", match.Line))

		if includeContext && len(match.Context) > 0 {
			result.WriteString("   Context:\n")
			for _, contextLine := range match.Context {
				result.WriteString(fmt.Sprintf("   │ %s\n", contextLine))
			}
		}
		result.WriteString("\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result.String()},
		},
	}, nil
}

// performSmartSearch - Implementación de búsqueda inteligente
func (fs *FilesystemHandler) performSmartSearch(path, pattern string, includeContent bool, fileTypes []string) (string, error) {
	var results []string
	var contentMatches []SearchMatch

	// Compilar regex del patrón
	regexPattern, err := regexp.Compile(pattern)
	if err != nil {
		// Si no es regex válido, usar búsqueda literal
		regexPattern = regexp.MustCompile(regexp.QuoteMeta(pattern))
	}

	err = filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continuar con otros archivos
		}

		// Validar path
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}

		// Filtrar por tipos de archivo si se especifican
		if len(fileTypes) > 0 {
			ext := strings.ToLower(filepath.Ext(currentPath))
			found := false
			for _, ft := range fileTypes {
				if strings.ToLower(ft) == ext {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		// Buscar en nombre de archivo
		if regexPattern.MatchString(info.Name()) {
			results = append(results, fmt.Sprintf("📄 %s (%s)", currentPath, pathToResourceURI(currentPath)))
		}

		// Buscar en contenido si es archivo de texto y se solicita
		if includeContent && !info.IsDir() && info.Size() < MAX_INLINE_SIZE {
			mimeType := detectMimeType(currentPath)
			if isTextFile(mimeType) {
				content, err := os.ReadFile(currentPath)
				if err == nil {
					lines := strings.Split(string(content), "\n")
					for lineNum, line := range lines {
						if regexPattern.MatchString(line) {
							match := SearchMatch{
								File:       currentPath,
								LineNumber: lineNum + 1,
								Line:       strings.TrimSpace(line),
							}
							contentMatches = append(contentMatches, match)
						}
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	var resultBuilder strings.Builder

	if len(results) > 0 {
		resultBuilder.WriteString(fmt.Sprintf("🔍 File name matches (%d):\n", len(results)))
		for _, result := range results {
			resultBuilder.WriteString(fmt.Sprintf("  %s\n", result))
		}
		resultBuilder.WriteString("\n")
	}

	if len(contentMatches) > 0 {
		resultBuilder.WriteString(fmt.Sprintf("📝 Content matches (%d):\n", len(contentMatches)))
		for _, match := range contentMatches {
			resultBuilder.WriteString(fmt.Sprintf("  📁 %s:%d - %s\n", match.File, match.LineNumber, match.Line))
		}
	}

	if len(results) == 0 && len(contentMatches) == 0 {
		return fmt.Sprintf("🔍 No matches found for pattern '%s' in %s", pattern, path), nil
	}

	return resultBuilder.String(), nil
}

// performAdvancedTextSearch - Implementación de búsqueda avanzada de texto
func (fs *FilesystemHandler) performAdvancedTextSearch(path, pattern string, caseSensitive, wholeWord, includeContext bool, contextLines int) ([]SearchMatch, error) {
	var matches []SearchMatch

	// Preparar el patrón
	searchPattern := pattern
	if !caseSensitive {
		searchPattern = "(?i)" + searchPattern
	}
	if wholeWord {
		searchPattern = `\b` + searchPattern + `\b`
	}

	regexPattern, err := regexp.Compile(searchPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	err = filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Validar path
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}

		// Solo buscar en archivos de texto
		mimeType := detectMimeType(currentPath)
		if !isTextFile(mimeType) || info.Size() > MAX_INLINE_SIZE {
			return nil
		}

		content, err := os.ReadFile(currentPath)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			if regexPattern.MatchString(line) {
				match := SearchMatch{
					File:       currentPath,
					LineNumber: lineNum + 1,
					Line:       strings.TrimSpace(line),
				}

				// Agregar contexto si se solicita
				if includeContext {
					var context []string
					start := max(0, lineNum-contextLines)
					end := min(len(lines), lineNum+contextLines+1)

					for i := start; i < end; i++ {
						if i != lineNum {
							context = append(context, strings.TrimSpace(lines[i]))
						}
					}
					match.Context = context
				}

				matches = append(matches, match)
			}
		}

		return nil
	})

	return matches, err
}

// Funciones auxiliares
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleFindDuplicates - Encuentra archivos duplicados por hash
func (fs *FilesystemHandler) handleFindDuplicates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	if len(duplicates) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "✅ No duplicate files found"},
			},
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("🔍 Found %d groups of duplicate files:\n\n", len(duplicates)))

	totalWastedSpace := int64(0)
	for hash, files := range duplicates {
		if len(files) > 1 {
			result.WriteString(fmt.Sprintf("📋 Hash: %s\n", hash[:16]+"..."))
			result.WriteString(fmt.Sprintf("   Size: %d bytes each\n", files[0].Size))
			result.WriteString(fmt.Sprintf("   Wasted space: %d bytes\n", files[0].Size*int64(len(files)-1)))
			totalWastedSpace += files[0].Size * int64(len(files)-1)
			
			for _, file := range files {
				result.WriteString(fmt.Sprintf("   📄 %s\n", file.Path))
			}
			result.WriteString("\n")
		}
	}

	result.WriteString(fmt.Sprintf("💾 Total wasted space: %d bytes (%.2f MB)\n", 
		totalWastedSpace, float64(totalWastedSpace)/(1024*1024)))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result.String()},
		},
	}, nil
}

// findDuplicateFiles - Busca archivos duplicados por contenido (hash MD5)
func (fs *FilesystemHandler) findDuplicateFiles(path string) (map[string][]DuplicateFile, error) {
	hashMap := make(map[string][]DuplicateFile)

	err := filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Validar path
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}

		// Solo archivos menores a 100MB para eficiencia
		if info.Size() > 100*1024*1024 {
			return nil
		}

		hash, err := calculateFileMD5(currentPath)
		if err != nil {
			return nil // Continuar con otros archivos
		}

		duplicate := DuplicateFile{
			Path: currentPath,
			Hash: hash,
			Size: info.Size(),
		}

		hashMap[hash] = append(hashMap[hash], duplicate)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Filtrar solo los que tienen duplicados
	duplicates := make(map[string][]DuplicateFile)
	for hash, files := range hashMap {
		if len(files) > 1 {
			duplicates[hash] = files
		}
	}

	return duplicates, nil
}

// calculateFileMD5 - Calcula hash MD5 de un archivo
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
