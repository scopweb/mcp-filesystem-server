package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scopweb/mcp-filesystem-go-light/internal/dashboardapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardRootPreservesQueryValues(t *testing.T) {
	handler := newDashboardHandler(t.TempDir(), t.TempDir(), ":8091")
	req := httptest.NewRequest(http.MethodGet, "/?tool=read_file&request_id=req-42&sort=asc&limit=5&recent_offset=10&errors_offset=15&request_offset=20", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `value="read_file"`)
	assert.Contains(t, body, `value="req-42"`)
	assert.Contains(t, body, `<option value="asc" selected>Oldest first</option>`)
	assert.Contains(t, body, `value="5"`)
	assert.Contains(t, body, `let requestOffset =`)
	assert.Contains(t, body, `20 ;`)
}

func TestDashboardRecentEndpointFiltersByTool(t *testing.T) {
	logDir := t.TempDir()
	writeAuditLog(t, logDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","op_id":"1","request_id":"req-1","kind":"tool","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok","args_raw":{"filepath":"a.txt"},"args_normalized":{"path":"a.txt"},"internal_action":"normalized","sub_operations":["normalize_arguments","read.parse_arguments"]}`,
		`{"ts":"2026-03-24T10:30:00Z","op_id":"3","request_id":"req-1b","kind":"tool","tool":"read_file","path":"a2.txt","duration_ms":6,"status":"ok"}`,
		`{"ts":"2026-03-24T11:00:00Z","op_id":"2","request_id":"req-2","kind":"tool","tool":"write_file","path":"b.txt","duration_ms":9,"status":"error","error":"boom"}`,
	})
	handler := newDashboardHandler(logDir, t.TempDir(), ":8091")
	req := httptest.NewRequest(http.MethodGet, "/api/recent?tool=read_file&limit=1&offset=1&sort=desc", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var payload dashboardapi.AuditPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, 2, payload.Total)
	assert.Equal(t, 1, payload.Offset)
	assert.Equal(t, 1, payload.Limit)
	require.Len(t, payload.Items, 1)
	assert.Equal(t, "read_file", payload.Items[0].Tool)
	assert.Equal(t, "a.txt", payload.Items[0].Path)
	assert.Equal(t, "a.txt", payload.Items[0].ArgsRaw["filepath"])
	assert.Equal(t, "a.txt", payload.Items[0].ArgsNormalized["path"])
	assert.Equal(t, "normalized", payload.Items[0].InternalAction)
	assert.Equal(t, []string{"normalize_arguments", "read.parse_arguments"}, payload.Items[0].SubOperations)
}

func TestDashboardErrorsEndpointReturnsOnlyErrors(t *testing.T) {
	logDir := t.TempDir()
	writeAuditLog(t, logDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","op_id":"1","request_id":"req-1","kind":"tool","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok"}`,
		`{"ts":"2026-03-24T10:30:00Z","op_id":"3","request_id":"req-3","kind":"tool","tool":"delete_file","path":"c.txt","duration_ms":8,"status":"error","error":"bad delete"}`,
		`{"ts":"2026-03-24T11:00:00Z","op_id":"2","request_id":"req-2","kind":"tool","tool":"write_file","path":"b.txt","duration_ms":9,"status":"error","error":"boom"}`,
	})
	handler := newDashboardHandler(logDir, t.TempDir(), ":8091")
	req := httptest.NewRequest(http.MethodGet, "/api/errors?limit=1&offset=1&sort=desc", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var payload dashboardapi.AuditPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, 2, payload.Total)
	assert.Equal(t, 1, payload.Offset)
	require.Len(t, payload.Items, 1)
	assert.Equal(t, "error", payload.Items[0].Status)
	assert.Equal(t, "delete_file", payload.Items[0].Tool)
}

