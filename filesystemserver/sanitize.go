package filesystemserver

import "unicode/utf8"

const (
	// maxOutputSize is the maximum size of tool output text (1MB).
	maxOutputSize = 1 * 1024 * 1024
)

// sanitizeOutput truncates output that exceeds the maximum size and ensures
// it is valid UTF-8. This prevents excessively large outputs from being
// passed to the LLM and mitigates prompt injection via file content.
func sanitizeOutput(text string) string {
	if len(text) > maxOutputSize {
		// Truncate at a valid UTF-8 boundary
		truncated := text[:maxOutputSize]
		for !utf8.ValidString(truncated) && len(truncated) > 0 {
			truncated = truncated[:len(truncated)-1]
		}
		text = truncated + "\n\n[Output truncated: exceeded 1MB limit]"
	}

	if !utf8.ValidString(text) {
		// Replace invalid UTF-8 sequences
		text = string([]rune(text))
	}

	return text
}
