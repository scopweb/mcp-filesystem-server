package filesystemserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleTree generates a tree view of directory structure
func (fs *FilesystemHandler) handleTree(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
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

	depth := 3
	if depthParam, ok := request.GetArguments()["depth"]; ok {
		if d, ok := depthParam.(float64); ok {
			depth = int(d)
		}
	}

	followSymlinks := false
	if followParam, ok := request.GetArguments()["follow_symlinks"]; ok {
		if f, ok := followParam.(bool); ok {
			followSymlinks = f
		}
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
				mcp.TextContent{Type: "text", Text: "Error: The specified path is not a directory"},
			},
			IsError: true,
		}, nil
	}

	tree, err := fs.buildTree(validPath, depth, 0, followSymlinks)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error building directory tree: %v", err)},
			},
			IsError: true,
		}, nil
	}

	jsonData, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error generating JSON: %v", err)},
			},
			IsError: true,
		}, nil
	}

	resourceURI := pathToResourceURI(validPath)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: fmt.Sprintf("Directory tree for %s (max depth: %d):\n\n%s", validPath, depth, string(jsonData))},
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

// handleGetFileInfo gets detailed file information
func (fs *FilesystemHandler) handleGetFileInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, ok := request.GetArguments()["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
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

	info, err := fs.getFileStats(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error getting file info: %v", err)},
			},
			IsError: true,
		}, nil
	}

	mimeType := "directory"
	if info.IsFile {
		mimeType = detectMimeType(validPath)
	}

	resourceURI := pathToResourceURI(validPath)

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
					info.Created.Format("2006-01-02 15:04:05"),
					info.Modified.Format("2006-01-02 15:04:05"),
					info.Accessed.Format("2006-01-02 15:04:05"),
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
					Text:     fmt.Sprintf("%s: %s (%s, %d bytes)", fileTypeText, validPath, mimeType, info.Size),
				},
			},
		},
	}, nil
}

// handleReadMultipleFiles reads multiple files at once
func (fs *FilesystemHandler) handleReadMultipleFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "read_multiple.parse_arguments")
	pathsParam, ok := request.GetArguments()["paths"]
	if !ok {
		return nil, fmt.Errorf("paths parameter is required")
	}

	pathsSlice, ok := pathsParam.([]any)
	if !ok {
		return nil, fmt.Errorf("paths must be an array of strings")
	}

	if len(pathsSlice) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "No files specified to read"},
			},
			IsError: true,
		}, nil
	}

	const maxFiles = 50
	if len(pathsSlice) > maxFiles {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Too many files requested. Maximum is %d files per request.", maxFiles)},
			},
			IsError: true,
		}, nil
	}

	var results []mcp.Content
	for index, pathInterface := range pathsSlice {
		appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.begin", index))
		path, ok := pathInterface.(string)
		if !ok {
			return nil, fmt.Errorf("each path must be a string")
		}

		if path == "." || path == "./" {
			appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.resolve_cwd", index))
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

		appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.validate_path", index))
		validPath, err := fs.validatePath(path)
		if err != nil {
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error with path '%s': %v", path, err),
			})
			continue
		}

		appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.stat_target", index))
		info, err := os.Stat(validPath)
		if err != nil {
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error accessing '%s': %v", path, err),
			})
			continue
		}

		if info.IsDir() {
			appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.directory_resource", index))
			resourceURI := pathToResourceURI(validPath)
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("'%s' is a directory. Use list_directory tool or resource URI: %s", path, resourceURI),
			})
			continue
		}

		if info.Size() > MAX_INLINE_SIZE {
			appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.large_resource", index))
			resourceURI := pathToResourceURI(validPath)
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("File '%s' is too large to display inline (%d bytes). Access it via resource URI: %s", path, info.Size(), resourceURI),
			})
			continue
		}

		appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.load_file", index))
		content, err := os.ReadFile(validPath)
		if err != nil {
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error reading file '%s': %v", path, err),
			})
			continue
		}

		results = append(results, mcp.TextContent{
			Type: "text",
			Text: fmt.Sprintf("--- File: %s ---", path),
		})

		appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.detect_mime", index))
		mimeType := detectMimeType(validPath)
		if isTextFile(mimeType) {
			appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.return_text", index))
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: string(content),
			})
		} else {
			appendSubOperation(ctx, fmt.Sprintf("read_multiple.file.%d.return_binary_resource", index))
			resourceURI := pathToResourceURI(validPath)
			results = append(results, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Binary file '%s' (%s, %d bytes). Access it via resource URI: %s", path, mimeType, info.Size(), resourceURI),
			})
		}
	}
	appendSubOperation(ctx, fmt.Sprintf("read_multiple.completed.%d", len(pathsSlice)))

	return &mcp.CallToolResult{
		Content: results,
	}, nil
}

// handleListAllowedDirectories lists allowed directories
func (fs *FilesystemHandler) handleListAllowedDirectories(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			mcp.TextContent{Type: "text", Text: result.String()},
		},
	}, nil
}

func (fs *FilesystemHandler) getFileStats(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		Size:        info.Size(),
		Created:     info.ModTime(),
		Modified:    info.ModTime(),
		Accessed:    info.ModTime(),
		IsDirectory: info.IsDir(),
		IsFile:      !info.IsDir(),
		Permissions: fmt.Sprintf("%o", info.Mode().Perm()),
	}, nil
}

func (fs *FilesystemHandler) buildTree(path string, maxDepth int, currentDepth int, followSymlinks bool) (*FileNode, error) {
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return nil, err
	}

	node := &FileNode{
		Name:     filepath.Base(validPath),
		Path:     validPath,
		Modified: info.ModTime(),
	}

	if info.IsDir() {
		node.Type = "directory"

		if currentDepth < maxDepth {
			entries, err := os.ReadDir(validPath)
			if err != nil {
				return nil, err
			}

			for _, entry := range entries {
				entryPath := filepath.Join(validPath, entry.Name())

				if entry.Type()&os.ModeSymlink != 0 {
					if !followSymlinks {
						continue
					}

					linkDest, err := filepath.EvalSymlinks(entryPath)
					if err != nil {
						continue
					}

					if !fs.isPathInAllowedDirs(linkDest) {
						continue
					}

					entryPath = linkDest
				}

				childNode, err := fs.buildTree(entryPath, maxDepth, currentDepth+1, followSymlinks)
				if err != nil {
					continue
				}

				node.Children = append(node.Children, childNode)
			}
		}
	} else {
		node.Type = "file"
		node.Size = info.Size()
	}

	return node, nil
}
