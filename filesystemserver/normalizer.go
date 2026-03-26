package filesystemserver

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

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
		"create_directory", "delete_file", "search",
		"tree", "analyze_project",
	}

	// Tools that accept a "path" for directories
	dirTools := []string{
		"list_directory", "create_directory", "tree",
		"search", "analyze_project",
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

		// write_file: accept text / data as alias for content
		{ID: "write-text", Tools: []string{"write_file"}, Type: RuleParamAlias, From: "text", To: "content"},
		{ID: "write-data", Tools: []string{"write_file"}, Type: RuleParamAlias, From: "data", To: "content"},

		// copy_file / move_file: src/dst/from/to → source/destination
		{ID: "copy-src", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "src", To: "source"},
		{ID: "copy-from", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "from", To: "source"},
		{ID: "copy-dst", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "dst", To: "destination"},
		{ID: "copy-dest", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "dest", To: "destination"},
		{ID: "copy-to", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "to", To: "destination"},
		{ID: "copy-target", Tools: []string{"copy_file", "move_file"}, Type: RuleParamAlias, From: "target", To: "destination"},

		// compare_files: source/target/path1/path2 → file1/file2
		{ID: "compare-source", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "source", To: "file1"},
		{ID: "compare-target", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "target", To: "file2"},
		{ID: "compare-path1", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "path1", To: "file1"},
		{ID: "compare-path2", Tools: []string{"compare_files"}, Type: RuleParamAlias, From: "path2", To: "file2"},

		// read_multiple_files: accept "files" as alias for "paths"
		{ID: "multi-files", Tools: []string{"read_multiple_files"}, Type: RuleParamAlias, From: "files", To: "paths"},

		// search: accept "query" / "regex" as alias for "pattern"; "type" as alias for "mode"
		{ID: "search-query", Tools: []string{"search"}, Type: RuleParamAlias, From: "query", To: "pattern"},
		{ID: "search-regex", Tools: []string{"search"}, Type: RuleParamAlias, From: "regex", To: "pattern"},
		{ID: "search-type", Tools: []string{"search"}, Type: RuleParamAlias, From: "type", To: "mode"},

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
		{ID: "context_lines-number", Tools: []string{"*"}, Type: RuleTypeCoerce, From: "context_lines", CoerceTo: "number"},

		// ── JSON accept-both ────────────────────────────────────────────
		{ID: "paths-json", Tools: []string{"read_multiple_files"}, Type: RuleJSONAcceptAny, From: "paths"},
		{ID: "operations-json", Tools: []string{"batch_operations"}, Type: RuleJSONAcceptAny, From: "operations"},
		{ID: "file_types-json", Tools: []string{"search"}, Type: RuleJSONAcceptAny, From: "file_types"},
		{ID: "target_files-json", Tools: []string{"plan_task"}, Type: RuleJSONAcceptAny, From: "target_files"},
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

// withNormalize wraps a handler with rate limiting, request normalization,
// and output sanitization. This is the single integration point — every
// handler goes through here.
func (fs *FilesystemHandler) withNormalize(tool string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	normalizer := NewNormalizer()
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Rate limiting (before any work)
		if !fs.limiter.allow() {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Rate limit exceeded: max %d calls per %s. Please wait before retrying.",
							fs.limiter.maxCalls, fs.limiter.window),
					},
				},
				IsError: true,
			}, nil
		}

		ctx = withAuditContext(ctx, extractRequestIDFromToolRequest(request))
		start := time.Now().UTC()
		rootsFetched := fs.tryFetchRoots(ctx)
		if rootsFetched {
			appendSubOperation(ctx, "roots_sync")
		}
		rawArgs := summarizeArguments(request.GetArguments())
		if args := request.GetArguments(); args != nil {
			normalizer.Normalize(tool, args)
		}
		normalizedArgs := summarizeArguments(request.GetArguments())
		if !reflect.DeepEqual(rawArgs, normalizedArgs) {
			appendSubOperation(ctx, "normalize_arguments")
		}
		result, err := handler(ctx, request)

		// Sanitize text content in output and add content annotations
		if result != nil {
			audience := contentAudienceForTool(tool)
			for i, content := range result.Content {
				if tc, ok := content.(mcp.TextContent); ok {
					tc.Text = sanitizeOutput(tc.Text)
					if tc.Annotations == nil && audience != nil {
						tc.Annotations = &mcp.Annotations{Audience: audience}
					}
					result.Content[i] = tc
				}
			}
		}

		fs.logToolCall(ctx, start, tool, rawArgs, normalizedArgs, buildInternalAction(rootsFetched, rawArgs, normalizedArgs), result, err)
		return result, err
	}
}

// contentAudienceForTool returns the appropriate audience annotation for a tool's output.
// Read tools produce content primarily for the assistant; write/edit tools produce
// confirmations for the user.
func contentAudienceForTool(tool string) []mcp.Role {
	switch tool {
	case "read_file", "read_multiple_files", "read_media_file",
		"search", "tree", "get_file_info", "analyze_project",
		"compare_files", "list_directory", "list_allowed_directories":
		return []mcp.Role{mcp.RoleAssistant}
	case "write_file", "edit_file", "copy_file", "move_file",
		"delete_file", "create_directory", "batch_operations":
		return []mcp.Role{mcp.RoleUser, mcp.RoleAssistant}
	default:
		return nil
	}
}
