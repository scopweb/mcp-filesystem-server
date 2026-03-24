# Comparativa: Tu proyecto (Go) vs MCP Filesystem Server oficial (TypeScript)

## Resumen general

| Aspecto | **Tu proyecto** (Go) | **Oficial** (TypeScript/Node.js) |
|---|---|---|
| Lenguaje | Go | TypeScript (Node.js) |
| SDK | `mcp-go` (mark3labs) | `@modelcontextprotocol/sdk` |
| Herramientas | **27** | **14** (incl. `read_file` deprecado) |
| Normalizer | ✅ 50+ reglas de alias y coerción | ❌ No tiene |
| MCP Roots | ❌ No soportado | ✅ Soporte dinámico |
| Tool Annotations | ❌ No definidas | ✅ (`readOnlyHint`, `idempotentHint`, `destructiveHint`) |
| Resource handler | ✅ (`file://`) | ❌ No expone resources |
| Docker | ✅ | ✅ |
| Tests | ✅ ([handler_test.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/handler_test.go), [file_edit_test.go](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/file_edit_test.go)) | ✅ (`__tests__/`) |

---

## Herramientas — Comparativa lado a lado

### Herramientas comunes (ambos proyectos)

| Herramienta | Tu proyecto | Oficial | Notas |
|---|---|---|---|
| Leer archivo | `read_file` | `read_text_file` + `read_file` (dep.) | El oficial añade `head`/`tail` por líneas |
| Leer múltiples archivos | `read_multiple_files` | `read_multiple_files` | Equivalentes |
| Escribir archivo | `write_file` | `write_file` | El oficial usa escritura atómica (temp + rename) |
| Editar archivo | `edit_file` (old_text/new_text) | `edit_file` (array de edits + dryRun) | El oficial soporta múltiples edits y modo preview |
| Crear directorio | `create_directory` | `create_directory` | Equivalentes |
| Listar directorio | `list_directory` | `list_directory` | Equivalentes |
| Mover archivo | `move_file` | `move_file` | Equivalentes |
| Buscar archivos | `search_files` | `search_files` | El oficial añade `excludePatterns` |
| Árbol de directorio | `tree` | `directory_tree` | Tu versión añade `depth` y `follow_symlinks` |
| Info de archivo | `get_file_info` | `get_file_info` | Equivalentes |
| Directorios permitidos | `list_allowed_directories` | `list_allowed_directories` | Equivalentes |

### Herramientas exclusivas del proyecto oficial

| Herramienta | Descripción |
|---|---|
| `read_media_file` | Lee archivos de imagen/audio y devuelve base64 con MIME type |
| `list_directory_with_sizes` | Lista con tamaños de archivo y opción de ordenar por nombre/tamaño |

### Herramientas exclusivas de tu proyecto (Go)

| Categoría | Herramienta | Descripción |
|---|---|---|
| **Archivos** | `copy_file` | Copiar archivos/directorios |
| | `delete_file` | Eliminar archivos con opción recursiva |
| | `write_file_safe` | Escritura atómica con backup opcional |
| **Análisis** | `analyze_file` | Análisis profundo: métricas de complejidad, hashes, dependencias |
| | `analyze_project` | Estructura completa del proyecto con detección de lenguajes |
| | `smart_search` | Búsqueda con regex, contenido y filtro por tipo de archivo |
| | `find_duplicates` | Buscar archivos duplicados por hash |
| | `compare_files` | Comparación avanzada con generación de diff |
| | `performance_analysis` | Análisis de rendimiento del filesystem |
| | `generate_report` | Reportes en JSON/HTML/Markdown |
| **Batch** | `batch_operations` | Operaciones masivas (rename, delete, copy) |
| | `assist_refactor` | Asistencia de refactoring con análisis de dependencias |
| | `plan_task` | Planificación paso a paso de operaciones complejas |
| | `smart_sync` | Sincronización inteligente con detección de conflictos |
| **Archivos grandes** | `chunked_write` | Escritura por fragmentos para archivos grandes |
| | `split_file` | Dividir archivos grandes en chunks |
| | `join_files` | Unir fragmentos en un solo archivo |

---

## Características arquitectónicas

### Lo que tu proyecto tiene y el oficial NO

| Característica | Detalle |
|---|---|
| **Normalizer** | 50+ reglas de alias de parámetros, coerción de tipos, escape de literales, edits idempotentes, batch flexible |
| **Rate limiter** | Control de frecuencia de operaciones |
| **Resource handler** | Expone `file://` como recurso MCP |
| **Tipos ricos** | Estructuras Go para análisis: [FileAnalysis](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go#87-103), [CodeComplexity](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go#111-117), [ProjectStructure](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go#126-135), [FileDiff](file:///c:/MCPs/clone/mcp-filesystem-go-server/filesystemserver/types.go#42-51), etc. |
| **Operaciones de archivos grandes** | Chunked write, split, join — el oficial no tiene equivalente |
| **Refactoring asistido** | `assist_refactor` con operaciones rename/extract/inline/move |

### Lo que el oficial tiene y tu proyecto NO

| Característica | Detalle | Impacto |
|---|---|---|
| **MCP Roots** | Directorios permitidos dinámicos vía `roots/list` + `roots/list_changed` | ⚠️ **Alto** — Es la forma recomendada por la spec MCP para control de acceso |
| **Tool Annotations** | `readOnlyHint`, `idempotentHint`, `destructiveHint` por herramienta | ⚠️ **Medio** — Los clientes pueden usar esto para UX/seguridad |
| **Lectura de media** | `read_media_file` con detección de MIME y base64 streaming | 🔵 Medio — Útil para imágenes/audio |
| **Edits múltiples + dryRun** | Soporta array de edits en una sola llamada y preview con diff | 🔵 Medio — Tu `edit_file` solo edita un bloque por llamada |
| **Escritura atómica** | Usa temp file + rename para prevenir symlink attacks y race conditions | ⚠️ **Alto** — Patrón de seguridad importante |
| **Resolución de path relativo** | Resuelve paths relativos contra directorios permitidos | 🔵 Bajo — Mejora de usabilidad |

---

## Recomendaciones

### Prioridad alta — Alinear con la spec MCP
1. **Implementar MCP Roots** — Es el método recomendado por la especificación oficial para control dinámico de directorios
2. **Añadir Tool Annotations** — Los clientes MCP modernos las utilizan para mejorar la UX y seguridad

### Prioridad media — Paridad funcional
3. **Añadir `read_media_file`** — Soporte para archivos binarios (imágenes/audio) con base64
4. **Mejorar `edit_file`** — Soportar array de edits y modo `dryRun` como el oficial
5. **Escritura atómica en `write_file`** — Usar temp + rename para prevenir race conditions

### Prioridad baja — Mejoras opcionales
6. **Añadir `list_directory_with_sizes`** — Listado con tamaños e info estadística
7. **Soporte de `head`/`tail` en `read_file`** — Lectura parcial de archivos por líneas

---

## Conclusión

Tu proyecto **tiene significativamente más funcionalidad** que el servidor oficial (27 vs 14 tools), especialmente en análisis, batch operations, y manejo de archivos grandes. El **Normalizer** es una ventaja única y muy útil para la interoperabilidad con LLMs.

Sin embargo, el servidor oficial está **más alineado con la especificación MCP** actual gracias a MCP Roots y Tool Annotations. Estos son estándares que los clientes MCP esperan encontrar.

La mayor oportunidad es **combinar ambos**: mantener tu funcionalidad extendida + adoptar los patrones de la spec MCP (Roots, Annotations, escritura atómica).
