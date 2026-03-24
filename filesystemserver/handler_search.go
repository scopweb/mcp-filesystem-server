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

// handleSearch — unified search dispatcher: mode=files|content|duplicates
func (fs *FilesystemHandler) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "search.parse_arguments")
	path, _ := request.GetArguments()["path"].(string)
	mode, _ := request.GetArguments()["mode"].(string)
	if mode == "" {
		mode = "files"
	}

	if path == "" {
		appendSubOperation(ctx, "search.reject_missing_path")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: path is required"},
			},
			IsError: true,
		}, nil
	}

	appendSubOperation(ctx, "search.validate_path")
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: Path error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	switch mode {
	case "files":
		appendSubOperation(ctx, "search.mode.files")
		return fs.searchByName(ctx, validPath, request)
	case "content":
		appendSubOperation(ctx, "search.mode.content")
		return fs.searchByContent(ctx, validPath, request)
	case "duplicates":
		appendSubOperation(ctx, "search.mode.duplicates")
		return fs.searchDuplicates(ctx, validPath)
	default:
		appendSubOperation(ctx, "search.reject_invalid_mode")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: mode must be 'files', 'content', or 'duplicates'"},
			},
			IsError: true,
		}, nil
	}
}

// searchByName — find files by name/glob pattern
func (fs *FilesystemHandler) searchByName(ctx context.Context, validPath string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "search.files.prepare_pattern")
	pattern, _ := request.GetArguments()["pattern"].(string)
	if pattern == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: pattern is required for mode=files"},
			},
			IsError: true,
		}, nil
	}

	isGlob := strings.ContainsAny(pattern, "*?[")
	var matches []string

	appendSubOperation(ctx, "search.files.walk")
	err := filepath.Walk(validPath, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}
		name := info.Name()
		var matched bool
		if isGlob {
			matched, _ = filepath.Match(pattern, name)
		} else {
			matched = strings.Contains(strings.ToLower(name), strings.ToLower(pattern))
		}
		if matched {
			matches = append(matches, currentPath)
		}
		return nil
	})
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
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("🔍 No files found matching '%s' in %s", pattern, validPath)},
			},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 Found %d file(s) matching '%s':\n", len(matches), pattern))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("  📄 %s\n", m))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: sb.String()}},
	}, nil
}

// searchByContent — regex search inside file contents
func (fs *FilesystemHandler) searchByContent(ctx context.Context, validPath string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "search.content.prepare_pattern")
	pattern, _ := request.GetArguments()["pattern"].(string)
	if pattern == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: pattern is required for mode=content"},
			},
			IsError: true,
		}, nil
	}

	includeContent := true
	if ic, ok := request.GetArguments()["include_content"].(bool); ok {
		includeContent = ic
	}

	fileTypesParam, _ := request.GetArguments()["file_types"].([]interface{})
	contextLines := 0
	if cl, ok := request.GetArguments()["context_lines"].(float64); ok && cl >= 0 {
		contextLines = int(cl)
	}

	fileTypes := []string{}
	for _, ft := range fileTypesParam {
		if str, ok := ft.(string); ok {
			fileTypes = append(fileTypes, str)
		}
	}

	appendSubOperation(ctx, "search.content.scan")
	results, err := fs.performSmartSearch(ctx, validPath, pattern, includeContent, fileTypes, contextLines)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: Search error: %v", err)},
			},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: results}},
	}, nil
}

// searchDuplicates — find duplicate files by content hash
func (fs *FilesystemHandler) searchDuplicates(ctx context.Context, validPath string) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "search.duplicates.hash_scan")
	duplicates, err := fs.findDuplicateFiles(ctx, validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: Duplicate detection error: %v", err)},
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
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: result.String()}},
	}, nil
}

// performSmartSearch — regex/literal search inside file contents
func (fs *FilesystemHandler) performSmartSearch(ctx context.Context, path, pattern string, includeContent bool, fileTypes []string, contextLines int) (string, error) {
	var nameMatches []string
	var contentMatches []SearchMatch

	regexPattern, err := regexp.Compile(pattern)
	if err != nil {
		appendSubOperation(ctx, "search.content.fallback_literal_pattern")
		regexPattern = regexp.MustCompile(regexp.QuoteMeta(pattern))
	}

	appendSubOperation(ctx, "search.content.walk")
	err = filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}

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

		if regexPattern.MatchString(info.Name()) {
			nameMatches = append(nameMatches, fmt.Sprintf("📄 %s (%s)", currentPath, pathToResourceURI(currentPath)))
		}

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
							if contextLines > 0 {
								start := max(0, lineNum-contextLines)
								end := min(len(lines), lineNum+contextLines+1)
								for i := start; i < end; i++ {
									if i != lineNum {
										match.Context = append(match.Context, lines[i])
									}
								}
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

	var sb strings.Builder

	if len(nameMatches) > 0 {
		sb.WriteString(fmt.Sprintf("🔍 File name matches (%d):\n", len(nameMatches)))
		for _, r := range nameMatches {
			sb.WriteString(fmt.Sprintf("  %s\n", r))
		}
		sb.WriteString("\n")
	}

	if len(contentMatches) > 0 {
		sb.WriteString(fmt.Sprintf("📝 Content matches (%d):\n", len(contentMatches)))
		for _, match := range contentMatches {
			sb.WriteString(fmt.Sprintf("  📁 %s:%d - %s\n", match.File, match.LineNumber, match.Line))
			for _, ctxLine := range match.Context {
				sb.WriteString(fmt.Sprintf("    │ %s\n", ctxLine))
			}
		}
	}

	if len(nameMatches) == 0 && len(contentMatches) == 0 {
		return fmt.Sprintf("🔍 No matches found for pattern '%s' in %s", pattern, path), nil
	}

	return sb.String(), nil
}

// findDuplicateFiles — finds duplicate files by MD5 hash
func (fs *FilesystemHandler) findDuplicateFiles(ctx context.Context, path string) (map[string][]DuplicateFile, error) {
	hashMap := make(map[string][]DuplicateFile)

	appendSubOperation(ctx, "search.duplicates.walk")
	err := filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if _, err := fs.validatePath(currentPath); err != nil {
			return nil
		}
		// Skip files larger than 100 MB for efficiency
		if info.Size() > 100*1024*1024 {
			return nil
		}
		hash, err := calculateFileMD5(currentPath)
		if err != nil {
			return nil
		}
		hashMap[hash] = append(hashMap[hash], DuplicateFile{
			Path: currentPath,
			Hash: hash,
			Size: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	duplicates := make(map[string][]DuplicateFile)
	for hash, files := range hashMap {
		if len(files) > 1 {
			duplicates[hash] = files
		}
	}
	return duplicates, nil
}

// calculateFileMD5 — computes MD5 hash of a file
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
