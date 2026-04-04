package dashboardapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scopweb/mcp-filesystem-go-light/internal/logview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAuditPageDescending(t *testing.T) {
	entries := []logview.AuditEntry{
		{Path: "old.txt"},
		{Path: "mid.txt"},
		{Path: "new.txt"},
	}

	page := BuildAuditPage(entries, "desc", 1, 1)

	assert.Equal(t, 3, page.Total)
	assert.Equal(t, 1, page.Offset)
	assert.Equal(t, 1, page.Limit)
	assert.Equal(t, "desc", page.Sort)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "mid.txt", page.Items[0].Path)
}

func TestBuildAuditPageAscending(t *testing.T) {
	entries := []logview.AuditEntry{
		{Path: "newest.txt"},
		{Path: "older.txt"},
		{Path: "oldest.txt"},
	}

	page := BuildAuditPage(entries, "asc", 0, 2)

	require.Len(t, page.Items, 2)
	assert.Equal(t, "oldest.txt", page.Items[0].Path)
	assert.Equal(t, "older.txt", page.Items[1].Path)
	assert.Equal(t, "asc", page.Sort)
}

func TestBuildProxyPageHandlesNegativeOffset(t *testing.T) {
	entries := []logview.ProxyEntry{{Path: "a.txt"}}

	page := BuildProxyPage(entries, "desc", -5, 10)

	assert.Equal(t, 0, page.Offset)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "a.txt", page.Items[0].Path)
}

func TestBuildRequestPageComposesServerAndProxy(t *testing.T) {
	serverEntries := []logview.AuditEntry{{Path: "server.txt"}}
	proxyEntries := []logview.ProxyEntry{{Path: "proxy.txt"}}

	page := BuildRequestPage("req-7", serverEntries, proxyEntries, "desc", 0, 5)

	assert.Equal(t, "req-7", page.RequestID)
	assert.Equal(t, 1, page.Server.Total)
	assert.Equal(t, 1, page.Proxy.Total)
	require.Len(t, page.Server.Items, 1)
	require.Len(t, page.Proxy.Items, 1)
	assert.Equal(t, "server.txt", page.Server.Items[0].Path)
	assert.Equal(t, "proxy.txt", page.Proxy.Items[0].Path)
}

func TestReadHelpersAndPageSummary(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?limit=7&offset=3&sort=asc", nil)

	assert.Equal(t, 7, ReadLimit(req, 12))
	assert.Equal(t, 3, ReadOffset(req, "offset"))
	assert.Equal(t, "asc", ReadSort(req))
	assert.Equal(t, "Showing 4-6 of 10 · page 2 of 4", PageSummary(10, 3, 3, 3))
}

func TestReadHelpersFallbacks(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?limit=nope&offset=-1&sort=weird", nil)

	assert.Equal(t, 12, ReadLimit(req, 12))
	assert.Equal(t, 0, ReadOffset(req, "offset"))
	assert.Equal(t, "desc", ReadSort(req))
	assert.Equal(t, "Showing 0-0 of 0 · page 0 of 0", PageSummary(0, 0, 12, 0))
}

func TestFilterHelpers(t *testing.T) {
	auditEntries := []logview.AuditEntry{
		{Status: "ok", RequestID: "req-1", Path: "ok.txt"},
		{Status: "error", RequestID: "req-2", Path: "err.txt"},
		{Status: "error", RequestID: "req-1", Path: "req.txt"},
	}
	proxyEntries := []logview.ProxyEntry{
		{RequestID: "req-1", Path: "proxy-a.txt"},
		{RequestID: "req-2", Path: "proxy-b.txt"},
	}

	errorsOnly := FilterErrorEntries(auditEntries)
	reqEntries := FilterRequestEntries(auditEntries, "req-1")
	proxyReqEntries := FilterProxyRequestEntries(proxyEntries, "req-1")

	require.Len(t, errorsOnly, 2)
	assert.Equal(t, "err.txt", errorsOnly[0].Path)
	assert.Equal(t, "req.txt", errorsOnly[1].Path)
	require.Len(t, reqEntries, 2)
	assert.Equal(t, "ok.txt", reqEntries[0].Path)
	assert.Equal(t, "req.txt", reqEntries[1].Path)
	require.Len(t, proxyReqEntries, 1)
	assert.Equal(t, "proxy-a.txt", proxyReqEntries[0].Path)
}

func TestRespondJSONSetsHeaderAndBody(t *testing.T) {
	recorder := httptest.NewRecorder()

	RespondJSON(recorder, map[string]interface{}{"status": "ok", "count": 2})

	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var payload map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &payload)
	require.NoError(t, err)
	assert.Equal(t, "ok", payload["status"])
	assert.Equal(t, float64(2), payload["count"])
	assert.Contains(t, recorder.Body.String(), "\n")
}
