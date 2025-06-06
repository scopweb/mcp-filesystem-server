package filesystemserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleEditFile handles file editing operations
func (fs *FilesystemHandler) handleEditFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := make(map[string]string)
	requiredParams := []string{"path", "old_text", "new_text"}

	for _, param := range requiredParams {
		if value, exists := request.Params.Arguments[param]; exists {
			switch v := value.(type) {
			case string:
				params[param] = v
			case nil:
				return nil, fmt.Errorf("parameter %s is null", param)
			default:
				if str, ok := convertToString(v); ok {
					params[param] = str
				} else {
					return nil, fmt.Errorf("parameter %s must be string, got %T: %v", param, v, v)
				}
			}
		} else {
			return nil, fmt.Errorf("missing required parameter: %s", param)
		}
	}

	path := params["path"]
	oldText := params["old_text"]
	newText := params["new_text"]

	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, fmt.Errorf("path error: %v", err)
	}

	if err := fs.validateEditableFile(validPath); err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	backupPath, err := fs.createBackup(validPath)
	if err != nil {
		return nil, fmt.Errorf("could not create backup: %v", err)
	}
	defer func() {
		if backupPath != "" {
			os.Remove(backupPath)
		}
	}()

	content, err := os.ReadFile(validPath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	analysis := fs.analyzeContent(string(content), oldText)
	result, err := fs.performIntelligentEdit(string(content), oldText, newText, analysis)
	if err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	if err := os.WriteFile(validPath, []byte(result.ModifiedContent), 0644); err != nil {
		return nil, fmt.Errorf("error writing file: %v", err)
	}

	if backupPath != "" {
		os.Remove(backupPath)
		backupPath = ""
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("✅ Successfully edited %s\n📊 Changes: %d replacement(s)\n🎯 Match confidence: %s\n📝 Lines affected: %d",
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

// handleReadResource handles resource reading
func (fs *FilesystemHandler) handleReadResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	if !strings.HasPrefix(uri, "file://") {
		return nil, fmt.Errorf("unsupported URI scheme: %s", uri)
	}

	path := strings.TrimPrefix(uri, "file://")
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(validPath)
	if err != nil {
		return nil, err
	}

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
					result.WriteString(fmt.Sprintf("[FILE] %s (%s) - %d bytes\n", entry.Name(), entryURI, info.Size()))
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

	if fileInfo.Size() > MAX_INLINE_SIZE {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "text/plain",
				Text:     fmt.Sprintf("File is too large to display inline (%d bytes). Use the read_file tool to access specific portions.", fileInfo.Size()),
			},
		}, nil
	}

	content, err := os.ReadFile(validPath)
	if err != nil {
		return nil, err
	}

	mimeType := detectMimeType(validPath)

	if isTextFile(mimeType) {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: mimeType,
				Text:     string(content),
			},
		}, nil
	} else {
		if fileInfo.Size() <= MAX_BASE64_SIZE {
			return []mcp.ResourceContents{
				mcp.BlobResourceContents{
					URI:      uri,
					MIMEType: mimeType,
					Blob:     base64.StdEncoding.EncodeToString(content),
				},
			}, nil
		} else {
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

// Placeholder handlers - implementaciones básicas
func (fs *FilesystemHandler) handleAnalyzeFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}

// handleAnalyzeProject - Implementado en handler_analyze.go

// handleGenerateChecksum - Implementado en handler_analyze.go

func (fs *FilesystemHandler) handleAnalyzeDependencies(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}

func (fs *FilesystemHandler) handleCodeQualityCheck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}

func (fs *FilesystemHandler) handlePerformanceAnalysis(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}

func (fs *FilesystemHandler) handleGenerateReport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}

func (fs *FilesystemHandler) handleSmartSync(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}

func (fs *FilesystemHandler) handleAssistRefactor(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: "Feature not implemented yet"},
		},
	}, nil
}
