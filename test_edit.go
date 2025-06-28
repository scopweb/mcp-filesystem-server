package main

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/scopweb/mcp-filesystem-server/filesystemserver"
)

func main() {
	// Crear un archivo de prueba
	testFile := "test_file.txt"
	testContent := `This is line 1
This is line 2
    This has indentation
Some text with     multiple   spaces
Last line`

	// Escribir archivo de prueba
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		panic(err)
	}
	defer os.Remove(testFile)

	// Crear handler
	allowedDirs := []string{"."}
	handler, _ := filesystemserver.NewFilesystemHandler(allowedDirs)

	// Prueba 1: Reemplazo exacto
	fmt.Println("=== Test 1: Exact match ===")
	testEdit(handler, testFile, "line 2", "LINE TWO")

	// Prueba 2: Reemplazo con espacios
	fmt.Println("\n=== Test 2: With spaces ===")
	testEdit(handler, testFile, "multiple   spaces", "single space")

	// Prueba 3: Línea completa con indentación
	fmt.Println("\n=== Test 3: Full line with indent ===")
	testEdit(handler, testFile, "This has indentation", "This is now modified")

	// Prueba 4: Multi-línea
	multilineContent := `First line
Second line
Third line`
	os.WriteFile(testFile, []byte(multilineContent), 0644)
	
	fmt.Println("\n=== Test 4: Multiline ===")
	testEdit(handler, testFile, "Second line\nThird", "Modified second\nModified third")
}

func testEdit(handler *filesystemserver.FilesystemHandler, file, oldText, newText string) {
	// Leer contenido actual
	before, _ := os.ReadFile(file)
	fmt.Printf("Before:\n%s\n\n", before)

	// Simular llamada a edit_file
	request := mcp.CallToolRequest{
		Method: "edit_file",
		Params: mcp.CallToolRequestParams{
			Name: "edit_file",
			Arguments: map[string]interface{}{
				"path":     file,
				"old_text": oldText,
				"new_text": newText,
			},
		},
	}

	result, err := handler.HandleEditFile(nil, request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Leer contenido después
	after, _ := os.ReadFile(file)
	fmt.Printf("After:\n%s\n", after)
	fmt.Printf("Result: %v\n\n", result.Content[0])
}
