package filesystemserver

import (
	"fmt"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestFileEdits(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp(".", "testdir-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after tests

	tests := []struct {
		name     string
		content  string
		oldText  string
		newText  string
		expected string
	}{
		{
			name:     "exact match",
			content:  "This is line 1\nThis is line 2\n    This has indentation\nSome text with     multiple   spaces\nLast line",
			oldText:  "line 2",
			newText:  "LINE TWO",
			expected: "This is line 1\nThis is LINE TWO\n    This has indentation\nSome text with     multiple   spaces\nLast line",
		},
		{
			name:     "with spaces",
			content:  "This is line 1\nThis is line 2\n    This has indentation\nSome text with     multiple   spaces\nLast line",
			oldText:  "multiple   spaces",
			newText:  "single space",
			expected: "This is line 1\nThis is line 2\n    This has indentation\nSome text with     single space\nLast line",
		},
		{
			name:     "full line with indent",
			content:  "This is line 1\nThis is line 2\n    This has indentation\nSome text with     multiple   spaces\nLast line",
			oldText:  "This has indentation",
			newText:  "This is now modified",
			expected: "This is line 1\nThis is line 2\n    This is now modified\nSome text with     multiple   spaces\nLast line",
		},
		{
			name:     "multiline",
			content:  "First line\nSecond line\nThird line",
			oldText:  "Second line\nThird",
			newText:  "Modified second\nModified third",
			expected: "First line\nModified second\nModified third line",
		},
	}

	// Create handler with both current directory and temp directory allowed
	allowedDirs := []string{".", tempDir}
	handler, err := NewFilesystemHandler(allowedDirs)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file in the allowed temp directory
			tmpFile, err := os.CreateTemp(tempDir, "testfile-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			tmpPath := tmpFile.Name()
			tmpFile.Close() // Close the file so we can write to it

			// Write test content
			if err := os.WriteFile(tmpPath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Test the file edit functionality
			testEditFile(t, handler, tmpPath, tc.oldText, tc.newText, tc.expected)
		})
	}
}

// testEditFile is a helper function to test file editing functionality
func testEditFile(t *testing.T, handler *FilesystemHandler, filePath, oldText, newText, expected string) {
	// Create the request structure that matches the actual implementation
	req := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name: "edit_file",
			Arguments: map[string]interface{}{
				"path":     filePath,
				"old_text": oldText,
				"new_text": newText,
			},
		},
	}

	// Execute the file edit operation
	_, err := handler.handleEditFile(nil, req)

	if err != nil {
		t.Fatalf("Edit failed: %v", err)
	}

	// Read the result
	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read result file: %v", err)
	}

	// Verify the result
	assert.Equal(t, expected, string(result), "File content does not match expected")
}

func TestMain(m *testing.M) {
	// Pre-test setup
	fmt.Println("Setting up tests...")
	
	// Run tests
	exitCode := m.Run()
	
	// Post-test cleanup
	fmt.Println("Tests completed")
	
	// Exit with the test result code
	os.Exit(exitCode)
}
