package filesystemserver

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleCompareFiles - Comparación avanzada de archivos
func (fs *FilesystemHandler) handleCompareFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file1, _ := request.Params.Arguments["file1"].(string)
	file2, _ := request.Params.Arguments["file2"].(string)
	format, _ := request.Params.Arguments["format"].(string)

	if file1 == "" || file2 == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: both file1 and file2 are required"},
			},
			IsError: true,
		}, nil
	}

	if format == "" {
		format = "unified"
	}

	validPath1, err := fs.validatePath(file1)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error with file1: %v", err)},
			},
			IsError: true,
		}, nil
	}

	validPath2, err := fs.validatePath(file2)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error with file2: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Verificar que ambos archivos existen
	if _, err := os.Stat(validPath1); os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: file1 does not exist: %s", file1)},
			},
			IsError: true,
		}, nil
	}

	if _, err := os.Stat(validPath2); os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: file2 does not exist: %s", file2)},
			},
			IsError: true,
		}, nil
	}

	diff, err := fs.compareFiles(validPath1, validPath2, format)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Comparison error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Si los archivos son idénticos
	if diff.Similar == 100.0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "✅ Files are identical"},
			},
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("🔍 File Comparison Results:\n\n"))
	result.WriteString(fmt.Sprintf("📁 File 1: %s\n", file1))
	result.WriteString(fmt.Sprintf("📁 File 2: %s\n", file2))
	result.WriteString(fmt.Sprintf("📊 Similarity: %.1f%%\n\n", diff.Similar))

	if len(diff.Added) > 0 {
		result.WriteString(fmt.Sprintf("➕ Added lines (%d):\n", len(diff.Added)))
		for _, line := range diff.Added {
			result.WriteString(fmt.Sprintf("  + %s\n", line))
		}
		result.WriteString("\n")
	}

	if len(diff.Removed) > 0 {
		result.WriteString(fmt.Sprintf("➖ Removed lines (%d):\n", len(diff.Removed)))
		for _, line := range diff.Removed {
			result.WriteString(fmt.Sprintf("  - %s\n", line))
		}
		result.WriteString("\n")
	}

	if len(diff.Modified) > 0 {
		result.WriteString(fmt.Sprintf("📝 Modified lines (%d):\n", len(diff.Modified)))
		for _, line := range diff.Modified {
			result.WriteString(fmt.Sprintf("  ~ %s\n", line))
		}
		result.WriteString("\n")
	}

	result.WriteString(fmt.Sprintf("📈 Unchanged lines: %d\n", diff.Unchanged))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result.String()},
		},
	}, nil
}

// compareFiles - Realiza la comparación entre dos archivos
func (fs *FilesystemHandler) compareFiles(path1, path2, format string) (*FileDiff, error) {
	// Verificar si son archivos de texto
	mimeType1 := detectMimeType(path1)
	mimeType2 := detectMimeType(path2)

	if !isTextFile(mimeType1) || !isTextFile(mimeType2) {
		return fs.compareBinaryFiles(path1, path2)
	}

	return fs.compareTextFiles(path1, path2, format)
}

// compareTextFiles - Compara archivos de texto línea por línea
func (fs *FilesystemHandler) compareTextFiles(path1, path2, format string) (*FileDiff, error) {
	lines1, err := readFileLines(path1)
	if err != nil {
		return nil, fmt.Errorf("error reading file1: %v", err)
	}

	lines2, err := readFileLines(path2)
	if err != nil {
		return nil, fmt.Errorf("error reading file2: %v", err)
	}

	diff := &FileDiff{
		File1: path1,
		File2: path2,
	}

	// Crear mapas para comparación rápida
	lines1Map := make(map[string]bool)
	lines2Map := make(map[string]bool)

	for _, line := range lines1 {
		lines1Map[line] = true
	}

	for _, line := range lines2 {
		lines2Map[line] = true
	}

	// Encontrar líneas agregadas (en file2 pero no en file1)
	for _, line := range lines2 {
		if !lines1Map[line] {
			diff.Added = append(diff.Added, line)
		}
	}

	// Encontrar líneas eliminadas (en file1 pero no en file2)
	for _, line := range lines1 {
		if !lines2Map[line] {
			diff.Removed = append(diff.Removed, line)
		}
	}

	// Contar líneas sin cambios
	for _, line := range lines1 {
		if lines2Map[line] {
			diff.Unchanged++
		}
	}

	// Calcular similitud
	totalLines := len(lines1) + len(lines2)
	if totalLines > 0 {
		diff.Similar = float64(diff.Unchanged*2) / float64(totalLines) * 100
	} else {
		diff.Similar = 100.0
	}

	// Para líneas modificadas, intentar encontrar líneas similares
	diff.Modified = fs.findModifiedLines(diff.Removed, diff.Added)

	return diff, nil
}

// compareBinaryFiles - Compara archivos binarios por hash
func (fs *FilesystemHandler) compareBinaryFiles(path1, path2 string) (*FileDiff, error) {
	hash1, err := calculateFileMD5(path1)
	if err != nil {
		return nil, fmt.Errorf("error calculating hash for file1: %v", err)
	}

	hash2, err := calculateFileMD5(path2)
	if err != nil {
		return nil, fmt.Errorf("error calculating hash for file2: %v", err)
	}

	diff := &FileDiff{
		File1: path1,
		File2: path2,
	}

	if hash1 == hash2 {
		diff.Similar = 100.0
		diff.Unchanged = 1
	} else {
		diff.Similar = 0.0
		diff.Added = []string{"Binary files differ"}
	}

	return diff, nil
}

// findModifiedLines - Intenta encontrar líneas que fueron modificadas
func (fs *FilesystemHandler) findModifiedLines(removed, added []string) []string {
	var modified []string
	const similarityThreshold = 0.6

	usedAdded := make(map[int]bool)
	for _, removedLine := range removed {
		bestMatch := -1
		bestSimilarity := 0.0

		for i, addedLine := range added {
			if usedAdded[i] {
				continue
			}

			similarity := calculateStringSimilarity(removedLine, addedLine)
			if similarity > bestSimilarity && similarity > similarityThreshold {
				bestSimilarity = similarity
				bestMatch = i
			}
		}

		if bestMatch != -1 {
			modified = append(modified, fmt.Sprintf("%s → %s", removedLine, added[bestMatch]))
			usedAdded[bestMatch] = true
		}
	}

	return modified
}

// readFileLines - Lee un archivo y devuelve sus líneas
func readFileLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}

	return lines, scanner.Err()
}

// calculateStringSimilarity - Calcula similitud entre dos strings
func calculateStringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Algoritmo simple de similitud basado en caracteres comunes
	longer := s1
	shorter := s2
	if len(s2) > len(s1) {
		longer = s2
		shorter = s1
	}

	longerLen := len(longer)
	if longerLen == 0 {
		return 1.0
	}

	editDistance := levenshteinDistance(longer, shorter)
	return float64(longerLen-editDistance) / float64(longerLen)
}

// levenshteinDistance - Calcula la distancia de Levenshtein entre dos strings
func levenshteinDistance(s1, s2 string) int {
	len1, len2 := len(s1), len(s2)
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min3(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len1][len2]
}

// min3 - Función auxiliar para encontrar el mínimo de 3 valores
func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}
