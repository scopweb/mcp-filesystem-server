package filesystemserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// rateLimiter implements a simple sliding-window rate limiter.
type rateLimiter struct {
	mu       sync.Mutex
	calls    []time.Time
	maxCalls int
	window   time.Duration
}

func newRateLimiter(maxCalls int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		calls:    make([]time.Time, 0, maxCalls),
		maxCalls: maxCalls,
		window:   window,
	}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remove expired entries
	valid := rl.calls[:0]
	for _, t := range rl.calls {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.calls = valid

	if len(rl.calls) >= rl.maxCalls {
		return false
	}

	rl.calls = append(rl.calls, now)
	return true
}

// withRateLimit wraps a tool handler with rate limiting.
// Allows 60 calls per minute per server instance (shared across all tools).
func (fs *FilesystemHandler) withRateLimit(handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if !fs.limiter.allow() {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Rate limit exceeded: max %d calls per %s. Please wait before retrying.",
							fs.limiter.maxCalls, fs.limiter.window),
					},
				},
				IsError: true,
			}, nil
		}
		return handler(ctx, request)
	}
}
