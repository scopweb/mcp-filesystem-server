package filesystemserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Error:")
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

func TestEditFile_InvalidPathReturnsToolError(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	missingParentPath := filepath.Join(dir, "missing", "edit_test.txt")
	request := mcp.CallToolRequest{}
	request.Params.Name = "edit_file"
	request.Params.Arguments = map[string]any{
		"path":     missingParentPath,
		"old_text": "before",
		"new_text": "after",
	}

	result, err := handler.handleEditFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	// validatePath now walks up to find an existing ancestor, so the path is accepted.
	// edit_file then fails because the file doesn't exist on disk.
	errText := result.Content[0].(mcp.TextContent).Text
	assert.True(t,
		strings.Contains(errText, "no puede encontrar") ||
			strings.Contains(errText, "no such file") ||
			strings.Contains(errText, "cannot find") ||
			strings.Contains(errText, "system cannot find"),
		"expected file-not-found error, got: %s", errText)
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
		"path":    filepath.Join(dir, "test"),
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
		"path":    "/forbidden/path",
		"content": "test",
	}

	result, err := handler.handleWriteFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied - path outside allowed directories")
}

func TestTryFetchRoots_SkipsWhenAllowedDirsConfigured(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	called := false
	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		called = true
		return []string{t.TempDir()}, nil
	}

	fetched := handler.tryFetchRoots(context.Background())

	assert.False(t, fetched)
	assert.False(t, called)
	assert.False(t, handler.rootsFetched)
	assert.Len(t, handler.allowedDirs, 1)
}

func TestTryFetchRoots_LoadsDynamicRootsWithoutAllowedDirs(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler(nil)
	require.NoError(t, err)

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return []string{dir}, nil
	}

	fetched := handler.tryFetchRoots(context.Background())

	assert.True(t, fetched)
	assert.True(t, handler.rootsFetched)
	if assert.Len(t, handler.allowedDirs, 1) {
		assert.Equal(t, filepath.Clean(dir)+string(filepath.Separator), handler.allowedDirs[0])
	}
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
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Error:")
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
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Error:")
}

// Test de handleSearchFiles
func TestSearchFiles_Valid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test1.txt"), []byte("test content"), 0644)
	os.WriteFile(filepath.Join(dir, "test2.txt"), []byte("other content"), 0644)
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "search"
	request.Params.Arguments = map[string]any{
		"path":    dir,
		"mode":    "files",
		"pattern": "*.txt",
	}

	result, err := handler.handleSearch(context.Background(), request)
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
	request.Params.Name = "search"
	request.Params.Arguments = map[string]any{
		"path":    dir,
		"mode":    "files",
		"pattern": "*.jpg",
	}

	result, err := handler.handleSearch(context.Background(), request)
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
	assert.Contains(t, fmt.Sprint(result.Content[0]), "Error:")
}

func TestReadfile_Outline_Go(t *testing.T) {
	dir := t.TempDir()
	goCode := `package main

import "fmt"

type Server struct {
	Name string
}

type Handler interface {
	Handle()
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start() {
	fmt.Println("started")
}

var defaultPort = 8080

const maxRetries = 3
`
	err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(goCode), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path":    filepath.Join(dir, "main.go"),
		"outline": true,
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "=== Structs ===")
	assert.Contains(t, text, "=== Interfaces ===")
	assert.Contains(t, text, "=== Functions ===")
	assert.Contains(t, text, "=== Methods ===")
	assert.Contains(t, text, "type Server struct")
	assert.Contains(t, text, "func NewServer()")
	assert.Contains(t, text, "func (s *Server) Start()")
}

func TestReadfile_Outline_Python(t *testing.T) {
	dir := t.TempDir()
	pyCode := `class UserService:
    def __init__(self, db):
        self.db = db

    def get_user(self, user_id):
        return self.db.get(user_id)

def main():
    svc = UserService(None)
`
	err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(pyCode), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path":    filepath.Join(dir, "app.py"),
		"outline": true,
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "=== Classes ===")
	assert.Contains(t, text, "=== Functions ===")
	assert.Contains(t, text, "=== Methods ===")
	assert.Contains(t, text, "class UserService:")
	assert.Contains(t, text, "def main():")
}

func TestReadfile_Outline_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path":    filepath.Join(dir, "notes.txt"),
		"outline": true,
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Outline not supported")
	assert.Contains(t, text, ".txt")
}

func TestReadfile_Outline_NoSymbols(t *testing.T) {
	dir := t.TempDir()
	goCode := `package main

// This file has no symbols, just a comment.
`
	err := os.WriteFile(filepath.Join(dir, "empty.go"), []byte(goCode), 0644)
	require.NoError(t, err)

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path":    filepath.Join(dir, "empty.go"),
		"outline": true,
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "No symbols found")
}

// TestWriteFile_Base64 verifica que write_file decodifica correctament base64
func TestWriteFile_Base64(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	// PNG mínim de 2x2 píxels (binari real)
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02,
		0x08, 0x02, 0x00, 0x00, 0x00, 0xfd, 0xd4, 0x9a,
		0x73, 0x00, 0x00, 0x00, 0x01, 0x73, 0x52, 0x47, // sRGB
		0x42, 0x00, 0xae, 0xce, 0x1c, 0xe9,
	}
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	outPath := filepath.Join(dir, "test.png")

	request := mcp.CallToolRequest{}
	request.Params.Name = "write_file"
	request.Params.Arguments = map[string]any{
		"path":     outPath,
		"content":  encoded,
		"encoding": "base64",
	}

	result, err := handler.handleWriteFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError, "esperava èxit, got: %v", result.Content)

	// Verificar que el fitxer conté els bytes binaris, no el text base64
	written, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, pngBytes, written, "el fitxer hauria de contenir bytes binaris, no text base64")
	assert.NotEqual(t, []byte(encoded), written, "el fitxer NO hauria de contenir el text base64 literal")
}

// TestWriteFile_Base64_InvalidInput verifica que base64 invàlid retorna error
func TestWriteFile_Base64_InvalidInput(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "write_file"
	request.Params.Arguments = map[string]any{
		"path":     filepath.Join(dir, "test.bin"),
		"content":  "això no és base64 vàlid!!!",
		"encoding": "base64",
	}

	result, err := handler.handleWriteFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError, "esperava error per base64 invàlid")
	assert.Contains(t, fmt.Sprint(result.Content[0]), "invalid base64")
}

// TestTree_NoDuplicateContent verifica que tree no retorna JSON duplicat
func TestTree_NoDuplicateContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "tree"
	request.Params.Arguments = map[string]any{
		"path":  dir,
		"depth": float64(2),
	}

	result, err := handler.handleTree(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Ha de retornar exactament 1 content item
	assert.Len(t, result.Content, 1, "tree ha de retornar exactament 1 content item, no duplicat")

	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Directory tree for")
	assert.Contains(t, text, "file.txt")
}
