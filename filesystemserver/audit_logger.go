package filesystemserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	auditLogBufferSize      = 256
	auditLogRotateSize      = 10 * 1024 * 1024
	auditLogRetentionPeriod = 10 * 24 * time.Hour
	metricsFlushInterval    = 15 * time.Second
	maxLoggedStringLength   = 256
)

type AuditEntry struct {
	Timestamp      time.Time              `json:"ts"`
	OperationID    string                 `json:"op_id"`
	RequestID      string                 `json:"request_id,omitempty"`
	Kind           string                 `json:"kind"`
	Tool           string                 `json:"tool,omitempty"`
	ResourceURI    string                 `json:"resource_uri,omitempty"`
	Path           string                 `json:"path,omitempty"`
	DurationMs     int64                  `json:"duration_ms"`
	Status         string                 `json:"status"`
	Error          string                 `json:"error,omitempty"`
	ArgsRaw        map[string]interface{} `json:"args_raw,omitempty"`
	ArgsNormalized map[string]interface{} `json:"args_normalized,omitempty"`
	InternalAction string                 `json:"internal_action,omitempty"`
	SubOperations  []string               `json:"sub_operations,omitempty"`
	ResultIsError  bool                   `json:"result_is_error,omitempty"`
}

type auditContextKey struct{}

type auditContextState struct {
	requestID     string
	mu            sync.Mutex
	subOperations []string
}

type MetricsSnapshot struct {
	UpdatedAt     time.Time              `json:"updated_at"`
	OpsTotal      int64                  `json:"ops_total"`
	OpsOK         int64                  `json:"ops_ok"`
	OpsError      int64                  `json:"ops_error"`
	DroppedLogs   int64                  `json:"dropped_logs"`
	AvgDurationMs float64                `json:"avg_duration_ms"`
	ByTool        map[string]ToolMetrics `json:"by_tool,omitempty"`
	ByKind        map[string]int64       `json:"by_kind,omitempty"`
}

