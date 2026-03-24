package filesystemserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLoggerCapturesRawAndNormalizedArgs(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0644))

	handler, err := NewFilesystemHandler([]string{tempDir})
	require.NoError(t, err)

	logger, err := NewAuditLogger(tempDir)
	require.NoError(t, err)
	handler.SetAuditLogger(logger)
	t.Cleanup(func() {
		require.NoError(t, logger.Close())
	})

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"file_path": filePath,
			},
		},
	}

	wrapped := handler.withNormalize("read_file", handler.handleReadFile)
	_, err = wrapped(context.Background(), request)
	require.NoError(t, err)
	require.NoError(t, logger.Close())

	entry := readLastAuditEntry(t, filepath.Join(tempDir, "operations.jsonl"))
	assert.Equal(t, "tool", entry.Kind)
	assert.Equal(t, "read_file", entry.Tool)
	assert.Equal(t, "ok", entry.Status)
	assert.Equal(t, filePath, entry.Path)
	assert.Equal(t, filePath, entry.ArgsRaw["file_path"])
	assert.Equal(t, filePath, entry.ArgsNormalized["path"])
	assert.Equal(t, "normalized", entry.InternalAction)
	_, hasRawPath := entry.ArgsRaw["path"]
	assert.False(t, hasRawPath)
}

func TestAuditLoggerCapturesToolErrors(t *testing.T) {
	tempDir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{tempDir})
	require.NoError(t, err)

	logger, err := NewAuditLogger(tempDir)
	require.NoError(t, err)
	handler.SetAuditLogger(logger)
	t.Cleanup(func() {
		require.NoError(t, logger.Close())
	})

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "read_file",
			Arguments: map[string]interface{}{
				"path": filepath.Join(tempDir, "missing.txt"),
			},
		},
	}

	wrapped := handler.withNormalize("read_file", handler.handleReadFile)
	_, err = wrapped(context.Background(), request)
	require.NoError(t, err)
	require.NoError(t, logger.Close())

	entry := readLastAuditEntry(t, filepath.Join(tempDir, "operations.jsonl"))
	assert.Equal(t, "error", entry.Status)
	assert.True(t, entry.ResultIsError)
	assert.NotEmpty(t, entry.Error)
}

func TestAuditLoggerCapturesRequestIDAndSubOperations(t *testing.T) {
	tempDir := t.TempDir()
	handler, err := NewFilesystemHandler([]string{tempDir})
	require.NoError(t, err)

	logger, err := NewAuditLogger(tempDir)
	require.NoError(t, err)
	handler.SetAuditLogger(logger)
	t.Cleanup(func() {
		require.NoError(t, logger.Close())
	})

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "list_allowed_directories",
			Arguments: map[string]interface{}{},
			Meta: &mcp.Meta{AdditionalFields: map[string]interface{}{
				"request_id": "req-42",
			}},
		},
	}

	wrapped := handler.withAudit("list_allowed_directories", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		appendSubOperation(ctx, "custom.prepare")
		appendSubOperation(ctx, "custom.finish")
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "ok"}}}, nil
	})

	_, err = wrapped(context.Background(), request)
	require.NoError(t, err)
	require.NoError(t, logger.Close())

	entry := readLastAuditEntry(t, filepath.Join(tempDir, "operations.jsonl"))
	assert.Equal(t, "req-42", entry.RequestID)
	assert.Equal(t, []string{"custom.prepare", "custom.finish"}, entry.SubOperations)
}

func readLastAuditEntry(t *testing.T, path string) AuditEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.NotEmpty(t, lines)

	var entry AuditEntry
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &entry))
	return entry
}
