package filesystemserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleAnalyzeProject - Análisis completo de estructura de proyecto
func (fs *FilesystemHandler) handleAnalyzeProject(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Verificar que es un directorio
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: Path must be a directory"},
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

	// Formatear resultado con emojis y estructura organizada
	var result strings.Builder
	result.WriteString("🏗️ **Project Structure Analysis**\n\n")
	result.WriteString(fmt.Sprintf("📁 **Root:** %s\n", structure.Root))
	result.WriteString(fmt.Sprintf("📊 **Total Files:** %d\n", structure.TotalFiles))
	result.WriteString(fmt.Sprintf("💾 **Total Size:** %.2f MB\n\n", float64(structure.TotalSize)/(1024*1024)))

	// Lenguajes detectados
	if len(structure.Languages) > 0 {
		result.WriteString("🔧 **Languages Detected:**\n")
		for lang, count := range structure.Languages {
			percentage := float64(count) / float64(structure.TotalFiles) * 100
			result.WriteString(fmt.Sprintf("  • %s: %d files (%.1f%%)\n", lang, count, percentage))
		}
		result.WriteString("\n")
	}

	// Tipos de archivo
	if len(structure.FileTypes) > 0 {
		result.WriteString("📄 **File Types:**\n")
		for ext, count := range structure.FileTypes {
			percentage := float64(count) / float64(structure.TotalFiles) * 100
			result.WriteString(fmt.Sprintf("  • %s: %d files (%.1f%%)\n", ext, count, percentage))
		}
		result.WriteString("\n")
	}

	// Estructura de directorios
	if len(structure.Directories) > 0 {
		result.WriteString("📂 **Directory Structure:**\n")
		for _, dir := range structure.Directories[:minInt2(10, len(structure.Directories))] {
			relDir := strings.TrimPrefix(dir, structure.Root)
			if relDir == "" {
				relDir = "/"
			}
			result.WriteString(fmt.Sprintf("  • %s\n", relDir))
		}
		if len(structure.Directories) > 10 {
			result.WriteString(fmt.Sprintf("  ... and %d more directories\n", len(structure.Directories)-10))
		}
		result.WriteString("\n")
	}

	// Patrones detectados
	patterns := fs.detectProjectPatterns(structure)
	if len(patterns) > 0 {
		result.WriteString("🎯 **Project Patterns:**\n")
		for _, pattern := range patterns {
			result.WriteString(fmt.Sprintf("  • %s\n", pattern))
		}
		result.WriteString("\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result.String()},
		},
	}, nil
}

// analyzeProjectStructure - Realiza el análisis detallado del proyecto
func (fs *FilesystemHandler) analyzeProjectStructure(path string) (*ProjectStructure, error) {
	structure := &ProjectStructure{
		Root:        path,
		Languages:   make(map[string]int),
		FileTypes:   make(map[string]int),
		Structure:   make(map[string][]string),
		Directories: []string{},
	}

	err := filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continuar con otros archivos
		}

		// Validar path
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}

		// Ignorar directorios comunes que no aportan valor
		if fs.shouldIgnorePath(currentPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			structure.Directories = append(structure.Directories, currentPath)
			return nil
		}

		// Procesar archivo
		structure.TotalFiles++
		structure.TotalSize += info.Size()

		// Analizar extensión
		ext := strings.ToLower(filepath.Ext(currentPath))
		if ext == "" {
			ext = "no-extension"
		}
		structure.FileTypes[ext]++

		// Detectar lenguaje
		language := fs.detectFileLanguage(currentPath, ext)
		if language != "unknown" {
			structure.Languages[language]++
		}

		// Analizar estructura de directorios
		dir := filepath.Dir(currentPath)
		relDir := strings.TrimPrefix(dir, path)
		if relDir != "" {
			structure.Structure[relDir] = append(structure.Structure[relDir], info.Name())
		}

		return nil
	})

	return structure, err
}

