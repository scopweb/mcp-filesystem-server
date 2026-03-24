# Mejoras prioritarias para mcp-filesystem-go-server

Implementar las 5 mejoras identificadas en la comparativa con el servidor oficial MCP filesystem, alineando el proyecto con la especificación MCP actual.

## User Review Required

> [!IMPORTANT]
> La actualización de `mcp-go` de v0.26.0 a v0.45.0 es un **salto de 19 versiones**. Es posible que haya breaking changes en la API. Lo gestionaré corrigiendo cualquier incompatibilidad.

> [!WARNING]
> MCP Roots cambia el modelo de control de acceso: los directorios permitidos podrán ser controlados dinámicamente por el cliente. Esto es comportamiento esperado según la spec, pero cambia quién tiene autoridad sobre los permisos.

## Proposed Changes

### Prerequisite: Upgrade mcp-go SDK

#### [MODIFY] [go.mod](file:///c:/MCPs/clone/mcp-filesystem-go-server/go.mod)
- Actualizar `github.com/mark3labs/mcp-go` de `v0.26.0` a `v0.45.0`
- Run `go get github.com/mark3labs/mcp-go@v0.45.0` y `go mod tidy`

---

### 1. Tool Annotations

#### [MODIFY] [server.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/server.go)
Añadir annotations a cada `mcp.NewTool(...)` usando las nuevas opciones del SDK:

| Tool | readOnlyHint | idempotentHint | destructiveHint |
|---|---|---|---|
| `read_file` | `true` | — | — |
| `read_multiple_files` | `true` | — | — |
| `list_directory` | `true` | — | — |
| `tree` | `true` | — | — |
| `search_files` | `true` | — | — |
| `get_file_info` | `true` | — | — |
| `list_allowed_directories` | `true` | — | — |
| `analyze_file` | `true` | — | — |
| `analyze_project` | `true` | — | — |
| `smart_search` | `true` | — | — |
| `find_duplicates` | `true` | — | — |
| `compare_files` | `true` | — | — |
| `performance_analysis` | `true` | — | — |
| `generate_report` (sin output file) | `true` | — | — |
| `create_directory` | `false` | `true` | `false` |
| `write_file` | `false` | `true` | `true` |
| `write_file_safe` | `false` | `true` | `true` |
| `edit_file` | `false` | `false` | `true` |
| `delete_file` | `false` | `false` | `true` |
| `copy_file` | `false` | `true` | `false` |
| `move_file` | `false` | `false` | `true` |
| `batch_operations` | `false` | `false` | `true` |
| `smart_sync` | `false` | `false` | `true` |
| `assist_refactor` | `false` | `false` | `true` |
| `plan_task` | `true` | — | — |
| `chunked_write` | `false` | `false` | `true` |
| `split_file` | `false` | `false` | `false` |
| `join_files` | `false` | `false` | `false` |

Ejemplo de cambio:
```diff
 s.AddTool(mcp.NewTool(
     "read_file",
     mcp.WithDescription("Read the complete contents of a file from the file system."),
+    mcp.WithReadOnlyHintAnnotation(true),
+    mcp.WithDestructiveHintAnnotation(false),
     mcp.WithString("path",
```

---

### 2. MCP Roots Support

#### [MODIFY] [server.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/server.go)
- Añadir `server.WithRootListWatcher()` o equivalente en la inicialización del servidor
- Crear método en [FilesystemHandler](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go#36-40) para actualizar `allowedDirs` dinámicamente a partir de Roots

#### [MODIFY] [handler_core.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler_core.go)
- Añadir método `UpdateAllowedDirs(dirs []string)` al handler con mutex para thread safety

#### [MODIFY] [types.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go)
- Añadir `sync.RWMutex` al [FilesystemHandler](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go#36-40) struct

#### [MODIFY] [main.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/main.go)
- Permitir iniciar sin argumentos CLI (no hacer `os.Exit(1)`) — los Roots del cliente los proporcionarán

---

### 3. Herramienta `read_media_file`

#### [MODIFY] [server.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/server.go)
- Registrar nueva herramienta `read_media_file` con parámetro [path](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler_utils.go#84-88)

#### [MODIFY] [handler_core.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler_core.go)
- Añadir `handleReadMediaFile` que:
  - Valida el path
  - Detecta MIME type
  - Lee archivo en base64
  - Retorna `mcp.ImageContent` o `mcp.AudioContent` según tipo

---

### 4. Mejora de `edit_file` con dryRun

#### [MODIFY] [server.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/server.go)
- Añadir parámetro `dry_run` (boolean) a la definición del tool `edit_file`

#### [MODIFY] [handler.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler.go)
- Modificar `handleEditFile` para:
  - Leer el parámetro `dry_run`
  - Si `dry_run=true`: calcular el diff pero NO escribir al archivo, devolver el diff como preview

---

### 5. Escritura atómica en `write_file`

#### [MODIFY] [handler_core.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler_core.go)
- En [handleWriteFile](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler_core.go#221-307): escribir a archivo temporal + `os.Rename()` en lugar de `os.WriteFile()` directo
- Patron: escribir a `path.tmp.{random}` → `os.Rename(tmp, path)` → cleanup en caso de error

---

## Verification Plan

### Automated Tests

1. **Tests existentes** (deben seguir pasando):
```bash
cd c:\MCPs\clone\mcp-filesystem-go-server
go test ./filesystemserver -v
```

2. **Build completo** (sin errores de compilación):
```bash
cd c:\MCPs\clone\mcp-filesystem-go-server
go build ./...
```

3. **Vet** (sin warnings):
```bash
cd c:\MCPs\clone\mcp-filesystem-go-server
go vet ./...
```

### Manual Verification
- Verificar que el servidor arranca sin argumentos CLI (para test de Roots) sin crash
- Confirmar que los tests existentes (54 tests aprox.) siguen pasando tras upgrade de SDK
