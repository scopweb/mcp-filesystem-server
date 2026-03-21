package filesystemserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleBatchEdit - Operaciones en lote para múltiples archivos
func (fs *FilesystemHandler) handleBatchEdit(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	operationsParam, ok := request.GetArguments()["operations"].([]interface{})
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "❌ Error: operations must be an array",
				},
			},
			IsError: true,
		}, nil
	}

	if len(operationsParam) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: no operations specified"},
			},
			IsError: true,
		}, nil
	}

	const maxOperations = 50
	if len(operationsParam) > maxOperations {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: too many operations (max: %d)", maxOperations)},
			},
			IsError: true,
		}, nil
	}

	results := []string{}
	errors := []string{}

	for i, op := range operationsParam {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			errors = append(errors, fmt.Sprintf("Operation %d: invalid format", i+1))
			continue
		}

		result, err := fs.processBatchOperation(opMap, i+1)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Operation %d: %v", i+1, err))
		} else {
			results = append(results, result)
		}
	}

	response := fmt.Sprintf("🔄 Batch Operations Completed\n✅ Successful: %d\n❌ Failed: %d\n\nResults:\n%s",
		len(results), len(errors), strings.Join(results, "\n"))

	if len(errors) > 0 {
		response += fmt.Sprintf("\n\nErrors:\n%s", strings.Join(errors, "\n"))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: response},
		},
	}, nil
}

// resolveStringField resolves a field value from multiple possible aliases.
// Returns the first non-empty string found, or ("", false) if none match.
func resolveStringField(op map[string]interface{}, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := op[k].(string); ok && v != "" {
			return v, true
		}
	}
	return "", false
}

// processBatchOperation - Procesa una operación individual del lote.
// Accepts "action" as alias for "type" (ported from ultra — Claude Desktop convention).
func (fs *FilesystemHandler) processBatchOperation(operation map[string]interface{}, opNum int) (string, error) {
	// Accept "type" or "action" for the operation type
	opType, ok := resolveStringField(operation, "type", "action")
	if !ok {
		return "", fmt.Errorf("missing 'type' (or 'action') field")
	}

	switch strings.ToLower(strings.TrimSpace(opType)) {
	case "rename", "move":
		return fs.processBatchMove(operation, opNum)
	case "copy", "cp":
		return fs.processBatchCopy(operation, opNum)
	case "delete", "remove", "rm":
		return fs.processBatchDelete(operation, opNum)
	case "create_dir", "mkdir":
		return fs.processBatchCreateDir(operation, opNum)
	case "write":
		return fs.processBatchWrite(operation, opNum)
	default:
		return "", fmt.Errorf("unsupported operation type: '%s' (supported: rename, move, copy, cp, delete, remove, rm, create_dir, mkdir, write)", opType)
	}
}

// processBatchMove - Procesa operación de mover/renombrar
// Accepts: from/source/src/path for origin; to/destination/dest/dst/target for target
func (fs *FilesystemHandler) processBatchMove(operation map[string]interface{}, opNum int) (string, error) {
	from, ok := resolveStringField(operation, "from", "source", "src", "path", "file")
	if !ok {
		return "", fmt.Errorf("missing 'from' (or 'source') field")
	}
	to, ok := resolveStringField(operation, "to", "destination", "dest", "dst", "target")
	if !ok {
		return "", fmt.Errorf("missing 'to' (or 'destination') field")
	}

	validFrom, err := fs.validatePath(from)
	if err != nil {
		return "", fmt.Errorf("invalid source path: %v", err)
	}

	validTo, err := fs.validatePath(to)
	if err != nil {
		return "", fmt.Errorf("invalid destination path: %v", err)
	}

	// Crear directorio padre si no existe
	parentDir := filepath.Dir(validTo)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %v", err)
	}

	if err := os.Rename(validFrom, validTo); err != nil {
		return "", fmt.Errorf("move failed: %v", err)
	}

	return fmt.Sprintf("  %d. ✅ Moved: %s → %s", opNum, from, to), nil
}

// processBatchCopy - Procesa operación de copiar
// Accepts: from/source/src for origin; to/destination/dest/dst/target for target
func (fs *FilesystemHandler) processBatchCopy(operation map[string]interface{}, opNum int) (string, error) {
	from, ok := resolveStringField(operation, "from", "source", "src", "path", "file")
	if !ok {
		return "", fmt.Errorf("missing 'from' (or 'source') field")
	}
	to, ok := resolveStringField(operation, "to", "destination", "dest", "dst", "target")
	if !ok {
		return "", fmt.Errorf("missing 'to' (or 'destination') field")
	}

	validFrom, err := fs.validatePath(from)
	if err != nil {
		return "", fmt.Errorf("invalid source path: %v", err)
	}

	validTo, err := fs.validatePath(to)
	if err != nil {
		return "", fmt.Errorf("invalid destination path: %v", err)
	}

	// Crear directorio padre si no existe
	parentDir := filepath.Dir(validTo)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %v", err)
	}

	if err := copyFile(validFrom, validTo); err != nil {
		return "", fmt.Errorf("copy failed: %v", err)
	}

	return fmt.Sprintf("  %d. ✅ Copied: %s → %s", opNum, from, to), nil
}

// processBatchDelete - Procesa operación de eliminar
// Accepts: path/from/source/src/file for the target path
func (fs *FilesystemHandler) processBatchDelete(operation map[string]interface{}, opNum int) (string, error) {
	path, ok := resolveStringField(operation, "path", "from", "source", "src", "file")
	if !ok {
		return "", fmt.Errorf("missing 'path' (or 'from' or 'source') field")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	info, err := os.Stat(validPath)
	if os.IsNotExist(err) {
		return fmt.Sprintf("  %d. ⚠️  Already deleted: %s", opNum, path), nil
	} else if err != nil {
		return "", fmt.Errorf("stat failed: %v", err)
	}

	recursive, _ := operation["recursive"].(bool)

	if info.IsDir() {
		if !recursive {
			return "", fmt.Errorf("directory deletion requires recursive=true")
		}
		if err := os.RemoveAll(validPath); err != nil {
			return "", fmt.Errorf("delete directory failed: %v", err)
		}
		return fmt.Sprintf("  %d. ✅ Deleted directory: %s", opNum, path), nil
	} else {
		if err := os.Remove(validPath); err != nil {
			return "", fmt.Errorf("delete file failed: %v", err)
		}
		return fmt.Sprintf("  %d. ✅ Deleted file: %s", opNum, path), nil
	}
}

// processBatchCreateDir - Procesa operación de crear directorio
// Accepts: path/from/target/destination for the directory path
func (fs *FilesystemHandler) processBatchCreateDir(operation map[string]interface{}, opNum int) (string, error) {
	path, ok := resolveStringField(operation, "path", "from", "target", "destination")
	if !ok {
		return "", fmt.Errorf("missing 'path' field")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	if err := os.MkdirAll(validPath, 0755); err != nil {
		return "", fmt.Errorf("create directory failed: %v", err)
	}

	return fmt.Sprintf("  %d. ✅ Created directory: %s", opNum, path), nil
}

// processBatchWrite - Procesa operación de escribir archivo
func (fs *FilesystemHandler) processBatchWrite(operation map[string]interface{}, opNum int) (string, error) {
	path, ok := operation["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing 'path' field")
	}
	content, ok := operation["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing 'content' field")
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	// Crear directorio padre si no existe
	parentDir := filepath.Dir(validPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %v", err)
	}

	if err := os.WriteFile(validPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write failed: %v", err)
	}

	return fmt.Sprintf("  %d. ✅ Written: %s (%d bytes)", opNum, path, len(content)), nil
}
