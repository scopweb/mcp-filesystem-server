package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/scopweb/mcp-filesystem-server/internal/logview"
)

func main() {
	logDir := flag.String("log-dir", "", "Directory containing operations.jsonl and metrics.json")
	proxyLogDir := flag.String("proxy-log-dir", "", "Directory containing proxy.jsonl (optional)")
	mode := flag.String("mode", "summary", "Mode: summary, recent, errors, request")
	limit := flag.Int("limit", 10, "Maximum number of operations to show")
	requestID := flag.String("request-id", "", "Request ID to inspect when mode=request")
	tool := flag.String("tool", "", "Filter entries by tool name")
	jsonOutput := flag.Bool("json", false, "Emit JSON instead of text")
	flag.Parse()

	if *logDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --log-dir is required")
		os.Exit(1)
	}

	entries, err := logview.ReadAuditEntries(filepath.Join(*logDir, "operations.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading operations log: %v\n", err)
		os.Exit(1)
	}
	entries = logview.FilterAuditEntries(entries, *tool)
	metrics, _ := logview.ReadMetrics(filepath.Join(*logDir, "metrics.json"))
	proxyEntries, _ := logview.ReadProxyEntries(*proxyLogDir)
	proxyEntries = logview.FilterProxyEntries(proxyEntries, *tool)

	if strings.TrimSpace(*tool) != "" {
		metrics = logview.BuildMetricsFromEntries(entries)
	}

	switch strings.ToLower(*mode) {
	case "summary":
		printSummary(metrics, entries, *limit, *jsonOutput)
	case "recent":
		printRecent(entries, *limit, *jsonOutput)
	case "errors":
		printErrors(entries, *limit, *jsonOutput)
	case "request":
		if strings.TrimSpace(*requestID) == "" {
			fmt.Fprintln(os.Stderr, "Error: --request-id is required for mode=request")
			os.Exit(1)
		}
		printRequest(entries, proxyEntries, *requestID, *jsonOutput)
	default:
		fmt.Fprintf(os.Stderr, "Error: unsupported mode %q\n", *mode)
		os.Exit(1)
	}
}

func printSummary(metrics *logview.MetricsSnapshot, entries []logview.AuditEntry, limit int, jsonOutput bool) {
	if jsonOutput {
		payload := map[string]interface{}{
			"metrics": metrics,
			"recent":  limitedEntries(entries, limit),
		}
		emitJSON(payload)
		return
	}
	if metrics != nil {
		fmt.Printf("Summary updated: %s\n", metrics.UpdatedAt.Format(time.RFC3339))
		fmt.Printf("Operations: total=%d ok=%d error=%d dropped_logs=%d avg=%.2fms\n", metrics.OpsTotal, metrics.OpsOK, metrics.OpsError, metrics.DroppedLogs, metrics.AvgDurationMs)
		fmt.Println()
		printTopTools(metrics.ByTool, limit)
	} else {
		fmt.Printf("Operations loaded: %d\n", len(entries))
	}
	fmt.Println()
	printRecent(entries, limit, false)
}

func printTopTools(byTool map[string]logview.ToolMetrics, limit int) {
	type item struct {
		Tool string
		logview.ToolMetrics
	}
	items := make([]item, 0, len(byTool))
	for tool, metric := range byTool {
		items = append(items, item{Tool: tool, ToolMetrics: metric})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})
	if limit > len(items) {
		limit = len(items)
	}
	fmt.Println("Top tools:")
	for i := 0; i < limit; i++ {
		it := items[i]
		fmt.Printf("  %s count=%d errors=%d avg=%.2fms\n", it.Tool, it.Count, it.Errors, it.AvgDurationMs)
	}
}

func printRecent(entries []logview.AuditEntry, limit int, jsonOutput bool) {
	entries = limitedEntries(entries, limit)
	if jsonOutput {
		emitJSON(entries)
		return
	}
	if limit > len(entries) {
		limit = len(entries)
	}
	fmt.Printf("Recent operations (%d):\n", limit)
	for i := 0; i < limit; i++ {
		entry := entries[i]
		fmt.Printf("  %s %s status=%s duration=%dms request_id=%s path=%s\n", entry.Timestamp.Format(time.RFC3339), logview.DisplayTool(entry), entry.Status, entry.DurationMs, entry.RequestID, entry.Path)
	}
}

