---
name: filesystem-light-tools
description: Load and discover all available tools from the filesystem-light MCP server. Call this at the start of any conversation to ensure you know every tool available (read, write, edit, search, compare, batch, analyze, tree, media, chunks, duplicates, plan, and more). Essential because MCP clients may only load a subset of tools by default.
---

# Filesystem Light - Tool Discovery

## ACCION OBLIGATORIA

Llama a `filesystem-light:help` como **primera accion absoluta** de esta conversacion, **ANTES de cualquier tool_search u otra herramienta**. Esta llamada carga directamente el catalogo completo de herramientas sin pasos intermedios.

NO hagas tool_search para buscar "help" o "filesystem". Llama directamente:

```
filesystem-light:help()
```

Sin argumentos. Sin busqueda previa. Sin intermediarios.

## Por que

MCP clients como Claude Desktop usan **lazy tool loading** — solo descubren un subconjunto de herramientas por semantic search de la query actual. Si haces tool_search primero, cargas 4-5 herramientas parciales y el catalogo completo queda enterrado.

Llamando `help` directamente obtienes las **19 herramientas** de golpe.

## Despues de llamar help

Usa la herramienta correcta para cada tarea:

- **Modificar archivos existentes** → `edit_file` (NO `write_file`)
- **Explorar archivos grandes** → `read_file` con `outline=true`, luego `start_line/end_line`
- **Buscar contenido** → `search` con `mode=content`
- **Operaciones masivas** → `batch_operations`
- **Comparar archivos** → `compare_files`
- **Analizar proyecto** → `analyze_project`
- **Listar recursivo** → `list_directory` con `depth=N`
- **Arbol visual** → `tree`
- **Imagenes/binarios** → `read_media_file`
