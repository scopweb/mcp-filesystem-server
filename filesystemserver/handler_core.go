package filesystemserver

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// sendProgress sends a progress notification to the client (best-effort, never errors).
func (fs *FilesystemHandler) sendProgress(ctx context.Context, progress, total float64, message string) {
	if fs.mcpServer == nil {
		return
	}
	params := map[string]any{
		"progress": progress,
		"total":    total,
		"message":  message,
	}
	_ = fs.mcpServer.SendNotificationToClient(ctx, "notifications/progress", params)
}

// sendLog sends a logging notification to the client (best-effort, never errors).
func (fs *FilesystemHandler) sendLog(ctx context.Context, level mcp.LoggingLevel, message string) {
	if fs.mcpServer == nil {
		return
	}
	params := map[string]any{
		"level":  string(level),
		"logger": "filesystem-server",
		"data":   message,
	}
	_ = fs.mcpServer.SendNotificationToClient(ctx, "notifications/message", params)
}

// refreshRoots re-fetches roots from the client. Called on roots/list_changed notification.
func (fs *FilesystemHandler) refreshRoots(ctx context.Context) {
	if fs.requestRootsFn == nil {
		return
	}
	dirs, err := fs.requestRootsFn(ctx)
	if err != nil || len(dirs) == 0 {
		return
	}
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		normalized = append(normalized, filepath.Clean(abs)+string(filepath.Separator))
	}
	fs.mu.Lock()
	fs.allowedDirs = normalized
	fs.mu.Unlock()
}

// UpdateAllowedDirs replaces the allowed directory list (thread-safe).
func (fs *FilesystemHandler) UpdateAllowedDirs(dirs []string) {
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		normalized = append(normalized, filepath.Clean(abs)+string(filepath.Separator))
	}
	fs.mu.Lock()
	fs.allowedDirs = normalized
	fs.mu.Unlock()
}

// tryFetchRoots asks the client for its roots and merges them into allowedDirs.
// It only runs once per handler lifetime; subsequent calls are no-ops.
func (fs *FilesystemHandler) tryFetchRoots(ctx context.Context) bool {
	if fs.requestRootsFn == nil {
		return false
	}
	fs.mu.RLock()
	hasAllowedDirs := len(fs.allowedDirs) > 0
	fs.mu.RUnlock()
	if hasAllowedDirs {
		return false
	}
	fs.mu.Lock()
	if fs.rootsFetched {
		fs.mu.Unlock()
		return false
	}
	fs.rootsFetched = true
	fs.mu.Unlock()

	dirs, err := fs.requestRootsFn(ctx)
	if err != nil || len(dirs) == 0 {
		return false
	}

	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		normalized = append(normalized, filepath.Clean(abs)+string(filepath.Separator))
	}

	fs.mu.Lock()
	if len(fs.allowedDirs) == 0 {
		fs.allowedDirs = normalized
	} else {
		fs.allowedDirs = append(fs.allowedDirs, normalized...)
	}
	fs.mu.Unlock()
	return true
}

// NewFilesystemHandler creates a new filesystem handler
func NewFilesystemHandler(allowedDirs []string) (*FilesystemHandler, error) {
	normalized := make([]string, 0, len(allowedDirs))
	for _, dir := range allowedDirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path %s: %w", dir, err)
		}

		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("failed to access directory %s: %w", abs, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("path is not a directory: %s", abs)
		}

		normalized = append(normalized, filepath.Clean(abs)+string(filepath.Separator))
	}
	return &FilesystemHandler{
		allowedDirs: normalized,
		limiter:     newRateLimiter(60, time.Minute),
	}, nil
}

// validatePath checks if a path is within allowed directories
func (fs *FilesystemHandler) validatePath(requestedPath string) (string, error) {
	abs, err := filepath.Abs(requestedPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if !fs.isPathInAllowedDirs(abs) {
		return "", fmt.Errorf("access denied - path outside allowed directories: %s", abs)
	}

	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(abs)
		realParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", fmt.Errorf("parent directory does not exist: %s", parent)
		}

		if !fs.isPathInAllowedDirs(realParent) {
			return "", fmt.Errorf("access denied - parent directory outside allowed directories")
		}
		return abs, nil
	}

	if !fs.isPathInAllowedDirs(realPath) {
		return "", fmt.Errorf("access denied - symlink target outside allowed directories")
	}

	return realPath, nil
}

