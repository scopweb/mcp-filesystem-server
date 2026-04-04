package main

import (
	"encoding/json"
	"testing"

	"github.com/scopweb/mcp-filesystem-go-light/internal/logview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSummaryJSONIncludesRecentEntries(t *testing.T) {
	metrics := &logview.MetricsSnapshot{OpsTotal: 2, OpsOK: 2}
	entries := []logview.AuditEntry{
		{Tool: "read_file", Status: "ok", DurationMs: 10},
		{Tool: "write_file", Status: "ok", DurationMs: 20},
	}

	output, err := renderSummaryJSON(metrics, entries, 1)
	require.NoError(t, err)

	var payload struct {
		Metrics logview.MetricsSnapshot `json:"metrics"`
		Recent  []logview.AuditEntry    `json:"recent"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &payload))
	assert.Equal(t, int64(2), payload.Metrics.OpsTotal)
	require.Len(t, payload.Recent, 1)
	assert.Equal(t, "read_file", payload.Recent[0].Tool)
}

func TestRenderRequestJSONFiltersByRequestID(t *testing.T) {
	entries := []logview.AuditEntry{
		{RequestID: "req-1", Tool: "read_file", Status: "ok"},
		{RequestID: "req-2", Tool: "write_file", Status: "error"},
	}
	proxyEntries := []logview.ProxyEntry{
		{RequestID: "req-1", Tool: "read_file", Status: "ok"},
		{RequestID: "req-9", Tool: "search", Status: "ok"},
	}

	output, err := renderRequestJSON(entries, proxyEntries, "req-1")
	require.NoError(t, err)

	var payload struct {
		RequestID string               `json:"request_id"`
		Proxy     []logview.ProxyEntry `json:"proxy"`
		Server    []logview.AuditEntry `json:"server"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &payload))
	assert.Equal(t, "req-1", payload.RequestID)
	require.Len(t, payload.Server, 1)
	assert.Equal(t, "read_file", payload.Server[0].Tool)
	require.Len(t, payload.Proxy, 1)
	assert.Equal(t, "read_file", payload.Proxy[0].Tool)
}
