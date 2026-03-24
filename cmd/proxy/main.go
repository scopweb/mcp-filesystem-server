package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

type jsonRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type callToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Meta      *mcp.Meta              `json:"_meta,omitempty"`
}

type ProxyLogEntry struct {
	Timestamp  time.Time `json:"ts"`
	Model      string    `json:"model,omitempty"`
	Tool       string    `json:"tool"`
	Path       string    `json:"path,omitempty"`
	BytesIn    int64     `json:"bytes_in"`
	BytesOut   int64     `json:"bytes_out"`
	TokensIn   int64     `json:"tokens_in"`
	TokensOut  int64     `json:"tokens_out"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
}

type pendingCall struct {
	entry ProxyLogEntry
	start time.Time
}

type proxyLogger struct {
	mu      sync.Mutex
	file    *os.File
	logDir  string
	logPath string
	written int64
}

var generatedID atomic.Uint64

func main() {
	model := flag.String("model", "", "Model name to tag in logs")
	logDir := flag.String("log-dir", "", "Directory for proxy logs (required)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mcp-proxy [--model NAME] [--log-dir DIR] -- <command> [args...]")
		os.Exit(1)
	}
	if *logDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --log-dir is required")
		os.Exit(1)
	}
	if args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no target command specified")
		os.Exit(1)
	}

	logger, err := newProxyLogger(*logDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize proxy logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	log.SetOutput(os.Stderr)
	log.Printf("mcp-proxy: model=%q log-dir=%q target=%v", *model, *logDir, args)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr

	childStdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get child stdin: %v", err)
	}
	childStdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get child stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start child: %v", err)
	}

	var mu sync.Mutex
	pending := map[string]*pendingCall{}

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			forwardLine := append([]byte(nil), line...)

			var msg jsonRPCMessage
			if err := json.Unmarshal(line, &msg); err == nil && msg.Method == "tools/call" {
				var params callToolParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					reqID := extractID(msg.ID)
					if reqID == "" {
						reqID = nextGeneratedID()
					}
					params.Meta = ensureMeta(params.Meta)
					params.Meta.AdditionalFields["request_id"] = reqID
					updatedParams, marshalErr := json.Marshal(params)
					if marshalErr == nil {
						msg.Params = updatedParams
						updatedMessage, marshalErr := json.Marshal(msg)
						if marshalErr == nil {
							forwardLine = updatedMessage
						}
					}

					argsBytes, _ := json.Marshal(params.Arguments)
					entry := ProxyLogEntry{
						Timestamp: time.Now().UTC(),
						Model:     *model,
						Tool:      params.Name,
						BytesIn:   int64(len(argsBytes)),
						TokensIn:  int64(len(argsBytes)) / 4,
						RequestID: reqID,
					}
					if p, ok := params.Arguments["path"].(string); ok {
						entry.Path = p
					}

					mu.Lock()
					pending[reqID] = &pendingCall{entry: entry, start: time.Now()}
					mu.Unlock()
				}
			}

			_, _ = childStdin.Write(forwardLine)
			_, _ = childStdin.Write([]byte("\n"))
		}
		_ = childStdin.Close()
	}()

	scanner := bufio.NewScanner(childStdout)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		_, _ = os.Stdout.Write(line)
		_, _ = os.Stdout.Write([]byte("\n"))

		var msg jsonRPCMessage
		if err := json.Unmarshal(line, &msg); err == nil && msg.ID != nil && msg.Method == "" {
			reqID := extractID(msg.ID)

			mu.Lock()
			pc, ok := pending[reqID]
			if ok {
				delete(pending, reqID)
			}
			mu.Unlock()

			if ok {
				pc.entry.DurationMs = time.Since(pc.start).Milliseconds()
				pc.entry.BytesOut = int64(len(line))
				pc.entry.TokensOut = int64(len(line)) / 4

				if msg.Error != nil && len(msg.Error) > 0 && string(msg.Error) != "null" {
					pc.entry.Status = "error"
					var errObj struct {
						Message string `json:"message"`
					}
					if json.Unmarshal(msg.Error, &errObj) == nil {
						pc.entry.Error = errObj.Message
					}
				} else {
					pc.entry.Status = "ok"
					var result struct {
						IsError bool `json:"isError"`
					}
					if json.Unmarshal(msg.Result, &result) == nil && result.IsError {
						pc.entry.Status = "error"
					}
				}

				logger.Log(pc.entry)
			}
		}
	}

	_ = cmd.Wait()
}

func ensureMeta(meta *mcp.Meta) *mcp.Meta {
	if meta == nil {
		return &mcp.Meta{AdditionalFields: map[string]interface{}{}}
	}
	if meta.AdditionalFields == nil {
		meta.AdditionalFields = map[string]interface{}{}
	}
	return meta
}

func nextGeneratedID() string {
	value := generatedID.Add(1)
	return fmt.Sprintf("proxy-%d-%d", time.Now().UTC().UnixNano(), value)
}

func extractID(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	return strings.Trim(string(raw), `"`)
}

func newProxyLogger(logDir string) (*proxyLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(logDir, "proxy.jsonl")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	info, _ := file.Stat()
	written := int64(0)
	if info != nil {
		written = info.Size()
	}
	return &proxyLogger{file: file, logDir: logDir, logPath: logPath, written: written}, nil
}

func (l *proxyLogger) Log(entry ProxyLogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	n, _ := l.file.Write(data)
	l.written += int64(n)
	if l.written >= 10*1024*1024 {
		l.rotate()
	}
}

func (l *proxyLogger) rotate() {
	_ = l.file.Close()
	ts := time.Now().Format("20060102-150405")
	rotated := filepath.Join(l.logDir, fmt.Sprintf("proxy-%s.jsonl", ts))
	_ = os.Rename(l.logPath, rotated)

	paths, _ := filepath.Glob(filepath.Join(l.logDir, "proxy-*.jsonl"))
	if len(paths) > 3 {
		for i := 0; i < len(paths)-3; i++ {
			_ = os.Remove(paths[i])
		}
	}

	file, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		l.file = nil
		l.written = 0
		return
	}
	l.file = file
	l.written = 0
}

func (l *proxyLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}
