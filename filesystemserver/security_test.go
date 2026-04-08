package filesystemserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Path Validation — Directory Traversal
// ─────────────────────────────────────────────────────────────────────────────

func TestValidatePath_DirectoryTraversal_DotDot(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	attacks := []string{
		filepath.Join(dir, "..", "etc", "passwd"),
		filepath.Join(dir, "..", "..", "..", "etc", "passwd"),
		filepath.Join(dir, "subdir", "..", "..", "secret.txt"),
	}

	for _, path := range attacks {
		_, err := handler.validatePath(path)
		assert.Error(t, err, "path %q should be rejected", path)
		if err != nil {
			assert.Contains(t, err.Error(), "access denied", "path %q error message", path)
		}
	}
}

func TestValidatePath_DirectoryTraversal_ViaReadFile(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{
		"path": filepath.Join(dir, "..", "..", "etc", "passwd"),
	}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied")
}

func TestValidatePath_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	_, err = handler.validatePath("")
	if err == nil {
		t.Log("empty path resolved without error — cwd may be inside allowed dirs")
	}
}

func TestValidatePath_DotPath(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	_, err = handler.validatePath(".")
	if err != nil {
		assert.Contains(t, err.Error(), "access denied")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Path Validation — Symlinks
// ─────────────────────────────────────────────────────────────────────────────

func TestValidatePath_SymlinkOutsideAllowedDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	allowed := t.TempDir()
	outside := t.TempDir()

	secretFile := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret"), 0644))

	linkPath := filepath.Join(allowed, "evil_link")
	require.NoError(t, os.Symlink(secretFile, linkPath))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	_, err = handler.validatePath(linkPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied - symlink target outside allowed directories")
}

func TestValidatePath_SymlinkDirOutsideAllowedDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	allowed := t.TempDir()
	outside := t.TempDir()

	linkPath := filepath.Join(allowed, "evil_dir_link")
	require.NoError(t, os.Symlink(outside, linkPath))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	_, err = handler.validatePath(filepath.Join(linkPath, "file.txt"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestValidatePath_SymlinkInsideAllowedDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	allowed := t.TempDir()

	realFile := filepath.Join(allowed, "real.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("ok"), 0644))
	linkPath := filepath.Join(allowed, "safe_link")
	require.NoError(t, os.Symlink(realFile, linkPath))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	resolved, err := handler.validatePath(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, realFile, resolved)
}

func TestValidatePath_NestedSymlinkChain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	allowed := t.TempDir()
	outside := t.TempDir()

	secretFile := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret"), 0644))

	link2 := filepath.Join(allowed, "link2")
	require.NoError(t, os.Symlink(secretFile, link2))

	link1 := filepath.Join(allowed, "link1")
	require.NoError(t, os.Symlink(link2, link1))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	_, err = handler.validatePath(link1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestReadFile_ViaSymlinkOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	allowed := t.TempDir()
	outside := t.TempDir()

	secretFile := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("TOP SECRET"), 0644))

	linkPath := filepath.Join(allowed, "sneaky")
	require.NoError(t, os.Symlink(secretFile, linkPath))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{"path": linkPath}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied")
}

// ─────────────────────────────────────────────────────────────────────────────
// Path Validation — Windows-specific
// ─────────────────────────────────────────────────────────────────────────────

func TestValidatePath_BackslashNormalization(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("ok"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	forwardSlashPath := strings.ReplaceAll(testFile, `\`, `/`)
	resolved, err := handler.validatePath(forwardSlashPath)
	assert.NoError(t, err)
	assert.Equal(t, testFile, resolved)
}

func TestValidatePath_WindowsTraversalWithBackslash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	attack := dir + `\..\..\Windows\System32\config\SAM`
	_, err = handler.validatePath(attack)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestValidatePath_UNCPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	_, err = handler.validatePath(`\\server\share\file.txt`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

// ─────────────────────────────────────────────────────────────────────────────
// Path Validation — WSL paths
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeWSLPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WSL path normalization only applies on Windows")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"/mnt/c/Users/DAVID", `C:\Users\DAVID`},
		{"/mnt/c/Users/DAVID/Documents", `C:\Users\DAVID\Documents`},
		{"/mnt/d/Projects/repo", `D:\Projects\repo`},
		{"/mnt/c", `C:\`},
		{"/mnt/C/Users", `C:\Users`},
		{"C:\\Users\\DAVID", `C:\Users\DAVID`},
		{"/mnt/", "/mnt/"},
		{"/mnt/cifs/share", "/mnt/cifs/share"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeWSLPath(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestValidatePath_WSLStyle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WSL path test only applies on Windows")
	}

	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	wslPath := "/mnt/" + strings.ToLower(strings.ReplaceAll(
		strings.Replace(dir, ":", "", 1),
		`\`, "/",
	))

	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello"), 0644))

	wslFilePath := wslPath + "/test.txt"
	resolved, err := handler.validatePath(wslFilePath)
	assert.NoError(t, err, "path WSL dins allowed dir hauria de ser acceptat")
	assert.Equal(t, testFile, resolved)
}

// ─────────────────────────────────────────────────────────────────────────────
// Path Validation — Edge Cases
// ─────────────────────────────────────────────────────────────────────────────

func TestValidatePath_UnicodeFilenames(t *testing.T) {
	dir := t.TempDir()

	unicodeFile := filepath.Join(dir, "café_résumé.txt")
	require.NoError(t, os.WriteFile(unicodeFile, []byte("unicode content"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	resolved, err := handler.validatePath(unicodeFile)
	assert.NoError(t, err)
	assert.Equal(t, unicodeFile, resolved)

	request := mcp.CallToolRequest{}
	request.Params.Name = "read_file"
	request.Params.Arguments = map[string]any{"path": unicodeFile}

	result, err := handler.handleReadFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "unicode content", result.Content[0].(mcp.TextContent).Text)
}

func TestValidatePath_SpacesInPath(t *testing.T) {
	dir := t.TempDir()

	spacePath := filepath.Join(dir, "path with spaces", "file name.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(spacePath), 0755))
	require.NoError(t, os.WriteFile(spacePath, []byte("spaced"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	resolved, err := handler.validatePath(spacePath)
	assert.NoError(t, err)
	assert.Equal(t, spacePath, resolved)
}

func TestValidatePath_DeepNesting(t *testing.T) {
	dir := t.TempDir()

	nested := dir
	for i := 0; i < 10; i++ {
		nested = filepath.Join(nested, fmt.Sprintf("level%d", i))
	}
	require.NoError(t, os.MkdirAll(nested, 0755))
	deepFile := filepath.Join(nested, "deep.txt")
	require.NoError(t, os.WriteFile(deepFile, []byte("deep"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	resolved, err := handler.validatePath(deepFile)
	assert.NoError(t, err)
	assert.Equal(t, deepFile, resolved)
}

func TestValidatePath_NonExistentFileInAllowedDir(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	newFile := filepath.Join(dir, "new_file.txt")
	resolved, err := handler.validatePath(newFile)
	assert.NoError(t, err)
	assert.Equal(t, newFile, resolved)
}

func TestValidatePath_AllowedDirItself(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	resolved, err := handler.validatePath(dir)
	assert.NoError(t, err)
	assert.Equal(t, dir, resolved)
}

// ─────────────────────────────────────────────────────────────────────────────
// Parameter Validation — Null / Missing
// ─────────────────────────────────────────────────────────────────────────────

func TestEditFile_NullParams(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "null old_text",
			args: map[string]any{"path": filepath.Join(dir, "f.txt"), "old_text": nil, "new_text": "x"},
			want: "null",
		},
		{
			name: "missing old_text",
			args: map[string]any{"path": filepath.Join(dir, "f.txt"), "new_text": "x"},
			want: "missing required parameter",
		},
		{
			name: "missing path",
			args: map[string]any{"old_text": "a", "new_text": "b"},
			want: "missing required parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{}
			request.Params.Name = "edit_file"
			request.Params.Arguments = tt.args

			result, err := handler.handleEditFile(context.Background(), request)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, fmt.Sprint(result.Content[0]), tt.want)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Roots — Refresh, Error, Merge
// ─────────────────────────────────────────────────────────────────────────────

func TestRefreshRoots_ReplacesAllowedDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	handler, err := NewFilesystemHandler([]string{dir1})
	require.NoError(t, err)

	assert.Len(t, handler.allowedDirs, 1)

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return []string{dir2}, nil
	}

	handler.refreshRoots(context.Background())

	handler.mu.RLock()
	dirs := handler.allowedDirs
	handler.mu.RUnlock()

	assert.Len(t, dirs, 1)
	assert.True(t, strings.HasPrefix(dirs[0], filepath.Clean(dir2)))
}

func TestRefreshRoots_ErrorKeepsExistingDirs(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	originalDirs := handler.allowedDirs

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return nil, fmt.Errorf("connection lost")
	}

	handler.refreshRoots(context.Background())
	assert.Equal(t, originalDirs, handler.allowedDirs)
}

func TestRefreshRoots_EmptyResultKeepsExistingDirs(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	originalDirs := handler.allowedDirs

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return []string{}, nil
	}

	handler.refreshRoots(context.Background())
	assert.Equal(t, originalDirs, handler.allowedDirs)
}

func TestRefreshRoots_NilFnIsNoop(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	handler.requestRootsFn = nil
	handler.refreshRoots(context.Background())
	assert.Len(t, handler.allowedDirs, 1)
}

func TestRefreshRoots_MultipleRoots(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()

	handler, err := NewFilesystemHandler(nil)
	require.NoError(t, err)

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return []string{dir1, dir2, dir3}, nil
	}

	handler.refreshRoots(context.Background())

	handler.mu.RLock()
	dirs := handler.allowedDirs
	handler.mu.RUnlock()

	assert.Len(t, dirs, 3)
}

func TestTryFetchRoots_OnlyRunsOnce(t *testing.T) {
	handler, err := NewFilesystemHandler(nil)
	require.NoError(t, err)

	callCount := 0
	dir := t.TempDir()
	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		callCount++
		return []string{dir}, nil
	}

	assert.True(t, handler.tryFetchRoots(context.Background()))
	assert.Equal(t, 1, callCount)

	assert.False(t, handler.tryFetchRoots(context.Background()))
	assert.Equal(t, 1, callCount)
}

func TestTryFetchRoots_ErrorReturnsFalse(t *testing.T) {
	handler, err := NewFilesystemHandler(nil)
	require.NoError(t, err)

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return nil, fmt.Errorf("client disconnected")
	}

	fetched := handler.tryFetchRoots(context.Background())
	assert.False(t, fetched)
	assert.True(t, handler.rootsFetched)
}

func TestTryFetchRoots_NilFnReturnsFalse(t *testing.T) {
	handler, err := NewFilesystemHandler(nil)
	require.NoError(t, err)

	handler.requestRootsFn = nil
	fetched := handler.tryFetchRoots(context.Background())
	assert.False(t, fetched)
}

// ─────────────────────────────────────────────────────────────────────────────
// Concurrency
// ─────────────────────────────────────────────────────────────────────────────

func TestConcurrentReadFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "concurrent.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("concurrent content"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request := mcp.CallToolRequest{}
			request.Params.Name = "read_file"
			request.Params.Arguments = map[string]any{"path": testFile}

			result, err := handler.handleReadFile(context.Background(), request)
			if err != nil {
				errors <- err
				return
			}
			if result.IsError {
				errors <- fmt.Errorf("unexpected tool error: %v", result.Content[0])
				return
			}
			text := result.Content[0].(mcp.TextContent).Text
			if text != "concurrent content" {
				errors <- fmt.Errorf("unexpected content: %q", text)
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentValidatePath(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("ok"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resolved, err := handler.validatePath(testFile)
			assert.NoError(t, err)
			assert.Equal(t, testFile, resolved)
		}()
	}
	wg.Wait()
}

func TestConcurrentRefreshRoots(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	handler, err := NewFilesystemHandler([]string{dir1})
	require.NoError(t, err)

	handler.requestRootsFn = func(ctx context.Context) ([]string, error) {
		return []string{dir2}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.refreshRoots(context.Background())
		}()
	}
	wg.Wait()

	handler.mu.RLock()
	dirs := handler.allowedDirs
	handler.mu.RUnlock()
	assert.NotEmpty(t, dirs)
}

// ─────────────────────────────────────────────────────────────────────────────
// Resource Handler
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleReadResource_TextFile(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.ReadResourceRequest{}
	request.Params.URI = "file://" + testFile

	contents, err := handler.handleReadResource(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	textContent, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "hello world", textContent.Text)
}

func TestHandleReadResource_Directory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0755))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.ReadResourceRequest{}
	request.Params.URI = "file://" + dir

	contents, err := handler.handleReadResource(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, contents, 1)

	textContent, ok := contents[0].(mcp.TextResourceContents)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "a.txt")
	assert.Contains(t, textContent.Text, "[DIR]")
	assert.Contains(t, textContent.Text, "subdir")
}

func TestHandleReadResource_InvalidScheme(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.ReadResourceRequest{}
	request.Params.URI = "https://example.com/file"

	_, err = handler.handleReadResource(context.Background(), request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported URI scheme")
}

func TestHandleReadResource_OutsideAllowedDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "secret.txt"), []byte("secret"), 0644))

	handler, err := NewFilesystemHandler([]string{dir1})
	require.NoError(t, err)

	request := mcp.ReadResourceRequest{}
	request.Params.URI = "file://" + filepath.Join(dir2, "secret.txt")

	_, err = handler.handleReadResource(context.Background(), request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

// ─────────────────────────────────────────────────────────────────────────────
// Multiple Allowed Directories
// ─────────────────────────────────────────────────────────────────────────────

func TestMultipleAllowedDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir1, "f1.txt"), []byte("one"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "f2.txt"), []byte("two"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir3, "f3.txt"), []byte("three"), 0644))

	handler, err := NewFilesystemHandler([]string{dir1, dir2})
	require.NoError(t, err)

	_, err = handler.validatePath(filepath.Join(dir1, "f1.txt"))
	assert.NoError(t, err)

	_, err = handler.validatePath(filepath.Join(dir2, "f2.txt"))
	assert.NoError(t, err)

	_, err = handler.validatePath(filepath.Join(dir3, "f3.txt"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

// ─────────────────────────────────────────────────────────────────────────────
// Edit — dry_run, already_present, large edit tip
// ─────────────────────────────────────────────────────────────────────────────

func TestEditFile_DryRun(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "dry.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "edit_file"
	request.Params.Arguments = map[string]any{
		"path":     filePath,
		"old_text": "hello",
		"new_text": "goodbye",
		"dry_run":  true,
	}

	result, err := handler.handleEditFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Dry run")

	content, _ := os.ReadFile(filePath)
	assert.Equal(t, "hello world", string(content))
}

func TestEditFile_AlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "present.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("new content here"), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "edit_file"
	request.Params.Arguments = map[string]any{
		"path":     filePath,
		"old_text": "old stuff",
		"new_text": "new content here",
	}

	result, err := handler.handleEditFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "already")
}

func TestEditFile_LargeEditTip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.txt")

	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d: old value", i))
	}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644))

	handler, err := NewFilesystemHandler([]string{dir})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "edit_file"
	request.Params.Arguments = map[string]any{
		"path":     filePath,
		"old_text": strings.Join(lines, "\n"),
		"new_text": strings.ReplaceAll(strings.Join(lines, "\n"), "old value", "new value"),
	}

	result, err := handler.handleEditFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "(+20 -20) | 20 lines")
}

// ─────────────────────────────────────────────────────────────────────────────
// Copy / Move — path security
// ─────────────────────────────────────────────────────────────────────────────

func TestCopyFile_DestOutsideAllowed(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()

	srcFile := filepath.Join(allowed, "src.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("data"), 0644))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "copy_file"
	request.Params.Arguments = map[string]any{
		"source":      srcFile,
		"destination": filepath.Join(outside, "stolen.txt"),
	}

	result, err := handler.handleCopyFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied")
}

func TestMoveFile_DestOutsideAllowed(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()

	srcFile := filepath.Join(allowed, "src.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("data"), 0644))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "move_file"
	request.Params.Arguments = map[string]any{
		"source":      srcFile,
		"destination": filepath.Join(outside, "exfiltrated.txt"),
	}

	result, err := handler.handleMoveFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "access denied")

	_, statErr := os.Stat(srcFile)
	assert.NoError(t, statErr)
}

// ─────────────────────────────────────────────────────────────────────────────
// Protected allowed directories — cannot delete or move root paths
// ─────────────────────────────────────────────────────────────────────────────

func TestDeleteFile_CannotDeleteAllowedDir(t *testing.T) {
	allowed := t.TempDir()

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "delete_file"
	request.Params.Arguments = map[string]any{
		"path":      allowed,
		"recursive": true,
	}

	result, err := handler.handleDeleteFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "protected root path")

	_, statErr := os.Stat(allowed)
	assert.NoError(t, statErr)
}

func TestMoveFile_CannotMoveAllowedDir(t *testing.T) {
	allowed := t.TempDir()
	dest := filepath.Join(t.TempDir(), "moved")

	handler, err := NewFilesystemHandler([]string{allowed, filepath.Dir(dest)})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "move_file"
	request.Params.Arguments = map[string]any{
		"source":      allowed,
		"destination": dest,
	}

	result, err := handler.handleMoveFile(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, fmt.Sprint(result.Content[0]), "protected root path")

	_, statErr := os.Stat(allowed)
	assert.NoError(t, statErr)
}

func TestDeleteFile_CanDeleteSubdirOfAllowed(t *testing.T) {
	allowed := t.TempDir()
	subdir := filepath.Join(allowed, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	handler, err := NewFilesystemHandler([]string{allowed})
	require.NoError(t, err)

	request := mcp.CallToolRequest{}
	request.Params.Name = "delete_file"
	request.Params.Arguments = map[string]any{
		"path":      subdir,
		"recursive": true,
	}

	result, err := handler.handleDeleteFile(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	_, statErr := os.Stat(subdir)
	assert.True(t, os.IsNotExist(statErr))
	_, statErr = os.Stat(allowed)
	assert.NoError(t, statErr)
}
