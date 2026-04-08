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

// convertToString converts interface{} to string with broad type support.
// Handles float64, bool, int, json.Number — types that LLMs send unexpectedly.
func convertToString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val)), true
		}
		return fmt.Sprintf("%g", val), true
	case bool:
		if val {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", val), true
	case int64:
		return fmt.Sprintf("%d", val), true
	case nil:
		return "", false
	default:
		return fmt.Sprintf("%v", val), true
	}
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
	if oldText == "" {
		return nil, fmt.Errorf("old_text cannot be empty")
	}

	// Normalización única
	content = normalizeLineEndings(content)
	oldText = normalizeLineEndings(oldText)
	newText = normalizeLineEndings(newText)

	originalLines := strings.Count(content, "\n") + 1

	// Fast path: Check exact match primero (más común)
	if idx := strings.Index(content, oldText); idx >= 0 {
		newContent := strings.ReplaceAll(content, oldText, newText)
		replacements := strings.Count(content, oldText)
		linesAffected := calculateLinesWithText(content, oldText)
		oldTextLines := strings.Count(oldText, "\n") + 1
		newTextLines := strings.Count(newText, "\n") + 1
		totalLines := strings.Count(newContent, "\n") + 1

		return &EditResult{
			ModifiedContent:  newContent,
			ReplacementCount: replacements,
			MatchConfidence:  "high",
			LinesAffected:    linesAffected,
			LinesRemoved:     oldTextLines * replacements,
			LinesAdded:       newTextLines * replacements,
			TotalLines:       totalLines,
		}, nil
	}

	// ── Phase 2: already-present detection (idempotent — ported from ultra) ──
	// If old_text is absent but new_text is already in the file → skip gracefully.
	fixedOld := FixLiteralEscapes(oldText)
	fixedNew := FixLiteralEscapes(newText)

	oldAbsent := !strings.Contains(content, oldText) && !strings.Contains(content, fixedOld)
	newPresent := newText != "" && (strings.Contains(content, newText) || strings.Contains(content, fixedNew))

	if newPresent && oldAbsent {
		return &EditResult{
			ModifiedContent:  content,
			ReplacementCount: 0,
			MatchConfidence:  "already_present",
			LinesAffected:    0,
			LinesAdded:       0,
			LinesRemoved:     0,
			TotalLines:       originalLines,
		}, nil
	}

	// ── Phase 3: literal-escape recovery (ported from ultra) ──
	// Claude Desktop sometimes sends literal \n (two chars) instead of real newlines.
	if fixedOld != oldText {
		countFixed := strings.Count(content, fixedOld)
		if countFixed > 0 {
			replacement := fixedNew
			if replacement == "" {
				replacement = newText
			}
			newContent := strings.ReplaceAll(content, fixedOld, replacement)
			linesAffected := strings.Count(fixedOld, "\n") + 1
			oldLines := strings.Count(fixedOld, "\n") + 1
			newLines := strings.Count(replacement, "\n") + 1
			return &EditResult{
				ModifiedContent:  newContent,
				ReplacementCount: countFixed,
				MatchConfidence:  "high_after_escape_fix",
				LinesAffected:    linesAffected,
				LinesRemoved:     oldLines * countFixed,
				LinesAdded:       newLines * countFixed,
				TotalLines:       strings.Count(newContent, "\n") + 1,
			}, nil
		}
	}

	// Solo si no hay match exacto, hacer búsqueda flexible
	lines := strings.Split(content, "\n")
	newLines := make([]string, len(lines)) // Pre-allocate exact size
	replacements := 0
	affectedLines := 0

	normalizedOld := strings.TrimSpace(oldText)

	// Primero intentar línea por línea
	for i, line := range lines {
		newLine := line
		
		// Checks en orden de probabilidad
		if strings.Contains(line, oldText) {
			newLine = strings.ReplaceAll(line, oldText, newText)
			replacements += strings.Count(line, oldText)
			affectedLines++
		} else if trimmed := strings.TrimSpace(line); trimmed == normalizedOld {
			newLine = getIndentation(line) + strings.TrimSpace(newText)
			replacements++
			affectedLines++
		} else if strings.Contains(line, normalizedOld) {
			newLine = strings.ReplaceAll(line, normalizedOld, newText)
			replacements += strings.Count(line, normalizedOld)
			affectedLines++
		}
		
		newLines[i] = newLine
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
				LinesRemoved:     strings.Count(oldText, "\n") + 1,
				LinesAdded:       strings.Count(newText, "\n") + 1,
				TotalLines:       strings.Count(newContent, "\n") + 1,
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
				totalRemoved := 0
				for _, m := range matches {
					totalRemoved += strings.Count(m, "\n") + 1
				}
				return &EditResult{
					ModifiedContent:  newContent,
					ReplacementCount: len(matches),
					MatchConfidence:  "low",
					LinesAffected:    countAffectedLines(content, matches),
					LinesRemoved:     totalRemoved,
					LinesAdded:       (strings.Count(newText, "\n") + 1) * len(matches),
					TotalLines:       strings.Count(newContent, "\n") + 1,
				}, nil
			}
		}
	}

	// Si encontramos reemplazos, devolver el resultado
	if replacements > 0 {
		modifiedContent := strings.Join(newLines, "\n")
		totalLines := strings.Count(modifiedContent, "\n") + 1
		// Line-by-line replacements: each affected line is removed and re-added
		return &EditResult{
			ModifiedContent:  modifiedContent,
			ReplacementCount: replacements,
			MatchConfidence:  "medium",
			LinesAffected:    affectedLines,
			LinesRemoved:     affectedLines,
			LinesAdded:       affectedLines,
			TotalLines:       totalLines,
		}, nil
	}

	// No match — return 0 replacements without error (non-blocking like ultra)
	return &EditResult{
		ModifiedContent:  content,
		ReplacementCount: 0,
		MatchConfidence:  "none",
		LinesAffected:    0,
		LinesAdded:       0,
		LinesRemoved:     0,
		TotalLines:       originalLines,
	}, nil
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
	affected := make(map[int]bool)
	totalLines := strings.Count(content, "\n") + 1

	for _, match := range matches {
		idx := strings.Index(content, match)
		if idx >= 0 {
			lineNum := strings.Count(content[:idx], "\n")
			matchLines := strings.Count(match, "\n") + 1
			// Asegurarse de no exceder el número de líneas del contenido
			for i := 0; i < matchLines && (lineNum+i) < totalLines; i++ {
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

	// Explicitly use 'lines' to avoid unused variable warning
	if len(lines) == 0 {
		return 0
	}

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
	source, ok := request.GetArguments()["source"].(string)
	if !ok {
		return toolError("source must be a string")
	}
	destination, ok := request.GetArguments()["destination"].(string)
	if !ok {
		return toolError("destination must be a string")
	}

	validSource, err := fs.validatePath(source)
	if err != nil {
		return toolErrorf("Error with source path: %v", err)
	}

	validDest, err := fs.validatePath(destination)
	if err != nil {
		return toolErrorf("Error with destination path: %v", err)
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
	source, ok := request.GetArguments()["source"].(string)
	if !ok {
		return toolError("source must be a string")
	}
	destination, ok := request.GetArguments()["destination"].(string)
	if !ok {
		return toolError("destination must be a string")
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

	// Protect allowed directories from being moved
	if fs.isAllowedDir(validSource) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: cannot move allowed directory %s — this is a protected root path", source)},
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

// buildUnifiedDiff produces a simple unified-diff preview between two strings.
func buildUnifiedDiff(original, modified, filename string) string {
	oldLines := strings.Split(original, "\n")
	newLines := strings.Split(modified, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s (original)\n", filename))
	sb.WriteString(fmt.Sprintf("+++ %s (modified)\n", filename))

	// Simple line-by-line diff
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if i < len(oldLines) {
				sb.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if i < len(newLines) {
				sb.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	return sb.String()
}

// calculateLinesWithText calculates how many lines are affected by the text
func calculateLinesWithText(content, text string) int {
	// Count how many lines the matched text spans
	return strings.Count(text, "\n") + 1
}