// detectFileLanguage - Detecta el lenguaje de programación de un archivo
func (fs *FilesystemHandler) detectFileLanguage(filePath, ext string) string {
	// Mapeo de extensiones a lenguajes
	languageMap := map[string]string{
		".go":         "Go",
		".py":         "Python",
		".js":         "JavaScript",
		".ts":         "TypeScript",
		".jsx":        "React JSX",
		".tsx":        "React TSX",
		".java":       "Java",
		".kt":         "Kotlin",
		".rs":         "Rust",
		".cpp":        "C++",
		".c":          "C",
		".cs":         "C#",
		".php":        "PHP",
		".rb":         "Ruby",
		".swift":      "Swift",
		".dart":       "Dart",
		".scala":      "Scala",
		".html":       "HTML",
		".css":        "CSS",
		".scss":       "SASS",
		".less":       "LESS",
		".vue":        "Vue",
		".sql":        "SQL",
		".sh":         "Shell",
		".ps1":        "PowerShell",
		".bat":        "Batch",
		".dockerfile": "Docker",
		".yaml":       "YAML",
		".yml":        "YAML",
		".json":       "JSON",
		".xml":        "XML",
		".toml":       "TOML",
		".ini":        "INI",
		".md":         "Markdown",
		".tex":        "LaTeX",
		".r":          "R",
		".m":          "MATLAB",
		".jl":         "Julia",
		".elm":        "Elm",
		".ex":         "Elixir",
		".exs":        "Elixir",
		".erl":        "Erlang",
		".hrl":        "Erlang",
		".clj":        "Clojure",
		".fs":         "F#",
		".ml":         "OCaml",
		".hs":         "Haskell",
		".lua":        "Lua",
		".pl":         "Perl",
		".vim":        "Vimscript",
	}

	if lang, exists := languageMap[ext]; exists {
		return lang
	}

	// Detectar por nombre de archivo
	filename := strings.ToLower(filepath.Base(filePath))
	switch filename {
	case "dockerfile":
		return "Docker"
	case "makefile":
		return "Makefile"
	case "rakefile":
		return "Ruby"
	case "gemfile":
		return "Ruby"
	case "package.json":
		return "Node.js"
	case "composer.json":
		return "PHP"
	case "pom.xml":
		return "Java"
	case "cargo.toml":
		return "Rust"
	case "go.mod":
		return "Go"
	case "requirements.txt":
		return "Python"
	case "pipfile":
		return "Python"
	}

	return "unknown"
}

// shouldIgnorePath - Determina si un path debe ser ignorado
func (fs *FilesystemHandler) shouldIgnorePath(path string) bool {
	ignorePaths := []string{
		".git", ".svn", ".hg",
		"node_modules", "vendor", "target",
		".vscode", ".idea", ".vs",
		"bin", "obj", "build", "dist",
		".cache", ".tmp", "temp",
		"__pycache__", ".pytest_cache",
		"coverage", ".nyc_output",
		"logs", "log",
	}

	pathBase := filepath.Base(path)
	for _, ignore := range ignorePaths {
		if pathBase == ignore {
			return true
		}
	}

	// Ignorar archivos ocultos
	if strings.HasPrefix(pathBase, ".") && len(pathBase) > 1 {
		// Excepto algunos archivos importantes
		importantDotFiles := []string{
			".gitignore", ".dockerignore", ".env.example",
			".editorconfig", ".prettierrc", ".eslintrc",
		}
		for _, important := range importantDotFiles {
			if pathBase == important {
				return false
			}
		}
		return true
	}

	return false
}

// detectProjectPatterns - Detecta patrones comunes del proyecto
func (fs *FilesystemHandler) detectProjectPatterns(structure *ProjectStructure) []string {
	var patterns []string

	// Detectar tipo de proyecto
	if structure.Languages["Go"] > 0 {
		if _, exists := structure.FileTypes[".mod"]; exists {
			patterns = append(patterns, "Go Module Project")
		}
	}

	if structure.Languages["JavaScript"] > 0 || structure.Languages["TypeScript"] > 0 {
		if _, exists := structure.FileTypes[".json"]; exists {
			patterns = append(patterns, "Node.js Project")
		}
		if structure.Languages["React JSX"] > 0 || structure.Languages["React TSX"] > 0 {
			patterns = append(patterns, "React Application")
		}
	}

	if structure.Languages["Python"] > 0 {
		patterns = append(patterns, "Python Project")
		if _, exists := structure.FileTypes[".txt"]; exists {
			patterns = append(patterns, "Python with Requirements")
		}
	}

	if structure.Languages["Java"] > 0 {
		patterns = append(patterns, "Java Project")
		if _, exists := structure.FileTypes[".xml"]; exists {
			patterns = append(patterns, "Maven Project")
		}
	}

	if structure.Languages["C#"] > 0 {
		patterns = append(patterns, ".NET Project")
	}

	// Detectar frameworks/herramientas
	if structure.Languages["Docker"] > 0 {
		patterns = append(patterns, "Containerized Application")
	}

	if structure.FileTypes[".md"] > 0 {
		patterns = append(patterns, "Well Documented")
	}

	// Detectar patrones de estructura
	totalDirs := len(structure.Directories)
	if totalDirs > 10 {
		patterns = append(patterns, "Complex Structure")
	} else if totalDirs < 5 {
		patterns = append(patterns, "Simple Structure")
	}

	if structure.TotalFiles > 100 {
		patterns = append(patterns, "Large Project")
	} else if structure.TotalFiles < 20 {
		patterns = append(patterns, "Small Project")
	}

	return patterns
}

// minInt2 - función auxiliar
func minInt2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
