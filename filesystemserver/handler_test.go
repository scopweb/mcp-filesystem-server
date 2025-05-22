package filesystemserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadfile_Valid(t *testing.T) {
	// prepare temp directory
	dir := t.TempDir()
	content := "test-content"
	err := os.WriteFile(filepath.Join(dir, "test"), []byte(content), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)
	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir, "test"),
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, content, result.Content[0].(mcp.TextContent).Text)
}

func TestReadfile_Invalid(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir, "test"),
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "no such file or directory")
}

func TestReadfile_NoAccess(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	handler, err := NewFilesystemHandler([]string{dir1})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir2, "test"),
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied - path outside allowed directories")
}

func TestEditFile(t *testing.T) {
	// Prepare temp directory
	dir := t.TempDir()
	originalContent := "This is a test file with some content that we want to modify."
	filePath := filepath.Join(dir, "edit_test.txt")
	err := os.WriteFile(filePath, []byte(originalContent), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)
	
	// Test valid edit
	request := mcp.CallToolRequest{}
	request.Params.Name = "edit_file"
	request.Params.Arguments = map[string]any{
		"path":     filePath,
		"old_text": "some content",
		"new_text": "modified content",
	}

	result, err := handler.handleEditFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Successfully edited file")

	// Verify file was actually modified
	updatedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "This is a test file with modified content that we want to modify.", string(updatedContent))
	
	// Test non-existent text
	request.Params.Arguments = map[string]any{
		"path":     filePath,
		"old_text": "non-existent text",
		"new_text": "something else",
	}

	result, err = handler.handleEditFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "No changes made")
}
