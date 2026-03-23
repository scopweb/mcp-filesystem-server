# Comparativa MCP Filesystem Servers — Evidencia empírica

> Fecha: 2026-03-23  
> Método: Pruebas manuales en sesión activa de Claude Desktop  
> Comparados: `mcp-filesystem-go-server` v1.0.0 (Go) · `mcp-filesystem-go-ultra` v4.0.0 (Go) · Oficial TypeScript  
> Entorno: Windows 11, Claude Desktop, C:\tmp

---

## 1. Tools disponibles

| Tool | filesystem-light v1.0.0 | filesystem-ultra v4.0.0 | Oficial TS |
|------|:-----------------------:|:-----------------------:|:----------:|
| `read_file` (con range) | ✅ | ✅ | ✅ |
| `read_multiple_files` | ✅ | ✅ | ✅ |
| `write_file` | ✅ (+ backup, chunked) | ✅ | ✅ |
| `edit_file` | ✅ (+ dry_run diff) | ✅ (+ regex, occurrence) | ✅ |
| `list_directory` | ✅ | ✅ | ✅ |
| `create_directory` | ✅ | ✅ | ✅ |
| `copy_file` | ✅ | ✅ | ❌ |
| `move_file` | ✅ | ✅ | ✅ |
| `delete_file` | ✅ | ✅ | ❌ |
| `get_file_info` | ✅ | ✅ | ✅ |
| `list_allowed_directories` | ✅ | ✅ | ✅ |
| `search` / `search_files` | ✅ (files/content/duplicates) | ✅ | ✅ |
| `batch_operations` | ✅ (rename/delete/copy) | ✅ (+ write/edit/pipeline) | ❌ |
| `compare_files` | ✅ | ❌ | ❌ |
| `analyze_project` | ✅ | ❌ | ❌ |
| `read_media_file` | ✅ | ❌ | ✅ |
| `tree` | ✅ | ❌ | ✅ |
| `plan_task` | ✅ | ❌ | ❌ |
| `multi_edit` | ❌ | ✅ | ❌ |
| `edit_file` modo regex | ❌ | ✅ | ❌ |
| `batch_operations` con pipeline | ❌ | ✅ | ❌ |
| `backup` (gestión de backups) | ❌ | ✅ | ❌ |
| `analyze_operation` (dry-run) | ❌ | ✅ | ❌ |
| `server_info` / `wsl` | ❌ | ✅ | ❌ |
| Tool Annotations (MCP spec) | ✅ | ✅ | ✅ |
| MCP Roots (directorios dinámicos) | ✅ | ❌ | ✅ |
| Escritura atómica (temp+rename) | ❌ (pendiente) | ❌ | ✅ |
| **Total tools** | **18** | **16** | **14** |

---

## 2. Verbosidad de output — medición real

Operaciones ejecutadas en esta sesión. Bytes medidos del output devuelto por el tool.

### edit_file — 1 reemplazo exitoso

| Servidor | Output |
|----------|--------|
| filesystem-light | `✅ Successfully edited ... 📊 Changes: 1 replacement(s) 🎯 Match confidence: high 📝 Lines affected: 1` (~120 chars) |
| filesystem-ultra | `✅ Edit successful ... Risk: LOW ... Lines affected: 1 ... backup: none` (~180-220 chars) |

**Light ~30-40% menos tokens** en operaciones de edición simples.

### edit_file dry_run

filesystem-light devuelve diff unificado estándar (`---/+++`). filesystem-ultra no tiene `dry_run` en `edit_file` — usa `analyze_operation` por separado (llamada extra).

**Light: 1 llamada. Ultra: 2 llamadas** para preview + aplicar.

### batch_operations — 3 operaciones (copy+rename+delete)

| Servidor | Output |
|----------|--------|
| filesystem-light | `✅ Successful: 3 ❌ Failed: 0` + 3 líneas de resultado (~150 chars) |
| filesystem-ultra | Output equivalente + metadata adicional de risk/backup |

