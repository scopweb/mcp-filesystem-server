package filesystemserver_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/scopweb/mcp-filesystem-server/filesystemserver"
	"github.com/stretchr/testify/assert"
)

func TestFileEdits(t *testing.T) {
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
			expected: "This is line 1\nThis is line 2\n    This has indentation\nSome text with single space\nLast line",
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

	// Crear handler
	allowedDirs := []string{"."}
	handler, err := filesystemserver.NewFilesystemHandler(allowedDirs)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Crear archivo temporal
			tmpFile, err := os.CreateTemp("", "testfile-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Escribir contenido de prueba
			if err := os.WriteFile(tmpFile.Name(), []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Crear solicitud
			req := mcp.CallToolRequest{
				Tool: "edit_file",
				Arguments: map[string]interface{}{
					"path":     tmpFile.Name(),
					"old_text": tc.oldText,
					"new_text": tc.newText,
				},
			}

			// Ejecutar la edición
			_, err = handler.HandleEditFile(nil, req)
			if err != nil {
				t.Fatalf("Edit failed: %v", err)
			}

			// Leer el resultado
			result, err := os.ReadFile(tmpFile.Name())
			if err != nil {
				t.Fatalf("Failed to read result file: %v", err)
			}

			// Verificar el resultado
			assert.Equal(t, tc.expected, string(result), "File content does not match expected")
		})
	}
}

func TestMain(m *testing.M) {
	// Código de configuración previa a las pruebas
	fmt.Println("Setting up tests...")
	
	// Ejecutar las pruebas
	exitCode := m.Run()
	
	// Código de limpieza posterior a las pruebas
	fmt.Println("Tests completed")
	
	// Salir con el código de salida de las pruebas
	os.Exit(exitCode)
}
