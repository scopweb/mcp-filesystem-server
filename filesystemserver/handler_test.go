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
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Error: access denied - path outside allowed directories")
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
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "✅ Successfully edited")

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
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "📝 Changes: 0 replacement(s)")
}

// Test de handleWriteFile
func TestWriteFile_Valid(t *testing.T) {
	dir := t.TempDir()
	content := "test content"
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "write_file"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir, "test"),
		"content": content,
	}

	result, err := handler.handleWriteFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Successfully wrote")

	// Verificar que el archivo fue creado correctamente
	fileContent, err := os.ReadFile(filepath.Join(dir, "test"))
	require.NoError(t, err)
	assert.Equal(t, content, string(fileContent))
}

func TestWriteFile_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "write_file"
	request.Params.Arguments = map[string]any{
		"path": "/forbidden/path",
		"content": "test",
	}

	result, err := handler.handleWriteFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied - path outside allowed directories")
}

// Test de handleListDirectory
func TestListDirectory_Valid(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644)
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "list_directory"
	request.Params.Arguments = map[string]any{
		"path": dir,
	}

	result, err := handler.handleListDirectory(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "subdir")
	assert.Contains(t, fmt.Sprint(result.Content[0]), "file1.txt")
}

func TestListDirectory_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "list_directory"
	request.Params.Arguments = map[string]any{
		"path": "/nonexistent",
	}

	result, err := handler.handleListDirectory(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "no such file or directory")
}

// Test de handleCreateDirectory
func TestCreateDirectory_Valid(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "create_directory"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir, "newdir"),
	}

	result, err := handler.handleCreateDirectory(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Successfully created directory")

	// Verificar que el directorio existe
	_, err = os.Stat(filepath.Join(dir, "newdir"))
	require.NoError(t, err)
}

func TestCreateDirectory_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "create_directory"
	request.Params.Arguments = map[string]any{
		"path": "/forbidden",
	}

	result, err := handler.handleCreateDirectory(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Error: access denied - path outside allowed directories")
}

// Test de handleDeleteFile
func TestDeleteFile_Valid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test"), []byte("test"), 0644)
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "delete_file"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir, "test"),
	}

	result, err := handler.handleDeleteFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Successfully deleted")

	// Verificar que el archivo fue eliminado
	_, err = os.Stat(filepath.Join(dir, "test"))
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteFile_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "delete_file"
	request.Params.Arguments = map[string]any{
		"path": "/nonexistent",
	}

	result, err := handler.handleDeleteFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "no such file or directory")
}

// Test de handleSearchFiles
func TestSearchFiles_Valid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test1.txt"), []byte("test content"), 0644)
	os.WriteFile(filepath.Join(dir, "test2.txt"), []byte("other content"), 0644)
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_files"
	request.Params.Arguments = map[string]any{
		"path": dir,
		"pattern": "*.txt",
	}

	result, err := handler.handleSearchFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "test1.txt")
	assert.Contains(t, fmt.Sprint(result.Content[0]), "test2.txt")
}

func TestSearchFiles_NoResults(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search_files"
	request.Params.Arguments = map[string]any{
		"path": dir,
		"pattern": "*.jpg",
	}

	result, err := handler.handleSearchFiles(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "No files found")
}

// Test de handleTree
func TestTree_Valid(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir1/subsubdir"), 0755)
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dir, "subdir1/file2.txt"), []byte("test"), 0644)
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "tree"
	request.Params.Arguments = map[string]any{
		"path": dir,
	}

	result, err := handler.handleTree(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "file1.txt")
	assert.Contains(t, fmt.Sprint(result.Content[0]), "subdir1")
	assert.Contains(t, fmt.Sprint(result.Content[0]), "file2.txt")
}

func TestTree_InvalidPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "tree"
	request.Params.Arguments = map[string]any{
		"path": "/nonexistent",
	}

	result, err := handler.handleTree(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "no such file or directory")
}
