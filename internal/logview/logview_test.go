package logview

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterAuditEntriesByTool(t *testing.T) {
	entries := []AuditEntry{
		{Tool: "read_file", Kind: "tool"},
		{Tool: "write_file", Kind: "tool"},
		{Kind: "resource"},
	}

	filtered := FilterAuditEntries(entries, "read_file")
	require.Len(t, filtered, 1)
	assert.Equal(t, "read_file", filtered[0].Tool)
}

func TestBuildMetricsFromEntries(t *testing.T) {
	entries := []AuditEntry{
		{Tool: "read_file", Kind: "tool", Status: "ok", DurationMs: 10},
		{Tool: "read_file", Kind: "tool", Status: "error", DurationMs: 30},
		{Tool: "write_file", Kind: "tool", Status: "ok", DurationMs: 20},
	}

	metrics := BuildMetricsFromEntries(entries)
	require.NotNil(t, metrics)
	assert.Equal(t, int64(3), metrics.OpsTotal)
	assert.Equal(t, int64(2), metrics.OpsOK)
	assert.Equal(t, int64(1), metrics.OpsError)
	assert.Equal(t, 20.0, metrics.AvgDurationMs)
	assert.Equal(t, int64(2), metrics.ByTool["read_file"].Count)
	assert.Equal(t, int64(1), metrics.ByTool["read_file"].Errors)
	assert.Equal(t, 20.0, metrics.ByTool["read_file"].AvgDurationMs)
}

func TestReadAuditEntriesSortsNewestFirst(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "operations.jsonl")
	content := "{\"ts\":\"2026-03-24T10:00:00Z\",\"op_id\":\"1\",\"kind\":\"tool\",\"tool\":\"read_file\",\"duration_ms\":5,\"status\":\"ok\"}\n" +
		"{\"ts\":\"2026-03-24T11:00:00Z\",\"op_id\":\"2\",\"kind\":\"tool\",\"tool\":\"write_file\",\"duration_ms\":7,\"status\":\"ok\"}\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	entries, err := ReadAuditEntries(path)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "2", entries[0].OperationID)
	assert.True(t, entries[0].Timestamp.After(entries[1].Timestamp))
}

func TestReadAuditEntriesParsesArguments(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "operations.jsonl")
	content := "{\"ts\":\"2026-03-24T10:00:00Z\",\"op_id\":\"1\",\"kind\":\"tool\",\"tool\":\"read_file\",\"duration_ms\":5,\"status\":\"ok\",\"args_raw\":{\"path\":\"a.txt\"},\"args_normalized\":{\"path\":\"a.txt\",\"start_line\":1},\"internal_action\":\"normalized\",\"sub_operations\":[\"read.parse_arguments\"]}\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	entries, err := ReadAuditEntries(path)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "a.txt", entries[0].ArgsRaw["path"])
	assert.Equal(t, float64(1), entries[0].ArgsNormalized["start_line"])
	assert.Equal(t, "normalized", entries[0].InternalAction)
	assert.Equal(t, []string{"read.parse_arguments"}, entries[0].SubOperations)
}

func TestDisplayToolFallsBackToKind(t *testing.T) {
	entry := AuditEntry{Kind: "resource"}
	assert.Equal(t, "resource", DisplayTool(entry))
}

func TestReadMetrics(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "metrics.json")
	data := `{"updated_at":"2026-03-24T12:00:00Z","ops_total":4,"ops_ok":3,"ops_error":1,"avg_duration_ms":12.5}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	metrics, err := ReadMetrics(path)
	require.NoError(t, err)
	assert.Equal(t, int64(4), metrics.OpsTotal)
	assert.Equal(t, int64(3), metrics.OpsOK)
	assert.WithinDuration(t, time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC), metrics.UpdatedAt, time.Second)
}