func printErrors(entries []logview.AuditEntry, limit int, jsonOutput bool) {
	errorsOnly := make([]logview.AuditEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Status == "error" {
			errorsOnly = append(errorsOnly, entry)
		}
	}
	errorsOnly = limitedEntries(errorsOnly, limit)
	if jsonOutput {
		emitJSON(errorsOnly)
		return
	}
	count := 0
	fmt.Println("Recent errors:")
	for _, entry := range errorsOnly {
		fmt.Printf("  %s %s request_id=%s error=%s\n", entry.Timestamp.Format(time.RFC3339), logview.DisplayTool(entry), entry.RequestID, entry.Error)
		count++
		if count >= limit {
			return
		}
	}
	if count == 0 {
		fmt.Println("  no errors found")
	}
}

func printRequest(entries []logview.AuditEntry, proxyEntries []logview.ProxyEntry, requestID string, jsonOutput bool) {
	requestEntries := make([]logview.AuditEntry, 0)
	for _, entry := range entries {
		if entry.RequestID == requestID {
			requestEntries = append(requestEntries, entry)
		}
	}
	requestProxyEntries := make([]logview.ProxyEntry, 0)
	for _, entry := range proxyEntries {
		if entry.RequestID == requestID {
			requestProxyEntries = append(requestProxyEntries, entry)
		}
	}
	if jsonOutput {
		output, err := renderRequestJSON(entries, proxyEntries, requestID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, output)
		return
	}
	fmt.Printf("Request %s\n", requestID)
	for _, entry := range requestProxyEntries {
		fmt.Printf("Proxy: ts=%s tool=%s status=%s duration=%dms path=%s error=%s\n", entry.Timestamp.Format(time.RFC3339), entry.Tool, entry.Status, entry.DurationMs, entry.Path, entry.Error)
	}
	matched := false
	for _, entry := range requestEntries {
		matched = true
		fmt.Printf("Server: ts=%s tool=%s status=%s duration=%dms action=%s path=%s\n", entry.Timestamp.Format(time.RFC3339), logview.DisplayTool(entry), entry.Status, entry.DurationMs, entry.InternalAction, entry.Path)
		if len(entry.SubOperations) > 0 {
			fmt.Printf("  sub_operations: %s\n", strings.Join(entry.SubOperations, ", "))
		}
		if entry.Error != "" {
			fmt.Printf("  error: %s\n", entry.Error)
		}
	}
	if !matched {
		fmt.Println("No server operations found for that request_id")
	}
}

func limitedEntries(entries []logview.AuditEntry, limit int) []logview.AuditEntry {
	if limit < len(entries) {
		return entries[:limit]
	}
	return entries
}

func emitJSON(value interface{}) {
	if err := encodeJSON(os.Stdout, value); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func encodeJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func renderSummaryJSON(metrics *logview.MetricsSnapshot, entries []logview.AuditEntry, limit int) (string, error) {
	payload := map[string]interface{}{
		"metrics": metrics,
		"recent":  limitedEntries(entries, limit),
	}
	var buf bytes.Buffer
	if err := encodeJSON(&buf, payload); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderRequestJSON(entries []logview.AuditEntry, proxyEntries []logview.ProxyEntry, requestID string) (string, error) {
	requestEntries := make([]logview.AuditEntry, 0)
	for _, entry := range entries {
		if entry.RequestID == requestID {
			requestEntries = append(requestEntries, entry)
		}
	}
	requestProxyEntries := make([]logview.ProxyEntry, 0)
	for _, entry := range proxyEntries {
		if entry.RequestID == requestID {
			requestProxyEntries = append(requestProxyEntries, entry)
		}
	}
	var buf bytes.Buffer
	if err := encodeJSON(&buf, map[string]interface{}{
		"request_id": requestID,
		"proxy":      requestProxyEntries,
		"server":     requestEntries,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
