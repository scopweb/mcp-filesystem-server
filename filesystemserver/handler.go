package filesystemserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func editFileErrorResult(message string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error: %s", message),
			},
		},
		IsError: true,
	}, nil
}

// handleEditFile handles file editing operations
func (fs *FilesystemHandler) handleEditFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appendSubOperation(ctx, "edit.parse_arguments")
	params := make(map[string]string)
	requiredParams := []string{"path", "old_text", "new_text"}

	for _, param := range requiredParams {
		if value, exists := request.GetArguments()[param]; exists {
			switch v := value.(type) {
			case string:
				params[param] = v
			case nil:
				return editFileErrorResult(fmt.Sprintf("parameter %s is null", param))
			default:
				if str, ok := convertToString(v); ok {
					params[param] = str
				} else {
					return editFileErrorResult(fmt.Sprintf("parameter %s must be string, got %T: %v", param, v, v))
				}
			}
		} else {
			return editFileErrorResult(fmt.Sprintf("missing required parameter: %s", param))
		}
	}

	path := params["path"]
	oldText := params["old_text"]
	newText := params["new_text"]

	appendSubOperation(ctx, "edit.validate_path")
	validPath, err := fs.validatePath(path)
	if err != nil {
		return editFileErrorResult(fmt.Sprintf("path error: %v", err))
	}

	appendSubOperation(ctx, "edit.validate_editable")
	if err := fs.validateEditableFile(validPath); err != nil {
		return editFileErrorResult(err.Error())
	}

	appendSubOperation(ctx, "edit.create_backup")
	backupPath, err := fs.createBackup(validPath)
	if err != nil {
		return editFileErrorResult(fmt.Sprintf("could not create backup: %v", err))
	}
	defer func() {
		if backupPath != "" {
			appendSubOperation(ctx, "edit.cleanup_backup")
			os.Remove(backupPath)
		}
	}()

	appendSubOperation(ctx, "edit.read_file")
	content, err := os.ReadFile(validPath)
	if err != nil {
		return editFileErrorResult(fmt.Sprintf("error reading file: %v", err))
	}

	// Internal timeout to prevent Claude Desktop 4-minute timeout kills.
	// analyzeContent + performIntelligentEdit can be slow on large files.
	const editTimeout = 2 * time.Minute
	type editResult struct {
		result *EditResult
		err    error
	}
	editCh := make(chan editResult, 1)
	go func() {
		analysis := fs.analyzeContent(string(content), oldText)
		r, err := fs.performIntelligentEdit(string(content), oldText, newText, analysis)
		editCh <- editResult{r, err}
	}()

	appendSubOperation(ctx, "edit.analyze_and_replace")
	var result *EditResult
	if ctx != nil {
		select {
		case er := <-editCh:
			if er.err != nil {
				return editFileErrorResult(er.err.Error())
			}
			result = er.result
		case <-time.After(editTimeout):
			return editFileErrorResult(fmt.Sprintf("edit timed out after %s — file may be too large for intelligent matching. Try a more specific old_text or split into smaller edits.", editTimeout))
		case <-ctx.Done():
			return editFileErrorResult(fmt.Sprintf("edit cancelled: %v", ctx.Err()))
		}
	} else {
		select {
		case er := <-editCh:
			if er.err != nil {
				return editFileErrorResult(er.err.Error())
			}
			result = er.result
		case <-time.After(editTimeout):
			return editFileErrorResult(fmt.Sprintf("edit timed out after %s — file may be too large for intelligent matching. Try a more specific old_text or split into smaller edits.", editTimeout))
		}
	}
	appendSubOperation(ctx, "edit.match_confidence."+result.MatchConfidence)

	// Already-present: no write needed, return success with hint (ported from ultra)
	if result.MatchConfidence == "already_present" {
		appendSubOperation(ctx, "edit.already_present")
		if backupPath != "" {
			os.Remove(backupPath)
			backupPath = ""
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("✅ Edit already applied — %s is up to date (new_text already present, old_text absent)", path),
				},
			},
		}, nil
	}

	// No match: don't write, return 0 replacements (non-blocking like ultra)
	if result.ReplacementCount == 0 {
		appendSubOperation(ctx, "edit.no_match")
		if backupPath != "" {
			os.Remove(backupPath)
			backupPath = ""
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("📝 Changes: %d replacement(s) — old_text not found in %s. Re-read the file to get current content.",
						result.ReplacementCount, path),
				},
			},
		}, nil
	}

	// dry_run: preview diff without writing
	dryRun := false
	if v, ok := request.GetArguments()["dry_run"]; ok {
		if b, ok := v.(bool); ok {
			dryRun = b
		}
	}

	if dryRun {
		appendSubOperation(ctx, "edit.dry_run")
		if backupPath != "" {
			os.Remove(backupPath)
			backupPath = ""
		}
		diff := buildUnifiedDiff(string(content), result.ModifiedContent, path)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("🔍 Dry run — no changes written\n📊 Would make: %d replacement(s)\n🎯 Match confidence: %s\n\n%s",
						result.ReplacementCount, result.MatchConfidence, diff),
				},
			},
		}, nil
	}

	appendSubOperation(ctx, "edit.write_file")
	if err := os.WriteFile(validPath, []byte(result.ModifiedContent), 0644); err != nil {
		return editFileErrorResult(fmt.Sprintf("error writing file: %v", err))
	}

	if backupPath != "" {
		os.Remove(backupPath)
		backupPath = ""
	}

	successMsg := fmt.Sprintf("✅ Successfully edited %s\n📊 Changes: %d replacement(s)\n🎯 Match confidence: %s\n📝 Lines affected: %d",
		path, result.ReplacementCount, result.MatchConfidence, result.LinesAffected)
	if result.LinesAffected > 10 {
		successMsg += "\n💡 TIP: Use compare_files to verify large edits"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: successMsg,
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
