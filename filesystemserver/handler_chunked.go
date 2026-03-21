package filesystemserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleChunkedWrite - Escribe archivo en fragmentos de 1MB
func (fs *FilesystemHandler) handleChunkedWrite(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := request.GetArguments()["path"].(string)
	content, _ := request.GetArguments()["content"].(string)
	chunkIndex, _ := request.GetArguments()["chunk_index"].(float64)
	totalChunks, _ := request.GetArguments()["total_chunks"].(float64)

	if path == "" || content == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: path and content are required"},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Primer chunk - crear/truncar archivo
	if chunkIndex == 0 {
		parentDir := filepath.Dir(validPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating directory: %v", err)},
				},
				IsError: true,
			}, nil
		}
	}

	// Escribir chunk
	var file *os.File
	if chunkIndex == 0 {
		file, err = os.Create(validPath)
	} else {
		file, err = os.OpenFile(validPath, os.O_WRONLY|os.O_APPEND, 0644)
	}
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error opening file: %v", err)},
			},
			IsError: true,
		}, nil
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error writing chunk: %v", err)},
			},
			IsError: true,
		}, nil
	}

	completed := int(chunkIndex) >= int(totalChunks)-1
	
	info, _ := os.Stat(validPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}

	status := "📝 In progress"
	if completed {
		status = "✅ Completed"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%s chunked write: %s\nChunk: %d/%d\nTotal size: %d bytes",
					status, path, int(chunkIndex)+1, int(totalChunks), size),
			},
		},
	}, nil
}

// handleSplitFile - Divide archivo en múltiples fragmentos
func (fs *FilesystemHandler) handleSplitFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := request.GetArguments()["path"].(string)
	chunkSizeParam, _ := request.GetArguments()["chunk_size"].(float64)

	if path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: path is required"},
			},
			IsError: true,
		}, nil
	}

	chunkSize := int64(MAX_CHUNK_SIZE)
	if chunkSizeParam > 0 {
		chunkSize = int64(chunkSizeParam)
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	if info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: Cannot split directory"},
			},
			IsError: true,
		}, nil
	}

	sourceFile, err := os.Open(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error opening file: %v", err)},
			},
			IsError: true,
		}, nil
	}
	defer sourceFile.Close()

	totalChunks := (info.Size() + chunkSize - 1) / chunkSize
	var chunkFiles []string

	for i := int64(0); i < totalChunks; i++ {
		chunkName := fmt.Sprintf("%s.part%03d", validPath, i)
		chunkFile, err := os.Create(chunkName)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating chunk: %v", err)},
				},
				IsError: true,
			}, nil
		}

		written, err := io.CopyN(chunkFile, sourceFile, chunkSize)
		chunkFile.Close()

		if err != nil && err != io.EOF {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error writing chunk: %v", err)},
				},
				IsError: true,
			}, nil
		}

		if written > 0 {
			chunkFiles = append(chunkFiles, chunkName)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("✅ Split completed: %s\nSource: %d bytes\nChunks: %d files\nChunk size: %d bytes",
					path, info.Size(), len(chunkFiles), chunkSize),
			},
		},
	}, nil
}

// handleJoinFiles - Une múltiples fragmentos en un archivo
func (fs *FilesystemHandler) handleJoinFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	targetPath, _ := request.GetArguments()["target_path"].(string)
	sourceFilesParam, _ := request.GetArguments()["source_files"].([]interface{})

	if targetPath == "" || len(sourceFilesParam) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: target_path and source_files are required"},
			},
			IsError: true,
		}, nil
	}

	validTargetPath, err := fs.validatePath(targetPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error with target: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Convertir source files
	var sourceFiles []string
	for _, sf := range sourceFilesParam {
		if str, ok := sf.(string); ok {
			validPath, err := fs.validatePath(str)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error with source file %s: %v", str, err)},
					},
					IsError: true,
				}, nil
			}
			sourceFiles = append(sourceFiles, validPath)
		}
	}

	// Crear directorio padre si no existe
	parentDir := filepath.Dir(validTargetPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating directory: %v", err)},
			},
			IsError: true,
		}, nil
	}

	targetFile, err := os.Create(validTargetPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating target: %v", err)},
			},
			IsError: true,
		}, nil
	}
	defer targetFile.Close()

	var totalSize int64
	for _, sourcePath := range sourceFiles {
		sourceFile, err := os.Open(sourcePath)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error opening %s: %v", sourcePath, err)},
				},
				IsError: true,
			}, nil
		}

		written, err := io.Copy(targetFile, sourceFile)
		sourceFile.Close()

		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error copying %s: %v", sourcePath, err)},
				},
				IsError: true,
			}, nil
		}

		totalSize += written
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("✅ Join completed: %s\nSources: %d files\nTotal size: %d bytes",
					targetPath, len(sourceFiles), totalSize),
			},
		},
	}, nil
}

// handleWriteFileSafe - Escritura con backup automático
func (fs *FilesystemHandler) handleWriteFileSafe(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := request.GetArguments()["path"].(string)
	content, _ := request.GetArguments()["content"].(string)
	createBackup, _ := request.GetArguments()["create_backup"].(bool)

	if path == "" || content == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: path and content are required"},
			},
			IsError: true,
		}, nil
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	var backupPath string
	
	// Crear backup si el archivo existe y se solicita
	if createBackup {
		if _, err := os.Stat(validPath); err == nil {
			backupPath, err = fs.createBackup(validPath)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating backup: %v", err)},
					},
					IsError: true,
				}, nil
			}
		}
	}

	// Crear directorio padre si no existe
	parentDir := filepath.Dir(validPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating directory: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Escribir archivo temporal primero
	tempPath := validPath + ".tmp"
	err = os.WriteFile(tempPath, []byte(content), 0644)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error writing temp file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Mover archivo temporal al destino final (operación atómica)
	err = os.Rename(tempPath, validPath)
	if err != nil {
		os.Remove(tempPath) // Limpiar archivo temporal
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error finalizing file: %v", err)},
			},
			IsError: true,
		}, nil
	}

	info, _ := os.Stat(validPath)
	size := int64(len(content))
	if info != nil {
		size = info.Size()
	}

	result := fmt.Sprintf("✅ Safe write completed: %s\nSize: %d bytes", path, size)
	if backupPath != "" {
		result += fmt.Sprintf("\nBackup: %s", backupPath)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result},
		},
	}, nil
}