type ToolMetrics struct {
	Count         int64   `json:"count"`
	Errors        int64   `json:"errors"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

type auditAggregate struct {
	opsTotal      int64
	opsOK         int64
	opsError      int64
	durationTotal int64
	byTool        map[string]*ToolMetrics
	byKind        map[string]int64
}

type AuditLogger struct {
	logDir  string
	logPath string

	entries chan AuditEntry
	stopCh  chan struct{}
	doneCh  chan struct{}

	closeOnce sync.Once
	dropped   atomic.Int64
	sequence  atomic.Uint64

	file    *os.File
	written int64
	agg     auditAggregate
}

func NewAuditLogger(logDir string) (*AuditLogger, error) {
	if strings.TrimSpace(logDir) == "" {
		return nil, fmt.Errorf("log directory is required")
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, "operations.jsonl")
	file, written, err := openAuditFile(logPath)
	if err != nil {
		return nil, err
	}

	logger := &AuditLogger{
		logDir:  logDir,
		logPath: logPath,
		entries: make(chan AuditEntry, auditLogBufferSize),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		file:    file,
		written: written,
		agg: auditAggregate{
			byTool: make(map[string]*ToolMetrics),
			byKind: make(map[string]int64),
		},
	}

	go logger.run()
	return logger, nil
}

func openAuditFile(logPath string) (*os.File, int64, error) {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, 0, fmt.Errorf("open log file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("stat log file: %w", err)
	}
	return file, info.Size(), nil
}

func (a *AuditLogger) Log(entry AuditEntry) {
	if a == nil {
		return
	}

	select {
	case a.entries <- entry:
	default:
		a.dropped.Add(1)
	}
}

func (a *AuditLogger) Close() error {
	if a == nil {
		return nil
	}

	a.closeOnce.Do(func() {
		close(a.stopCh)
		<-a.doneCh
	})

	return nil
}

func (a *AuditLogger) run() {
	ticker := time.NewTicker(metricsFlushInterval)
	defer ticker.Stop()
	defer close(a.doneCh)

	for {
		select {
		case entry := <-a.entries:
			a.writeEntry(entry)
		case <-ticker.C:
			a.writeMetricsSnapshot()
		case <-a.stopCh:
			a.drainEntries()
			a.writeMetricsSnapshot()
			if a.file != nil {
				_ = a.file.Close()
			}
			return
		}
	}
}

func (a *AuditLogger) drainEntries() {
	for {
		select {
		case entry := <-a.entries:
			a.writeEntry(entry)
		default:
			return
		}
	}
}

func (a *AuditLogger) writeEntry(entry AuditEntry) {
	if a.file == nil {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	n, err := a.file.Write(data)
	if err != nil {
		return
	}
	a.written += int64(n)
	a.updateMetrics(entry)

	if a.written >= auditLogRotateSize {
		a.rotate()
	}
}

func (a *AuditLogger) updateMetrics(entry AuditEntry) {
	a.agg.opsTotal++
	a.agg.durationTotal += entry.DurationMs
	a.agg.byKind[entry.Kind]++

	if entry.Status == "error" {
		a.agg.opsError++
	} else {
		a.agg.opsOK++
	}

	toolKey := entry.Tool
	if toolKey == "" {
		toolKey = entry.Kind
	}
	metric := a.agg.byTool[toolKey]
	if metric == nil {
		metric = &ToolMetrics{}
		a.agg.byTool[toolKey] = metric
	}
	metric.Count++
	if entry.Status == "error" {
		metric.Errors++
	}
	metric.AvgDurationMs = runningAverage(metric.AvgDurationMs, metric.Count, entry.DurationMs)
}

func runningAverage(current float64, count int64, last int64) float64 {
	if count <= 1 {
		return float64(last)
	}
	prevCount := float64(count - 1)
	return ((current * prevCount) + float64(last)) / float64(count)
}

func (a *AuditLogger) rotate() {
	if a.file != nil {
		_ = a.file.Close()
	}

	ts := time.Now().Format("20060102-150405")
	rotatedPath := filepath.Join(a.logDir, fmt.Sprintf("operations-%s.jsonl", ts))
	_ = os.Rename(a.logPath, rotatedPath)
	a.cleanOldLogs()

	file, written, err := openAuditFile(a.logPath)
	if err != nil {
		a.file = nil
		a.written = 0
		return
	}
	a.file = file
	a.written = written
}

func (a *AuditLogger) cleanOldLogs() {
	paths, err := filepath.Glob(filepath.Join(a.logDir, "operations-*.jsonl"))
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-auditLogRetentionPeriod)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
}

func (a *AuditLogger) writeMetricsSnapshot() {
	snapshot := MetricsSnapshot{
		UpdatedAt:   time.Now().UTC(),
		OpsTotal:    a.agg.opsTotal,
		OpsOK:       a.agg.opsOK,
		OpsError:    a.agg.opsError,
		DroppedLogs: a.dropped.Load(),
		ByTool:      make(map[string]ToolMetrics, len(a.agg.byTool)),
		ByKind:      make(map[string]int64, len(a.agg.byKind)),
	}

	if a.agg.opsTotal > 0 {
		snapshot.AvgDurationMs = float64(a.agg.durationTotal) / float64(a.agg.opsTotal)
	}
	for key, metric := range a.agg.byTool {
		snapshot.ByTool[key] = *metric
	}
	for key, value := range a.agg.byKind {
		snapshot.ByKind[key] = value
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(a.logDir, "metrics.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return
	}
	_ = os.Remove(path)
	_ = os.Rename(tmpPath, path)
}

func (a *AuditLogger) nextOperationID() string {
	seq := a.sequence.Add(1)
	return fmt.Sprintf("op-%d-%d", time.Now().UTC().UnixNano(), seq)
}

func (fs *FilesystemHandler) SetAuditLogger(logger *AuditLogger) {
	fs.mu.Lock()
	fs.auditLogger = logger
	fs.mu.Unlock()
}

func (fs *FilesystemHandler) getAuditLogger() *AuditLogger {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.auditLogger
}

func (fs *FilesystemHandler) withAudit(tool string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ctx = withAuditContext(ctx, extractRequestIDFromToolRequest(request))
		start := time.Now().UTC()
		result, err := handler(ctx, request)
		args := summarizeArguments(request.GetArguments())
		fs.logToolCall(ctx, start, tool, args, args, "direct", result, err)
		return result, err
	}
}

func (fs *FilesystemHandler) withResourceAudit(handler func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		start := time.Now().UTC()
		result, err := handler(ctx, request)
		fs.logResourceRead(ctx, start, request.Params.URI, err)
		return result, err
	}
}

func (fs *FilesystemHandler) logToolCall(ctx context.Context, start time.Time, tool string, rawArgs map[string]interface{}, normalizedArgs map[string]interface{}, internalAction string, result *mcp.CallToolResult, err error) {
	logger := fs.getAuditLogger()
	if logger == nil {
		return
	}
	requestID, subOperations := getAuditContextSnapshot(ctx)

	entry := AuditEntry{
		Timestamp:      start,
		OperationID:    logger.nextOperationID(),
		RequestID:      requestID,
		Kind:           "tool",
		Tool:           tool,
		Path:           extractPrimaryPath(normalizedArgs),
		DurationMs:     time.Since(start).Milliseconds(),
		Status:         "ok",
		ArgsRaw:        rawArgs,
		ArgsNormalized: normalizedArgs,
		InternalAction: internalAction,
		SubOperations:  subOperations,
	}

	if err != nil {
		entry.Status = "error"
		entry.Error = err.Error()
	} else if result != nil && result.IsError {
		entry.Status = "error"
		entry.ResultIsError = true
		entry.Error = extractResultError(result)
	}

	logger.Log(entry)
}

func (fs *FilesystemHandler) logResourceRead(ctx context.Context, start time.Time, uri string, err error) {
	logger := fs.getAuditLogger()
	if logger == nil {
		return
	}
	requestID, subOperations := getAuditContextSnapshot(ctx)

	entry := AuditEntry{
		Timestamp:      start,
		OperationID:    logger.nextOperationID(),
		RequestID:      requestID,
		Kind:           "resource_read",
		ResourceURI:    uri,
		Path:           strings.TrimPrefix(uri, "file://"),
		DurationMs:     time.Since(start).Milliseconds(),
		Status:         "ok",
		InternalAction: "resource_read",
		SubOperations:  subOperations,
	}
	if err != nil {
		entry.Status = "error"
		entry.Error = err.Error()
	}

	logger.Log(entry)
}

func withAuditContext(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, auditContextKey{}, &auditContextState{requestID: requestID})
}

func appendSubOperation(ctx context.Context, subOperation string) {
	if ctx == nil || subOperation == "" {
		return
	}
	state, ok := ctx.Value(auditContextKey{}).(*auditContextState)
	if !ok || state == nil {
		return
	}
	state.mu.Lock()
	state.subOperations = append(state.subOperations, subOperation)
	state.mu.Unlock()
}

func getAuditContextSnapshot(ctx context.Context) (string, []string) {
	if ctx == nil {
		return "", nil
	}
	state, ok := ctx.Value(auditContextKey{}).(*auditContextState)
	if !ok || state == nil {
		return "", nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	result := make([]string, len(state.subOperations))
	copy(result, state.subOperations)
	return state.requestID, result
}

func extractRequestIDFromToolRequest(request mcp.CallToolRequest) string {
	meta := request.Params.Meta
	if meta == nil || len(meta.AdditionalFields) == 0 {
		return ""
	}
	for _, key := range []string{"request_id", "proxy_request_id", "jsonrpc_id"} {
		if value, ok := meta.AdditionalFields[key]; ok {
			if requestID, ok := convertToString(value); ok {
				return requestID
			}
		}
	}
	return ""
}

func summarizeArguments(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return nil
	}
	result := make(map[string]interface{}, len(args))
	for key, value := range args {
		result[key] = sanitizeLoggedValue(key, value)
	}
	return result
}

func sanitizeLoggedValue(key string, value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		if shouldSummarizeLargeString(key) {
			return map[string]interface{}{
				"length":    len(typed),
				"preview":   truncateString(typed),
				"truncated": len(typed) > maxLoggedStringLength,
			}
		}
		return truncateString(typed)
	case map[string]interface{}:
		return summarizeArguments(typed)
	case []interface{}:
		items := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeLoggedValue(key, item))
		}
		return items
	default:
		return typed
	}
}

func shouldSummarizeLargeString(key string) bool {
	switch key {
	case "content", "content_base64", "old_text", "new_text", "blob", "data", "text":
		return true
	default:
		return false
	}
}

func truncateString(value string) string {
	if len(value) <= maxLoggedStringLength {
		return value
	}
	return value[:maxLoggedStringLength] + "..."
}

func extractPrimaryPath(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	for _, key := range []string{"path", "source", "destination", "file1", "file2"} {
		if value, ok := args[key]; ok {
			switch typed := value.(type) {
			case string:
				return typed
			case map[string]interface{}:
				if preview, ok := typed["preview"].(string); ok {
					return preview
				}
			}
		}
	}
	return ""
}

func extractResultError(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	for _, content := range result.Content {
		switch typed := content.(type) {
		case mcp.TextContent:
			return truncateString(typed.Text)
		case *mcp.TextContent:
			if typed != nil {
				return truncateString(typed.Text)
			}
		}
	}
	return "tool returned isError=true"
}

func buildInternalAction(rootsFetched bool, rawArgs map[string]interface{}, normalizedArgs map[string]interface{}) string {
	actions := make([]string, 0, 2)
	if rootsFetched {
		actions = append(actions, "roots_sync")
	}
	if !reflect.DeepEqual(rawArgs, normalizedArgs) {
		actions = append(actions, "normalized")
	}
	if len(actions) == 0 {
		return "direct"
	}
	return strings.Join(actions, " -> ")
}
