package filesystemserver

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/mark3labs/mcp-go/mcp"
)


// detectMimeType tries to determine the MIME type of a file
func detectMimeType(path string) string {
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		ext := filepath.Ext(path)
		if ext != "" {
			mimeType := mime.TypeByExtension(ext)
			if mimeType != "" {
				return mimeType
			}
		}
		return "application/octet-stream"
	}
	return mtype.String()
}

// isTextFile determines if a file is likely a text file based on MIME type
func isTextFile(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

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

	if strings.Contains(mimeType, "+xml") ||
		strings.Contains(mimeType, "+json") ||
		strings.Contains(mimeType, "+yaml") {
		return true
	}

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

// detectLanguage detects programming language from content
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

// convertToString converts interface{} to string
func convertToString(v interface{}) (string, bool) {
	if str, ok := v.(string); ok {
		return str, true
	}
	return "", false
}

// validateEditableFile checks if a file can be edited
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

// createBackup creates a backup of a file
func (fs *FilesystemHandler) createBackup(path string) (string, error) {
	backupPath := path + ".backup"
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(backupPath, content, 0644)
	return backupPath, err
}

// analyzeContent analyzes file content for editing
func (fs *FilesystemHandler) analyzeContent(content, oldText string) interface{} {
	return nil // Basic implementation
}

// performIntelligentEdit performs intelligent text replacement
func (fs *FilesystemHandler) performIntelligentEdit(content, oldText, newText string, analysis interface{}) (*EditResult, error) {
	// Si oldText está vacío, retornar error
	if oldText == "" {
		return nil, fmt.Errorf("old_text cannot be empty")
	}

	// Normalizar saltos de línea para compatibilidad Windows/Unix
	content = normalizeLineEndings(content)
	oldText = normalizeLineEndings(oldText)
	newText = normalizeLineEndings(newText)

	// Contador inicial para verificar si hay coincidencias exactas
	exactMatches := strings.Count(content, oldText)
	
	// Si hay coincidencias exactas, hacer reemplazo directo
	if exactMatches > 0 {
		newContent := strings.ReplaceAll(content, oldText, newText)
		
		// Calcular líneas afectadas
		linesAffected := 0
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, oldText) {
				linesAffected++
			}
		}

		return &EditResult{
			ModifiedContent:  newContent,
			ReplacementCount: exactMatches,
			MatchConfidence:  "high",
			LinesAffected:    linesAffected,
		}, nil
	}

	// Si no hay coincidencias exactas, intentar búsqueda flexible
	lines := strings.Split(content, "\n")
	newLines := make([]string, 0, len(lines))
	replacements := 0
	linesAffected := 0
	
	// Intentar con diferentes normalizaciones
	normalizedOld := strings.TrimSpace(oldText)
	
	// Primero intentar línea por línea
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		replaced := false
		
		// 1. Búsqueda exacta de línea completa (ignorando espacios)
		if trimmedLine == normalizedOld {
			// Preservar indentación original
			indent := getIndentation(line)
			newLines = append(newLines, indent+strings.TrimSpace(newText))
			replacements++
			linesAffected++
			replaced = true
		} else if strings.Contains(line, oldText) {
			// 2. Reemplazo parcial exacto dentro de la línea
			newLine := strings.ReplaceAll(line, oldText, newText)
			newLines = append(newLines, newLine)
			replacements += strings.Count(line, oldText)
			linesAffected++
			replaced = true
		} else if strings.Contains(line, normalizedOld) {
			// 3. Reemplazo con texto normalizado
			newLine := strings.ReplaceAll(line, normalizedOld, newText)
			newLines = append(newLines, newLine)
			replacements += strings.Count(line, normalizedOld)
			linesAffected++
			replaced = true
		}
		
		if !replaced {
			// 4. Intentar con normalización más agresiva
			lineNormalized := normalizeWhitespace(line)
			oldNormalized := normalizeWhitespace(oldText)
			
			if strings.Contains(lineNormalized, oldNormalized) {
				// Encontrar la posición y reemplazar manteniendo formato original
				idx := strings.Index(lineNormalized, oldNormalized)
				if idx >= 0 {
					// Reconstruir línea con el reemplazo
					result := reconstructLine(line, oldText, newText, idx)
					newLines = append(newLines, result)
					replacements++
					linesAffected++
					replaced = true
				}
			}
		}
		
		if !replaced {
			newLines = append(newLines, line)
		}
	}
	
	// Si aún no encontramos coincidencias, intentar búsqueda multi-línea
	if replacements == 0 {
		// Buscar coincidencias que crucen líneas
		multilineMatch := findMultilineMatch(content, oldText)
		if multilineMatch {
			newContent := strings.ReplaceAll(content, oldText, newText)
			return &EditResult{
				ModifiedContent:  newContent,
				ReplacementCount: 1,
				MatchConfidence:  "medium",
				LinesAffected:    strings.Count(oldText, "\n") + 1,
			}, nil
		}
		
		// Última opción: búsqueda con regex flexible
		escapedOld := regexp.QuoteMeta(oldText)
		// Permitir espacios flexibles y saltos de línea opcionales
		flexiblePattern := makeFlexiblePattern(escapedOld)
		
		re, err := regexp.Compile(flexiblePattern)
		if err == nil {
			matches := re.FindAllString(content, -1)
			if len(matches) > 0 {
				newContent := re.ReplaceAllString(content, newText)
				return &EditResult{
					ModifiedContent:  newContent,
					ReplacementCount: len(matches),
					MatchConfidence:  "low",
					LinesAffected:    countAffectedLines(content, matches),
				}, nil
			}
		}
	}

	// Si encontramos reemplazos, devolver el resultado
	if replacements > 0 {
		return &EditResult{
			ModifiedContent:  strings.Join(newLines, "\n"),
			ReplacementCount: replacements,
			MatchConfidence:  "medium",
			LinesAffected:    linesAffected,
		}, nil
	}

	// No se encontraron coincidencias - retornar con información de debug
	return &EditResult{
		ModifiedContent:  content,
		ReplacementCount: 0,
		MatchConfidence:  "none",
		LinesAffected:    0,
	}, fmt.Errorf("no matches found for text: %q", oldText)
}

