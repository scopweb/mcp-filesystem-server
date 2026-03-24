package logview

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type AuditEntry struct {
	Timestamp      time.Time              `json:"ts"`
	OperationID    string                 `json:"op_id"`
	RequestID      string                 `json:"request_id,omitempty"`
	Kind           string                 `json:"kind"`
	Tool           string                 `json:"tool,omitempty"`
	Path           string                 `json:"path,omitempty"`
	DurationMs     int64                  `json:"duration_ms"`
	Status         string                 `json:"status"`
	Error          string                 `json:"error,omitempty"`
	ArgsRaw        map[string]interface{} `json:"args_raw,omitempty"`
	ArgsNormalized map[string]interface{} `json:"args_normalized,omitempty"`
	InternalAction string                 `json:"internal_action,omitempty"`
	SubOperations  []string               `json:"sub_operations,omitempty"`
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

type ProxyEntry struct {
	Timestamp  time.Time `json:"ts"`
	Model      string    `json:"model,omitempty"`
	Tool       string    `json:"tool"`
	Path       string    `json:"path,omitempty"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
}

func ReadAuditEntries(path string) ([]AuditEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []AuditEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	return entries, nil
}

func ReadMetrics(path string) (*MetricsSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var metrics MetricsSnapshot
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, err
	}
	return &metrics, nil
}

func ReadProxyEntries(proxyLogDir string) ([]ProxyEntry, error) {
	if strings.TrimSpace(proxyLogDir) == "" {
		return nil, errors.New("proxy log dir not provided")
	}
	path := filepath.Join(proxyLogDir, "proxy.jsonl")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []ProxyEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ProxyEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	return entries, nil
}

func FilterAuditEntries(entries []AuditEntry, tool string) []AuditEntry {
	if strings.TrimSpace(tool) == "" {
		return entries
	}
	filtered := make([]AuditEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.EqualFold(DisplayTool(entry), tool) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func FilterProxyEntries(entries []ProxyEntry, tool string) []ProxyEntry {
	if strings.TrimSpace(tool) == "" {
		return entries
	}
	filtered := make([]ProxyEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.EqualFold(entry.Tool, tool) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func DisplayTool(entry AuditEntry) string {
	if entry.Tool != "" {
		return entry.Tool
	}
	return entry.Kind
}

func BuildMetricsFromEntries(entries []AuditEntry) *MetricsSnapshot {
	metrics := &MetricsSnapshot{
		UpdatedAt: time.Now().UTC(),
		ByTool:    make(map[string]ToolMetrics),
		ByKind:    make(map[string]int64),
	}
	var durationTotal int64
	for _, entry := range entries {
		metrics.OpsTotal++
		durationTotal += entry.DurationMs
		metrics.ByKind[entry.Kind]++

		if entry.Status == "error" {
			metrics.OpsError++
		} else {
			metrics.OpsOK++
		}

		toolKey := DisplayTool(entry)
		toolMetrics := metrics.ByTool[toolKey]
		toolMetrics.Count++
		if entry.Status == "error" {
			toolMetrics.Errors++
		}
		toolMetrics.AvgDurationMs = runningAverage(toolMetrics.AvgDurationMs, toolMetrics.Count, entry.DurationMs)
		metrics.ByTool[toolKey] = toolMetrics
	}
	if metrics.OpsTotal > 0 {
		metrics.AvgDurationMs = float64(durationTotal) / float64(metrics.OpsTotal)
	}
	return metrics
}

func runningAverage(current float64, count int64, last int64) float64 {
	if count <= 1 {
		return float64(last)
	}
	prevCount := float64(count - 1)
	return ((current * prevCount) + float64(last)) / float64(count)
}
