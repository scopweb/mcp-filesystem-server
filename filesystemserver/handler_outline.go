package filesystemserver

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// symbolPattern defines a regex pattern for extracting a category of symbols.
type symbolPattern struct {
	category string
	re       *regexp.Regexp
}

// languageFromExtension maps file extensions to language identifiers.
func languageFromExtension(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py", ".pyi":
		return "python"
	case ".cs":
		return "csharp"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	default:
		return ""
	}
}

// categoryOrder defines the deterministic display order for symbol categories.
var categoryOrder = []string{
	"Packages", "Imports",
	"Classes", "Structs", "Interfaces", "Traits", "Enums", "Types",
	"Functions", "Methods", "Properties",
	"Constants", "Variables",
}

// langPatterns maps language identifiers to their symbol extraction patterns.
// Patterns are compiled once at package init time.
var langPatterns = map[string][]symbolPattern{
	"go": {
		{category: "Structs", re: regexp.MustCompile(`^type\s+(\w+)\s+struct\b`)},
		{category: "Interfaces", re: regexp.MustCompile(`^type\s+(\w+)\s+interface\b`)},
		{category: "Types", re: regexp.MustCompile(`^type\s+(\w+)\s+[^si]`)},
		{category: "Functions", re: regexp.MustCompile(`^func\s+[A-Za-z]`)},
		{category: "Methods", re: regexp.MustCompile(`^func\s+\(`)},
		{category: "Constants", re: regexp.MustCompile(`^const\s+\(|^const\s+\w+`)},
		{category: "Variables", re: regexp.MustCompile(`^var\s+\(|^var\s+\w+`)},
	},
	"javascript": {
		{category: "Classes", re: regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[^=])\s*=>`)},
		{category: "Methods", re: regexp.MustCompile(`^\s+(?:async\s+)?(?:static\s+)?(?:get\s+|set\s+)?(\w+)\s*\([^)]*\)\s*\{`)},
	},
	"typescript": {
		{category: "Interfaces", re: regexp.MustCompile(`^(?:export\s+)?interface\s+\w+`)},
		{category: "Types", re: regexp.MustCompile(`^(?:export\s+)?type\s+\w+\s*=`)},
		{category: "Classes", re: regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[^=])\s*=>`)},
		{category: "Enums", re: regexp.MustCompile(`^(?:export\s+)?(?:const\s+)?enum\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s+(?:async\s+)?(?:static\s+)?(?:readonly\s+)?(?:get\s+|set\s+)?(\w+)\s*[\(<]`)},
	},
	"python": {
		{category: "Classes", re: regexp.MustCompile(`^class\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^def\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s+def\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^async\s+def\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s+async\s+def\s+\w+`)},
	},
	"csharp": {
		{category: "Classes", re: regexp.MustCompile(`^\s*(?:public|private|protected|internal)?\s*(?:static\s+)?(?:abstract\s+)?(?:sealed\s+)?(?:partial\s+)?class\s+\w+`)},
		{category: "Interfaces", re: regexp.MustCompile(`^\s*(?:public|private|protected|internal)?\s*(?:partial\s+)?interface\s+\w+`)},
		{category: "Structs", re: regexp.MustCompile(`^\s*(?:public|private|protected|internal)?\s*(?:readonly\s+)?struct\s+\w+`)},
		{category: "Enums", re: regexp.MustCompile(`^\s*(?:public|private|protected|internal)?\s*enum\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s*(?:public|private|protected|internal)\s+(?:static\s+)?(?:async\s+)?(?:virtual\s+)?(?:override\s+)?(?:abstract\s+)?[\w<>\[\],\s]+\s+\w+\s*\(`)},
		{category: "Properties", re: regexp.MustCompile(`^\s*(?:public|private|protected|internal)\s+(?:static\s+)?(?:virtual\s+)?(?:override\s+)?[\w<>\[\]?]+\s+\w+\s*\{`)},
	},
	"java": {
		{category: "Classes", re: regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?(?:abstract\s+)?(?:final\s+)?class\s+\w+`)},
		{category: "Interfaces", re: regexp.MustCompile(`^\s*(?:public|private|protected)?\s*interface\s+\w+`)},
		{category: "Enums", re: regexp.MustCompile(`^\s*(?:public|private|protected)?\s*enum\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s*(?:public|private|protected)\s+(?:static\s+)?(?:final\s+)?(?:synchronized\s+)?[\w<>\[\],\s]+\s+\w+\s*\(`)},
	},
	"rust": {
		{category: "Structs", re: regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?struct\s+\w+`)},
		{category: "Enums", re: regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?enum\s+\w+`)},
		{category: "Traits", re: regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:unsafe\s+)?trait\s+\w+`)},
		{category: "Types", re: regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?type\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]+"\s+)?fn\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s*impl(?:<[^>]*>)?\s+(?:dyn\s+)?\w+`)},
		{category: "Constants", re: regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:static|const)\s+\w+`)},
	},
	"php": {
		{category: "Classes", re: regexp.MustCompile(`^\s*(?:abstract\s+)?(?:final\s+)?class\s+\w+`)},
		{category: "Interfaces", re: regexp.MustCompile(`^\s*interface\s+\w+`)},
		{category: "Traits", re: regexp.MustCompile(`^\s*trait\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^\s*function\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s*(?:public|private|protected)\s+(?:static\s+)?function\s+\w+`)},
	},
	"ruby": {
		{category: "Classes", re: regexp.MustCompile(`^\s*class\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s*def\s+\w+`)},
		{category: "Methods", re: regexp.MustCompile(`^\s*def\s+self\.\w+`)},
	},
	"swift": {
		{category: "Classes", re: regexp.MustCompile(`^\s*(?:open\s+|public\s+|internal\s+|fileprivate\s+|private\s+)?(?:final\s+)?class\s+\w+`)},
		{category: "Structs", re: regexp.MustCompile(`^\s*(?:public\s+|internal\s+|fileprivate\s+|private\s+)?struct\s+\w+`)},
		{category: "Enums", re: regexp.MustCompile(`^\s*(?:public\s+|internal\s+|fileprivate\s+|private\s+)?enum\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^\s*(?:public\s+|internal\s+|fileprivate\s+|private\s+)?(?:static\s+)?(?:class\s+)?func\s+\w+`)},
	},
	"kotlin": {
		{category: "Classes", re: regexp.MustCompile(`^\s*(?:open\s+|abstract\s+|sealed\s+|data\s+)?class\s+\w+`)},
		{category: "Interfaces", re: regexp.MustCompile(`^\s*interface\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^\s*(?:private\s+|internal\s+|protected\s+)?(?:suspend\s+)?fun\s+`)},
		{category: "Enums", re: regexp.MustCompile(`^\s*enum\s+class\s+\w+`)},
	},
	"c": {
		{category: "Structs", re: regexp.MustCompile(`^(?:typedef\s+)?struct\s+\w+`)},
		{category: "Enums", re: regexp.MustCompile(`^(?:typedef\s+)?enum\s+\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?(?:const\s+)?[\w*]+\s+\*?\w+\s*\([^;]*$`)},
		{category: "Types", re: regexp.MustCompile(`^typedef\s+`)},
	},
	"cpp": {
		{category: "Classes", re: regexp.MustCompile(`^\s*(?:template\s*<[^>]*>\s*)?class\s+\w+`)},
		{category: "Structs", re: regexp.MustCompile(`^\s*(?:template\s*<[^>]*>\s*)?struct\s+\w+`)},
		{category: "Enums", re: regexp.MustCompile(`^\s*enum\s+(?:class\s+)?\w+`)},
		{category: "Functions", re: regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?(?:virtual\s+)?(?:const\s+)?[\w:*&<>]+\s+\*?\w+\s*\([^;]*$`)},
		{category: "Types", re: regexp.MustCompile(`^(?:typedef|using)\s+`)},
	},
}