// Funciones auxiliares para mejorar la búsqueda
func normalizeLineEndings(s string) string {
	// Convertir todos los saltos de línea a \n
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func normalizeWhitespace(s string) string {
	// Reemplazar múltiples espacios/tabs con un solo espacio
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}

func getIndentation(line string) string {
	// Obtener la indentación de una línea
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

func reconstructLine(original, oldText, newText string, normalizedPos int) string {
	// Reconstruir línea manteniendo formato original
	// Esta es una implementación simplificada
	return strings.Replace(original, strings.TrimSpace(oldText), strings.TrimSpace(newText), 1)
}

func findMultilineMatch(content, pattern string) bool {
	// Verificar si el patrón cruza múltiples líneas
	return strings.Contains(pattern, "\n") && strings.Contains(content, pattern)
}

func makeFlexiblePattern(escaped string) string {
	// Hacer el patrón más flexible con espacios y saltos de línea
	pattern := strings.ReplaceAll(escaped, `\ `, `\s+`)
	pattern = strings.ReplaceAll(pattern, `\n`, `\s*\n\s*`)
	return pattern
}

func countAffectedLines(content string, matches []string) int {
	// Contar líneas afectadas por los matches
	lines := strings.Split(content, "\n")
	affected := make(map[int]bool)
	
	for _, match := range matches {
		idx := strings.Index(content, match)
		if idx >= 0 {
			lineNum := strings.Count(content[:idx], "\n")
			matchLines := strings.Count(match, "\n") + 1
			for i := 0; i < matchLines; i++ {
				affected[lineNum+i] = true
			}
		}
	}
	
	return len(affected)
}

// isFileTooLarge checks if a file is too large for single operations
func (fs *FilesystemHandler) isFileTooLarge(path string) (bool, int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	return info.Size() > MAX_INLINE_SIZE, info.Size(), nil
}

// calculateCodeComplexity calculates code complexity metrics
func (fs *FilesystemHandler) calculateCodeComplexity(content, language string) CodeComplexity {
	complexity := CodeComplexity{
		CyclomaticComplexity: 1, // Base complexity
	}

	switch language {
	case "go":
		complexity.FunctionCount = len(regexp.MustCompile(`func\s+\w+`).FindAllString(content, -1))
		complexity.ClassCount = len(regexp.MustCompile(`type\s+\w+\s+struct`).FindAllString(content, -1))
		complexity.ImportCount = len(regexp.MustCompile(`import\s+`).FindAllString(content, -1))

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

// extractDependencies extracts dependencies from code
func (fs *FilesystemHandler) extractDependencies(content, language string) []string {
	var dependencies []string

	switch language {
	case "go":
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
		importRe := regexp.MustCompile(`(?:from\s+(\S+)\s+)?import\s+([^#\n]+)`)
		matches := importRe.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if match[1] != "" {
				dependencies = append(dependencies, match[1])
			}
			imports := strings.Split(match[2], ",")
			for _, imp := range imports {
				dep := strings.TrimSpace(strings.Split(imp, " as ")[0])
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
		}
	}

	// Remove duplicates
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

// calculateCommentRatio calculates the ratio of comment lines
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

// calculateAvgLineLength calculates average line length
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

// calculateMaxLineLength calculates maximum line length
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

// calculateComplexity calculates cyclomatic complexity
func (fs *FilesystemHandler) calculateComplexity(content, language string) int {
	complexity := 1

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

// handleCopyFile handles file copy operations
func (fs *FilesystemHandler) handleCopyFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source, ok := request.Params.Arguments["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source must be a string")
	}
	destination, ok := request.Params.Arguments["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination must be a string")
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error with source path: %v", err)},
			},
			IsError: true,
		}, nil
	}

	validDest, err := fs.validatePath(destination)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error with destination path: %v", err)},
			},
			IsError: true,
		}, nil
	}

	err = copyFile(validSource, validDest)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error copying file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	resourceURI := pathToResourceURI(validDest)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully copied %s to %s", source, destination)},
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

// handleMoveFile handles file move operations
func (fs *FilesystemHandler) handleMoveFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source, ok := request.Params.Arguments["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source must be a string")
	}
	destination, ok := request.Params.Arguments["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination must be a string")
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error with source path: %v", err)},
			},
			IsError: true,
		}, nil
	}

	validDest, err := fs.validatePath(destination)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error with destination path: %v", err)},
			},
			IsError: true,
		}, nil
	}

	parentDir := filepath.Dir(validDest)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error creating destination directory: %v", err)},
			},
			IsError: true,
		}, nil
	}

	err = os.Rename(validSource, validDest)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error moving file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	resourceURI := pathToResourceURI(validDest)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully moved %s to %s", source, destination)},
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