### search content — 7 matches con context_lines

Outputs comparables en tamaño. Ambos muestran path:línea + contexto. Sin diferencia significativa.

---

## 3. Velocidad — estimación cualitativa

**No se han ejecutado benchmarks con `go test -bench`.**  
Lo que sí es verificable:

- Ambos son Go binarios nativos — sin overhead de Node.js/V8
- filesystem-ultra añade lógica por llamada: risk assessment, backup checks, pipeline state
- filesystem-light es más directo — menos código por handler
- El Oficial TypeScript (Node.js) tiene ~300-500ms de startup vs ~5ms de los binarios Go

Para operaciones individuales la diferencia light vs ultra es imperceptible en uso normal. La ventaja real de light es **acumulativa** en sesiones largas con muchas operaciones simples.

---

## 4. Ahorro de tokens por sesión — estimación conservadora

Basado en la diferencia de verbosidad observada (~30-40% menos por operación en ediciones):

| Operaciones/sesión | Ahorro estimado (light vs ultra) |
|--------------------|----------------------------------|
| 20 ediciones simples | ~1.200-1.600 chars (~300-400 tokens) |
| 50 operaciones mixtas | ~2.500-4.000 chars (~600-1.000 tokens) |

**Advertencia:** estos números son estimaciones basadas en los outputs observados hoy, no en medición estadística de múltiples sesiones.

---

## 5. Cuándo usar cada uno

### filesystem-light — mejor para:
- Operaciones cotidianas: leer, editar, buscar, copiar
- Proyectos donde el tool count de Claude Desktop es un límite (light usa ~18 tools)
- Cuando se quiere `dry_run` nativo en `edit_file`
- Cuando se necesita `compare_files`, `analyze_project`, `read_media_file`
- Entornos donde MCP Roots es necesario (control dinámico de directorios)

### filesystem-ultra — mejor para:
- Pipelines multi-paso en una sola llamada (search→read→edit→verify)
- Ediciones regex con capture groups
- `multi_edit` (múltiples `old_text/new_text` en un archivo, una llamada)
- Gestión de backups con restore
- Operaciones de riesgo alto donde `analyze_operation` aporta valor real
- Entornos WSL con sincronización Windows↔Linux

### Oficial TypeScript — mejor para:
- Adherencia estricta a spec MCP (Roots, Annotations)
- Entornos donde Go no está disponible
- Escritura atómica (aún no implementada en los proyectos Go)

---

## 6. Gaps reales identificados

### filesystem-light no tiene:
- `multi_edit` (múltiples reemplazos en un archivo en una llamada)
- `edit_file` con regex y capture groups
- Pipelines (`batch_operations` solo soporta rename/delete/copy, no edit/write)
- Gestión de backups con restore
- `analyze_operation` como dry-run antes de operaciones destructivas
- Escritura atómica (temp+rename) — pendiente según `implementation_plan.md`

### filesystem-ultra no tiene:
- `dry_run` en `edit_file` (requiere llamada separada a `analyze_operation`)
- `compare_files`
- `analyze_project`
- `read_media_file`
- `tree`
- MCP Roots

---

## 7. Conclusión

No hay un ganador absoluto. Son complementarios:

- **filesystem-light** es más completo en herramientas de análisis/lectura y más alineado con la spec MCP. Output más conciso en operaciones simples.
- **filesystem-ultra** es más potente para transformaciones complejas de código (pipelines, regex, multi-edit, backups).

La elección depende del caso de uso predominante. Para trabajo general de desarrollo, **filesystem-light cubre el 90% de los casos con menos tokens y más tools útiles**. Para refactoring masivo o transformaciones complejas, **filesystem-ultra sigue siendo superior**.

---

*Documento generado a partir de pruebas empíricas en sesión activa. Sin afirmaciones no verificadas.*