// supportedExtensions returns a comma-separated list of supported file extensions.
func supportedExtensions() string {
	return ".go, .js, .jsx, .mjs, .ts, .tsx, .py, .pyi, .cs, .java, .rs, .php, .rb, .swift, .kt, .kts, .c, .h, .cpp, .cc, .cxx, .hpp"
}

// symbolEntry represents a matched symbol with its line number and source text.
type symbolEntry struct {
	line int
	text string
}

// extractOutline scans a file line-by-line and extracts symbol definitions grouped by category.
func extractOutline(filePath string, lang string) (string, error) {
	patterns, ok := langPatterns[lang]
	if !ok {
		return "", fmt.Errorf("no patterns defined for language: %s", lang)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Collect symbols grouped by category
	symbols := make(map[string][]symbolEntry)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024) // 64KB default, 4MB max line

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, sp := range patterns {
			if sp.re.MatchString(line) {
				symbols[sp.category] = append(symbols[sp.category], symbolEntry{
					line: lineNum,
					text: strings.TrimSpace(line),
				})
				break // first match wins per line
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	if len(symbols) == 0 {
		return fmt.Sprintf("No symbols found in %s (language: %s, %d lines scanned)",
			filepath.Base(filePath), lang, lineNum), nil
	}

	// Build output in deterministic order
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Outline for %s (%s) — %d lines:\n", filepath.Base(filePath), lang, lineNum))

	for _, cat := range categoryOrder {
		entries, ok := symbols[cat]
		if !ok || len(entries) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n=== %s ===\n", cat))
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("  %4d: %s\n", e.line, e.text))
		}
	}

	return sb.String(), nil
}