// isPathInAllowedDirs checks if a path is within any allowed directory
func (fs *FilesystemHandler) isPathInAllowedDirs(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	if !strings.HasSuffix(absPath, string(filepath.Separator)) {
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			absPath = filepath.Dir(absPath) + string(filepath.Separator)
		} else {
			absPath = absPath + string(filepath.Separator)
		}
	}

	fs.mu.RLock()
	dirs := fs.allowedDirs
	fs.mu.RUnlock()

	for _, dir := range dirs {
		if strings.HasPrefix(absPath, dir) {
			return true
		}
	}
	return false
}

// handleReadFile reads file contents, optionally limited to a line range.
func (fs *FilesystemHandler) handleReadFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "read.parse_arguments")
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return toolError("path must be a string")
	}

	// Optional line range (1-based, inclusive on both ends)
	startLine, endLine := 0, 0
	if v, ok := request.GetArguments()["start_line"]; ok {
		if n, ok := v.(float64); ok && n >= 1 {
			startLine = int(n)
		}
	}
	if v, ok := request.GetArguments()["end_line"]; ok {
		if n, ok := v.(float64); ok && n >= 1 {
			endLine = int(n)
		}
	}
	lineRangeRequested := startLine > 0 || endLine > 0

	if path == "." || path == "./" {
		appendSubOperation(ctx, "read.resolve_cwd")
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error resolving current directory: %v", err)},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	appendSubOperation(ctx, "read.validate_path")
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	appendSubOperation(ctx, "read.stat_target")
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if info.IsDir() {
		appendSubOperation(ctx, "read.return_directory_resource")
		resourceURI := pathToResourceURI(validPath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("This is a directory. Use the resource URI to browse its contents: %s", resourceURI)},
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

	// Outline mode: return symbol index instead of file contents
	outline := false
	if v, ok := request.GetArguments()["outline"].(bool); ok {
		outline = v
	}
	if outline {
		appendSubOperation(ctx, "read.outline")
		lang := languageFromExtension(validPath)
		if lang == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Outline not supported for file extension: %s\nSupported: %s", filepath.Ext(validPath), supportedExtensions())},
				},
			}, nil
		}
		result, err := extractOutline(validPath, lang)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error extracting outline: %v", err)},
				},
				IsError: true,
			}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: result},
			},
		}, nil
	}

	if info.Size() > MAX_INLINE_SIZE && !lineRangeRequested {
		appendSubOperation(ctx, "read.return_large_resource")
		resourceURI := pathToResourceURI(validPath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("File is too large to display inline (%d bytes). Access it via resource URI: %s", info.Size(), resourceURI)},
				mcp.EmbeddedResource{
					Type: "resource",
					Resource: mcp.TextResourceContents{
						URI:      resourceURI,
						MIMEType: "text/plain",
						Text:     fmt.Sprintf("Large file: %s (%d bytes)", validPath, info.Size()),
					},
				},
			},
		}, nil
	}

	appendSubOperation(ctx, "read.detect_mime")
	mimeType := detectMimeType(validPath)

	// For text files with a line range: stream with bufio — never loads the full file.
	if lineRangeRequested && isTextFile(mimeType) {
		appendSubOperation(ctx, "read.range_open")
		f, err := os.Open(validPath)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error reading file: %v", err)},
				},
				IsError: true,
			}, nil
		}
		defer f.Close()

		actualStart := startLine
		if actualStart < 1 {
			actualStart = 1
		}
		actualEnd := endLine // 0 means read to EOF

		appendSubOperation(ctx, "read.range_scan")
		scanner := bufio.NewScanner(f)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 4*1024*1024) // support lines up to 4 MB

		var selected []string
		lineNum := 0
		for scanner.Scan() {
			// Check for cancellation every 10000 lines
			lineNum++
			if lineNum%10000 == 0 {
				select {
				case <-ctx.Done():
					return toolError("operation cancelled")
				default:
				}
			}
			if lineNum >= actualStart {
				selected = append(selected, scanner.Text())
			}
			if actualEnd > 0 && lineNum >= actualEnd {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error reading file: %v", err)},
				},
				IsError: true,
			}, nil
		}

		displayEnd := lineNum
		if actualEnd > 0 && actualEnd <= lineNum {
			displayEnd = actualEnd
		}
		if len(selected) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Lines %d-%d: (out of range — file has %d lines)", actualStart, displayEnd, lineNum)},
				},
			}, nil
		}
		text := fmt.Sprintf("Lines %d-%d:\n%s", actualStart, displayEnd, strings.Join(selected, "\n"))
		appendSubOperation(ctx, "read.range_return_text")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: text},
			},
		}, nil
	}

	appendSubOperation(ctx, "read.load_file")
	content, err := os.ReadFile(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error reading file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if isTextFile(mimeType) {
		appendSubOperation(ctx, "read.return_text")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: string(content)},
			},
		}, nil
	} else if isImageFile(mimeType) {
		if info.Size() <= MAX_BASE64_SIZE {
			appendSubOperation(ctx, "read.return_image_base64")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Image file: %s (%s, %d bytes)", validPath, mimeType, info.Size())},
					mcp.ImageContent{
						Type:     "image",
						Data:     base64.StdEncoding.EncodeToString(content),
						MIMEType: mimeType,
					},
				},
			}, nil
		}
	}

	appendSubOperation(ctx, "read.return_binary_resource")
	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Binary file: %s (%s, %d bytes). Access it via resource URI: %s", validPath, mimeType, info.Size(), resourceURI)},
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

