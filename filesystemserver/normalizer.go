package filesystemserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ─────────────────────────────────────────────────────────────────────────────
// Normalizer — ported from mcp-filesystem-go-ultra
//
// Makes every handler resilient to the many ways LLMs (especially Claude Desktop)
// can send parameters:
//   - Parameter aliases   (old_str → old_text, action → type, src → source …)
//   - Type coercion       ("true" → true, "3" → 3.0)
//   - JSON accept-both    (raw []interface{} ↔ JSON string for array params)
//   - Literal-escape fix  (literal \n inside JSON strings → real newline)
//
// ZERO new tools exposed — this is purely internal intelligence.
// ─────────────────────────────────────────────────────────────────────────────

// NormalizationRuleType defines the kind of normalization to apply.
type NormalizationRuleType string

const (
	RuleParamAlias    NormalizationRuleType = "param_alias"
	RuleTypeCoerce    NormalizationRuleType = "type_coerce"
	RuleJSONAcceptAny NormalizationRuleType = "json_accept_any"
)

// NormalizationRule describes one data-driven rule.
type NormalizationRule struct {
	ID       string                // human-readable identifier
	Tools    []string              // tool names this applies to ("*" = all)
	Type     NormalizationRuleType // kind of rule
	From     string                // source param / field name
	To       string                // target param name (alias rules)
	CoerceTo string                // target Go type: "bool", "number" (coerce rules)
}

// NormalizationApplied records a single normalization that fired.
type NormalizationApplied struct {
	RuleID string
	Type   string
	Param  string
	From   string
	To     string
}

// NormalizationResult is what Normalize returns.
type NormalizationResult struct {
	Args        map[string]interface{}
	Applied     []NormalizationApplied
	WasModified bool
}

// Normalizer holds the rule set and applies them to incoming requests.
type Normalizer struct {
	rules []NormalizationRule
}

// NewNormalizer builds a Normalizer with the built-in rule set.
func NewNormalizer() *Normalizer {
	return &Normalizer{rules: builtInRules()}
}