func TestDashboardRequestEndpointCorrelatesServerAndProxy(t *testing.T) {
	logDir := t.TempDir()
	proxyDir := t.TempDir()
	writeAuditLog(t, logDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","op_id":"1","request_id":"req-42","kind":"tool","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok","sub_operations":["read.parse_arguments"]}`,
		`{"ts":"2026-03-24T11:00:00Z","op_id":"2","request_id":"req-9","kind":"tool","tool":"write_file","path":"b.txt","duration_ms":9,"status":"error","error":"boom"}`,
	})
	writeProxyLog(t, proxyDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","request_id":"req-42","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok"}`,
		`{"ts":"2026-03-24T11:00:00Z","request_id":"req-9","tool":"write_file","path":"b.txt","duration_ms":9,"status":"error","error":"boom"}`,
	})
	handler := newDashboardHandler(logDir, proxyDir, ":8091")
	req := httptest.NewRequest(http.MethodGet, "/api/request?id=req-42&tool=read_file", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var payload dashboardapi.RequestPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, "req-42", payload.RequestID)
	assert.Equal(t, 1, payload.Server.Total)
	assert.Equal(t, 1, payload.Proxy.Total)
	require.Len(t, payload.Server.Items, 1)
	require.Len(t, payload.Proxy.Items, 1)
	assert.Equal(t, "read_file", payload.Server.Items[0].Tool)
	assert.Equal(t, "read_file", payload.Proxy.Items[0].Tool)
}

func TestDashboardRequestEndpointPaginatesLongTrace(t *testing.T) {
	logDir := t.TempDir()
	proxyDir := t.TempDir()
	writeAuditLog(t, logDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","op_id":"1","request_id":"req-42","kind":"tool","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok"}`,
		`{"ts":"2026-03-24T10:01:00Z","op_id":"2","request_id":"req-42","kind":"tool","tool":"read_file","path":"b.txt","duration_ms":6,"status":"ok"}`,
	})
	writeProxyLog(t, proxyDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","request_id":"req-42","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok"}`,
		`{"ts":"2026-03-24T10:01:00Z","request_id":"req-42","tool":"read_file","path":"b.txt","duration_ms":6,"status":"ok"}`,
	})
	handler := newDashboardHandler(logDir, proxyDir, ":8091")
	req := httptest.NewRequest(http.MethodGet, "/api/request?id=req-42&tool=read_file&limit=1&offset=1&sort=desc", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var payload dashboardapi.RequestPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, 2, payload.Server.Total)
	assert.Equal(t, 2, payload.Proxy.Total)
	assert.Equal(t, 1, payload.Server.Offset)
	assert.Equal(t, 1, payload.Proxy.Offset)
	require.Len(t, payload.Server.Items, 1)
	require.Len(t, payload.Proxy.Items, 1)
	assert.Equal(t, "a.txt", payload.Server.Items[0].Path)
	assert.Equal(t, "a.txt", payload.Proxy.Items[0].Path)
}

func TestDashboardRequestDetailPageRendersEntries(t *testing.T) {
	logDir := t.TempDir()
	proxyDir := t.TempDir()
	writeAuditLog(t, logDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","op_id":"1","request_id":"req-42","kind":"tool","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok","sub_operations":["read.parse_arguments"]}`,
		`{"ts":"2026-03-24T10:01:00Z","op_id":"2","request_id":"req-42","kind":"tool","tool":"read_file","path":"b.txt","duration_ms":6,"status":"ok","sub_operations":["read.parse_arguments"]}`,
	})
	writeProxyLog(t, proxyDir, []string{
		`{"ts":"2026-03-24T10:00:00Z","request_id":"req-42","tool":"read_file","path":"a.txt","duration_ms":5,"status":"ok"}`,
		`{"ts":"2026-03-24T10:01:00Z","request_id":"req-42","tool":"read_file","path":"b.txt","duration_ms":6,"status":"ok"}`,
	})
	handler := newDashboardHandler(logDir, proxyDir, ":8091")
	req := httptest.NewRequest(http.MethodGet, "/request?id=req-42&tool=read_file&limit=1&offset=1&sort=desc", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Request Detail")
	assert.Contains(t, body, "request_id=req-42")
	assert.Contains(t, body, "Showing 2-2 of 2 · page 2 of 2")
	assert.Contains(t, body, "a.txt")
	assert.NotContains(t, body, "b.txt")
	assert.Contains(t, body, "Back to dashboard")
	assert.Contains(t, body, "offset=0")
}

func writeAuditLog(t *testing.T, dir string, lines []string) {
	t.Helper()
	path := filepath.Join(dir, "operations.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func writeProxyLog(t *testing.T, dir string, lines []string) {
	t.Helper()
	path := filepath.Join(dir, "proxy.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}