// handleWriteFile writes content to a file
func (fs *FilesystemHandler) handleWriteFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "write.parse_arguments")
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return toolError("path must be a string")
	}
	content, ok := request.GetArguments()["content"].(string)
	if !ok {
		return toolError("content must be a string")
	}

	if path == "." || path == "./" {
		appendSubOperation(ctx, "write.resolve_cwd")
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error resolving current directory: %v", err)},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	appendSubOperation(ctx, "write.validate_path")
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if info, err := os.Stat(validPath); err == nil && info.IsDir() {
		appendSubOperation(ctx, "write.reject_directory_target")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "Error: Cannot write to a directory"},
			},
			IsError: true,
		}, nil
	}

	parentDir := filepath.Dir(validPath)
	appendSubOperation(ctx, "write.ensure_parent_dir")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error creating parent directories: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// create_backup: copy existing file before overwriting
	if createBackup, _ := request.GetArguments()["create_backup"].(bool); createBackup {
		appendSubOperation(ctx, "write.create_backup")
		if _, statErr := os.Stat(validPath); statErr == nil {
			if src, err := os.Open(validPath); err == nil {
				if dst, err := os.Create(validPath + ".backup"); err == nil {
					io.Copy(dst, src) //nolint
					dst.Close()
				}
				src.Close()
			}
		}
	}

	// chunk_index: streamed chunked write — bypasses atomic write
	if ci, ok := request.GetArguments()["chunk_index"].(float64); ok {
		chunkIndex := int(ci)
		appendSubOperation(ctx, fmt.Sprintf("write.chunk.%d", chunkIndex))
		var chunkErr error
		if chunkIndex == 0 {
			appendSubOperation(ctx, "write.chunk.initial")
			chunkErr = os.WriteFile(validPath, []byte(content), 0644)
		} else {
			appendSubOperation(ctx, "write.chunk.append")
			f, err := os.OpenFile(validPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error opening file for chunk append: %v", err)},
					},
					IsError: true,
				}, nil
			}
			_, chunkErr = f.WriteString(content)
			f.Close()
		}
		if chunkErr != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error writing chunk %d: %v", chunkIndex, chunkErr)},
				},
				IsError: true,
			}, nil
		}
		totalChunks := -1
		if tc, ok := request.GetArguments()["total_chunks"].(float64); ok {
			totalChunks = int(tc)
		}
		if totalChunks > 0 && chunkIndex == totalChunks-1 {
			appendSubOperation(ctx, "write.chunk.complete")
			fileInfo, _ := os.Stat(validPath)
			size := int64(0)
			if fileInfo != nil {
				size = fileInfo.Size()
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("✅ Chunked write complete: %s (%d bytes, %d/%d chunks)", path, size, chunkIndex+1, totalChunks)},
				},
			}, nil
		}
		appendSubOperation(ctx, "write.chunk.partial")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("✅ Chunk %d written to %s", chunkIndex, path)},
			},
		}, nil
	}

	// Atomic write: write to temp file in same dir, then rename
	appendSubOperation(ctx, "write.atomic.temp_file")
	tmpFile, err := os.CreateTemp(parentDir, ".mcp-write-*.tmp")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error creating temp file: %v", err)},
			},
			IsError: true,
		}, nil
	}
	tmpPath := tmpFile.Name()
	_, writeErr := tmpFile.WriteString(content)
	closeErr := tmpFile.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(tmpPath)
		errMsg := writeErr
		if errMsg == nil {
			errMsg = closeErr
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error writing temp file: %v", errMsg)},
			},
			IsError: true,
		}, nil
	}
	appendSubOperation(ctx, "write.atomic.rename")
	if err := os.Rename(tmpPath, validPath); err != nil {
		os.Remove(tmpPath)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error writing file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	appendSubOperation(ctx, "write.stat_result")
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully wrote to %s", path)},
			},
		}, nil
	}

	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully wrote %d bytes to %s", info.Size(), path)},
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