// builtInRules returns all data-driven normalization rules for the tools
// registered in this server.
func builtInRules() []NormalizationRule {
	// Tools that accept a "path" parameter
	pathTools := []string{
		"read_file", "write_file", "list_directory", "get_file_info",
		"create_directory", "delete_file", "search_files", "analyze_file",
		"tree", "find_duplicates", "analyze_project",
		"performance_analysis", "smart_search", "generate_report",
		"chunked_write", "split_file", "write_file_safe",
	}

	// Tools that accept a "path" for directories
	dirTools := []string{
		"list_directory", "create_directory", "tree",
		"find_duplicates", "analyze_project",
	}

	return []NormalizationRule{
		// ── Parameter aliases ────────────────────────────────────────────

		// edit_file: Claude often uses old_str / new_str instead of old_text / new_text
		{ID: "edit-old_str", Tools: []string{"edit_file"}, Type: RuleParamAlias, From: "old_str", To: "old_text"},
		{ID: "edit-new_str", Tools: []string{"edit_file"}, Type: RuleParamAlias, From: "new_str", To: "new_text"},
		// edit_file: also accept camelCase
		{ID: "edit-oldText", Tools: []string{"edit_file"}, Type: RuleParamAlias, From: "oldText", To: "old_text"},
		{ID: "edit-newText", Tools: []string{"edit_file"}, Type: RuleParamAlias, From: "newText", To: "new_text"},
		// edit_file: accept file_path / filepath as alias for path
		{ID: "edit-file_path", Tools: []string{"edit_file"}, Type: RuleParamAlias, From: "file_path", To: "path"},
		{ID: "edit-filepath", Tools: []string{"edit_file"}, Type: RuleParamAlias, From: "filepath", To: "path"},

		// Global: file_path / filepath → path
		{ID: "global-file_path", Tools: pathTools, Type: RuleParamAlias, From: "file_path", To: "path"},
		{ID: "global-filepath", Tools: pathTools, Type: RuleParamAlias, From: "filepath", To: "path"},

		// Global: directory / dir / folder → path
		{ID: "global-directory", Tools: dirTools, Type: RuleParamAlias, From: "directory", To: "path"},
		{ID: "global-dir", Tools: dirTools, Type: RuleParamAlias, From: "dir", To: "path"},
		{ID: "global-folder", Tools: dirTools, Type: RuleParamAlias, From: "folder", To: "path"},

		// write_file / write_file_safe: accept text / data as alias for content
		{ID: "write-text", Tools: []string{"write_file", "write_file_safe", "chunked_write"}, Type: RuleParamAlias, From: "text", To: "content"},
		{ID: "write-data", Tools: []string{"write_file", "write_file_safe", "chunked_write"}, Type: RuleParamAlias, From: "data", To: "content"},

		// copy_file / move_file: src/dst/from/to → source/destination
		{ID: "copy-src", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "src", To: "source"},
		{ID: "copy-from", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "from", To: "source"},
		{ID: "copy-dst", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "dst", To: "destination"},
		{ID: "copy-dest", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "dest", To: "destination"},
		{ID: "copy-to", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "to", To: "destination"},
		{ID: "copy-target", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "target", To: "destination"},

		// smart_sync: src/dst → source/target
		{ID: "sync-src", Tools: []string{"smart_sync"}, Type: RuleParamAlias, From: "src", To: "source"},
		{ID: "sync-dst", Tools: []string{"smart_sync"}, Type: RuleParamAlias, From: "dst", To: "target"},
		{ID: "sync-dest", Tools: []string{"smart_sync"}, Type: RuleParamAlias, From: "dest", To: "target"},
		{ID: "sync-destination", Tools: []string{"smart_sync"}, Type: RuleParamAlias, From: "destination", To: "target"},

		// compare_files: source/target/path1/path2 → file1/file2
		{ID: "compare-source", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "source", To: "file1"},
		{ID: "compare-target", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "target", To: "file2"},
		{ID: "compare-path1", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "path1", To: "file1"},
		{ID: "compare-path2", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "path2", To: "file2"},

		// read_multiple_files: accept "files" as alias for "paths"
		{ID: "multi-files", Tools: []string{"read_multiple_files"}, Type: RuleParamAlias, From: "files", To: "paths"},

		// search_files / smart_search: accept "query" / "regex" / "search" as alias for "pattern"
		{ID: "search-query", Tools: []string{"search_files", "smart_search"}, Type: RuleParamAlias, From: "query", To: "pattern"},
		{ID: "search-regex", Tools: []string{"search_files", "smart_search"}, Type: RuleParamAlias, From: "regex", To: "pattern"},
		{ID: "search-search", Tools: []string{"search_files", "smart_search"}, Type: RuleParamAlias, From: "search", To: "pattern"},

		// join_files: accept "output" / "path" as alias for "target_path"
		{ID: "join-output", Tools: []string{"join_files"}, Type: RuleParamAlias, From: "output", To: "target_path"},
		{ID: "join-path", Tools: []string{"join_files"}, Type: RuleParamAlias, From: "path", To: "target_path"},
		// join_files: accept "files" / "chunks" / "paths" as alias for "source_files"
		{ID: "join-files", Tools: []string{"join_files"}, Type: RuleParamAlias, From: "files", To: "source_files"},
		{ID: "join-chunks", Tools: []string{"join_files"}, Type: RuleParamAlias, From: "chunks", To: "source_files"},
		{ID: "join-paths", Tools: []string{"join_files"}, Type: RuleParamAlias, From: "paths", To: "source_files"},

		// plan_task: accept "task" as alias for "description"
		{ID: "plan-task", Tools: []string{"plan_task"}, Type: RuleParamAlias, From: "task", To: "description"},

		// ── Type coercion (global) ──────────────────────────────────────
		{ID: "recursive-bool", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "recursive", CoerceTo: "bool"},
		{ID: "follow_symlinks-bool", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "follow_symlinks", CoerceTo: "bool"},
		{ID: "include_content-bool", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "include_content", CoerceTo: "bool"},
		{ID: "create_backup-bool", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "create_backup", CoerceTo: "bool"},
		{ID: "depth-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "depth", CoerceTo: "number"},
		{ID: "timeout-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "timeout", CoerceTo: "number"},
		{ID: "chunk_index-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "chunk_index", CoerceTo: "number"},
		{ID: "total_chunks-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "total_chunks", CoerceTo: "number"},
		{ID: "chunk_size-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "chunk_size", CoerceTo: "number"},
		{ID: "start_line-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "start_line", CoerceTo: "number"},
		{ID: "end_line-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "end_line", CoerceTo: "number"},

		// ── JSON accept-both ────────────────────────────────────────────
		{ID: "paths-json", Tools: []string{"read_multiple_files"}, Type: RuleJSONAcceptAny, From: "paths"},
		{ID: "operations-json", Tools: []string{"batch_operations"}, Type: RuleJSONAcceptAny, From: "operations"},
		{ID: "file_types-json", Tools: []string{"smart_search"}, Type: RuleJSONAcceptAny, From: "file_types"},
		{ID: "sections-json", Tools: []string{"generate_report"}, Type: RuleJSONAcceptAny, From: "sections"},
		{ID: "exclude-json", Tools: []string{"smart_sync"}, Type: RuleJSONAcceptAny, From: "exclude_patterns"},
		{ID: "target_files-json", Tools: []string{"plan_task"}, Type: RuleJSONAcceptAny, From: "target_files"},
		{ID: "source_files-json", Tools: []string{"join_files"}, Type: RuleJSONAcceptAny, From: "source_files"},
	}
}

// Normalize applies all matching rules to the arguments map in-place.
func (n *Normalizer) Normalize(tool string, args map[string]interface{}) NormalizationResult {
	result := NormalizationResult{Args: args}

	for _, rule := range n.rules {
		if !ruleMatchesTool(rule, tool) {
			continue
		}
		var applied *NormalizationApplied
		switch rule.Type {
		case RuleParamAlias:
			applied = applyParamAlias(rule, args)
		case RuleTypeCoerce:
			applied = applyTypeCoerce(rule, args)
		case RuleJSONAcceptAny:
			applied = applyJSONAcceptBoth(rule, args)
		}
		if applied != nil {
			result.Applied = append(result.Applied, *applied)
			result.WasModified = true
		}
	}
	return result
}

// ruleMatchesTool checks if a rule applies to the given tool name.
func ruleMatchesTool(rule NormalizationRule, tool string) bool {
	for _, t := range rule.Tools {
		if t == "*" || t == tool {
			return true
		}
	}
	return false
}

// ── Rule implementations ────────────────────────────────────────────────────

// applyParamAlias renames From→To if From exists and To doesn't.
func applyParamAlias(rule NormalizationRule, args map[string]interface{}) *NormalizationApplied {
	fromVal, fromOK := args[rule.From]
	_, toOK := args[rule.To]
	if fromOK && !toOK {
		args[rule.To] = fromVal
		delete(args, rule.From)
		return &NormalizationApplied{
			RuleID: rule.ID, Type: string(rule.Type),
			Param: rule.From, From: rule.From, To: rule.To,
		}
	}
	return nil
}

// applyTypeCoerce converts string representations to native Go types.
func applyTypeCoerce(rule NormalizationRule, args map[string]interface{}) *NormalizationApplied {
	val, exists := args[rule.From]
	if !exists {
		return nil
	}

	switch rule.CoerceTo {
	case "bool":
		s, ok := val.(string)
		if !ok {
			return nil
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "1", "yes":
			args[rule.From] = true
		case "false", "0", "no":
			args[rule.From] = false
		default:
			return nil
		}
		return &NormalizationApplied{
			RuleID: rule.ID, Type: string(rule.Type),
			Param: rule.From, From: s, To: fmt.Sprintf("%v", args[rule.From]),
		}

	case "number":
		s, ok := val.(string)
		if !ok {
			return nil
		}
		s = strings.TrimSpace(s)
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
			return nil
		}
		args[rule.From] = f
		return &NormalizationApplied{
			RuleID: rule.ID, Type: string(rule.Type),
			Param: rule.From, From: s, To: fmt.Sprintf("%v", f),
		}
	}
	return nil
}

// applyJSONAcceptBoth handles params that can arrive as either a raw JSON
// array/object OR a JSON-encoded string. It normalises to the native type.
func applyJSONAcceptBoth(rule NormalizationRule, args map[string]interface{}) *NormalizationApplied {
	val, exists := args[rule.From]
	if !exists {
		return nil
	}

	// If it's already a slice or map → already native, nothing to do.
	switch val.(type) {
	case []interface{}, map[string]interface{}:
		return nil
	}

	// If it's a string, try to parse it as JSON.
	s, isStr := val.(string)
	if !isStr {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Only attempt parse if it looks like JSON array or object.
	if s[0] == '[' || s[0] == '{' {
		var parsed interface{}
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			args[rule.From] = parsed
			return &NormalizationApplied{
				RuleID: rule.ID, Type: string(rule.Type),
				Param: rule.From, From: "string", To: "native",
			}
		}
	}
	return nil
}

// ── Literal escape fix ──────────────────────────────────────────────────────

// FixLiteralEscapes repairs strings where Claude Desktop sends literal
// backslash-n (two chars: '\' + 'n') instead of a real newline ('\n').
// This is applied selectively to text-content params like old_text / new_text.
func FixLiteralEscapes(s string) string {
	// Fast path: nothing to fix
	if !strings.Contains(s, `\n`) && !strings.Contains(s, `\t`) && !strings.Contains(s, `\r`) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '\\' {
			next := s[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
				i += 2
				continue
			case 't':
				b.WriteByte('\t')
				i += 2
				continue
			case 'r':
				b.WriteByte('\r')
				i += 2
				continue
			case '\\':
				b.WriteByte('\\')
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// ── Handler wrapper ─────────────────────────────────────────────────────────

// withNormalize wraps a handler with request normalization.
// This is the single integration point — every handler goes through here.
func (fs *FilesystemHandler) withNormalize(tool string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	normalizer := NewNormalizer()
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fs.tryFetchRoots(ctx)
		if args := request.GetArguments(); args != nil {
			normalizer.Normalize(tool, args)
		}
		return handler(ctx, request)
	}
}