// handleListDirectory lists directory contents with optional depth recursion.
func (fs *FilesystemHandler) handleListDirectory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return toolError("path must be a string")
	}

	// Parse depth (default 1, max 10)
	maxDepth := 1
	if v, ok := request.GetArguments()["depth"]; ok {
		if n, ok := v.(float64); ok && n >= 1 {
			maxDepth = int(n)
			if maxDepth > 10 {
				maxDepth = 10
			}
		}
	}

	if path == "." || path == "./" {
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error resolving current directory: %v", err)},
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
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "Error: Path is not a directory"},
			},
			IsError: true,
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory listing for: %s\n\n", validPath))

	listDirRecursive(ctx, &result, validPath, "", 1, maxDepth)

	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result.String()},
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

// listDirRecursive writes directory entries to the builder, recursing up to maxDepth levels.
func listDirRecursive(ctx context.Context, sb *strings.Builder, dirPath, indent string, currentDepth, maxDepth int) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		sb.WriteString(fmt.Sprintf("%s[ERROR] %v\n", indent, err))
		return
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		resourceURI := pathToResourceURI(entryPath)

		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("%s[DIR]  %s (%s)\n", indent, entry.Name(), resourceURI))
			if currentDepth < maxDepth {
				listDirRecursive(ctx, sb, entryPath, indent+"  ", currentDepth+1, maxDepth)
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				sb.WriteString(fmt.Sprintf("%s[FILE] %s (%s) - %d bytes\n", indent, entry.Name(), resourceURI, info.Size()))
			} else {
				sb.WriteString(fmt.Sprintf("%s[FILE] %s (%s)\n", indent, entry.Name(), resourceURI))
			}
		}
	}
}

// handleCreateDirectory creates a new directory
func (fs *FilesystemHandler) handleCreateDirectory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return toolError("path must be a string")
	}

	if path == "." || path == "./" {
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error resolving current directory: %v", err)},
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
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if info, err := os.Stat(validPath); err == nil {
		if info.IsDir() {
			resourceURI := pathToResourceURI(validPath)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Directory already exists: %s", path)},
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
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: Path exists but is not a directory: %s", path)},
			},
			IsError: true,
		}, nil
	}

	if err := os.MkdirAll(validPath, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error creating directory: %v", err)},
			},
			IsError: true,
		}, nil
	}

	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully created directory %s", path)},
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

// handleDeleteFile deletes a file or directory
func (fs *FilesystemHandler) handleDeleteFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return toolError("path must be a string")
	}

	if path == "." || path == "./" {
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error resolving current directory: %v", err)},
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
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	info, err := os.Stat(validPath)
	if os.IsNotExist(err) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: Path does not exist: %s", path)},
			},
			IsError: true,
		}, nil
	} else if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error accessing path: %v", err)},
			},
			IsError: true,
		}, nil
	}

	recursive := false
	if recursiveParam, ok := request.GetArguments()["recursive"]; ok {
		if r, ok := recursiveParam.(bool); ok {
			recursive = r
		}
	}

	if info.IsDir() {
		if !recursive {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %s is a directory. Use recursive=true to delete directories.", path)},
				},
				IsError: true,
			}, nil
		}

		if err := os.RemoveAll(validPath); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error deleting directory: %v", err)},
				},
				IsError: true,
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully deleted directory %s", path)},
			},
		}, nil
	}

	if err := os.Remove(validPath); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error deleting file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Successfully deleted file %s", path)},
		},
	}, nil
}

// handleReadMediaFile reads a media file and returns it base64-encoded with its MIME type.
func (fs *FilesystemHandler) handleReadMediaFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return toolError("path must be a string")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}
	if info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "Error: path is a directory, not a file"},
			},
			IsError: true,
		}, nil
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error reading file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	mimeType := detectMimeType(validPath)
	encoded := base64.StdEncoding.EncodeToString(content)

	if isImageFile(mimeType) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.ImageContent{
					Type:     "image",
					Data:     encoded,
					MIMEType: mimeType,
				},
			},
		}, nil
	}

	// Audio and other binary files: return as blob resource
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("file: %s\nmime_type: %s\nsize: %d bytes\nencoding: base64\ndata: %s",
					validPath, mimeType, info.Size(), encoded),
			},
		},
	}, nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}
