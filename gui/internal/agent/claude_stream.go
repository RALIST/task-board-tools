package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// claudeMaxToolOutputLines caps how many lines of a single tool_result we
// keep in the readable log. The full JSON-form output is still preserved by
// the upstream JSONL state writer; this limit just keeps the human-facing
// log from being dominated by one runaway command.
const claudeMaxToolOutputLines = 200

// claudeMaxEditPreviewLines caps how many lines of an Edit / Write input
// preview we render. Same rationale — the JSONL stream has the full bytes.
const claudeMaxEditPreviewLines = 40

// claudeTranslator wraps an io.Writer and converts each line of Claude's
// `--output-format stream-json` stream into a human-readable Codex-style
// block on the underlying writer. Lines that don't parse as JSON, or that
// don't match a known event shape, pass through unchanged so the runner
// stays resilient to schema drift.
//
// Contract:
//   - One Write call carries one full stream-json event (runExternal does
//     line-based scanning, so the upstream guarantees one line per Write).
//   - Output is rendered as one or more newline-terminated lines per event.
//   - Returns len(p) on the input even when the translated output is longer.
//     This keeps the io.Writer accounting honest from runExternal's POV.
type claudeTranslator struct {
	out io.Writer
}

func newClaudeTranslator(out io.Writer) io.Writer {
	return &claudeTranslator{out: out}
}

func (t *claudeTranslator) Write(p []byte) (int, error) {
	trimmed := bytes.TrimRight(p, "\r\n")
	if len(trimmed) == 0 {
		return len(p), nil
	}

	var ev map[string]any
	if err := json.Unmarshal(trimmed, &ev); err != nil {
		return t.passthrough(p)
	}

	typ, _ := ev["type"].(string)
	var lines []string
	switch typ {
	case "system":
		sub, _ := ev["subtype"].(string)
		if sub == "init" {
			lines = formatClaudeInit(ev)
		}
	case "assistant":
		lines = formatClaudeAssistant(ev)
	case "user":
		lines = formatClaudeUser(ev)
	case "result":
		lines = formatClaudeResult(ev)
	}

	if len(lines) == 0 {
		return len(p), nil
	}
	return t.writeLines(p, lines)
}

func (t *claudeTranslator) passthrough(p []byte) (int, error) {
	if _, err := t.out.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// writeLines emits each rendered line followed by '\n' as a separate Write
// to the underlying sink. Per-line writes keep the lineSink JSONL events
// one-line-per-event, mirroring how Codex's plain output is already split.
func (t *claudeTranslator) writeLines(orig []byte, lines []string) (int, error) {
	for _, line := range lines {
		if _, err := t.out.Write([]byte(line + "\n")); err != nil {
			return 0, err
		}
	}
	return len(orig), nil
}

func formatClaudeInit(ev map[string]any) []string {
	lines := []string{"Claude Code", "--------"}
	if v, _ := ev["cwd"].(string); v != "" {
		lines = append(lines, "workdir: "+v)
	}
	if v, _ := ev["model"].(string); v != "" {
		lines = append(lines, "model: "+v)
	}
	if v, _ := ev["permissionMode"].(string); v != "" {
		lines = append(lines, "permission mode: "+v)
	}
	if v, _ := ev["session_id"].(string); v != "" {
		lines = append(lines, "session id: "+v)
	}
	if tools, ok := ev["tools"].([]any); ok && len(tools) > 0 {
		names := make([]string, 0, len(tools))
		for _, tt := range tools {
			if s, ok := tt.(string); ok {
				names = append(names, s)
			}
		}
		if len(names) > 0 {
			lines = append(lines, "tools: "+strings.Join(names, ", "))
		}
	}
	if mcp, ok := ev["mcp_servers"].([]any); ok && len(mcp) > 0 {
		names := make([]string, 0, len(mcp))
		for _, m := range mcp {
			if mm, ok := m.(map[string]any); ok {
				if n, _ := mm["name"].(string); n != "" {
					names = append(names, n)
				}
			}
		}
		if len(names) > 0 {
			lines = append(lines, "mcp: "+strings.Join(names, ", "))
		}
	}
	lines = append(lines, "--------")
	return lines
}

func formatClaudeAssistant(ev map[string]any) []string {
	msg, ok := ev["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return nil
	}
	var lines []string
	for _, c := range content {
		block, ok := c.(map[string]any)
		if !ok {
			continue
		}
		bt, _ := block["type"].(string)
		switch bt {
		case "text":
			txt, _ := block["text"].(string)
			txt = strings.TrimRight(txt, "\n")
			if txt == "" {
				continue
			}
			lines = append(lines, "claude")
			lines = append(lines, strings.Split(txt, "\n")...)
			lines = append(lines, "")
		case "tool_use":
			name, _ := block["name"].(string)
			id, _ := block["id"].(string)
			header := "exec " + name
			if id != "" {
				header += " (" + shortenID(id) + ")"
			}
			lines = append(lines, header)
			lines = append(lines, formatClaudeToolInput(name, block["input"])...)
			lines = append(lines, "")
		}
	}
	return lines
}

func formatClaudeToolInput(name string, raw any) []string {
	input, ok := raw.(map[string]any)
	if !ok {
		if raw == nil {
			return nil
		}
		b, _ := json.MarshalIndent(raw, "", "  ")
		return strings.Split(string(b), "\n")
	}

	switch name {
	case "Bash":
		cmd, _ := input["command"].(string)
		if cmd != "" {
			return []string{"$ " + cmd}
		}
	case "Edit":
		path, _ := input["file_path"].(string)
		out := []string{"path: " + path}
		if old, _ := input["old_string"].(string); old != "" {
			out = append(out, "--- old ---")
			out = append(out, truncateLines(old, claudeMaxEditPreviewLines)...)
		}
		if ns, _ := input["new_string"].(string); ns != "" {
			out = append(out, "+++ new +++")
			out = append(out, truncateLines(ns, claudeMaxEditPreviewLines)...)
		}
		return out
	case "Write":
		path, _ := input["file_path"].(string)
		out := []string{"path: " + path}
		if content, _ := input["content"].(string); content != "" {
			out = append(out, "+++ content +++")
			out = append(out, truncateLines(content, claudeMaxEditPreviewLines)...)
		}
		return out
	case "MultiEdit":
		path, _ := input["file_path"].(string)
		out := []string{"path: " + path}
		if edits, ok := input["edits"].([]any); ok {
			out = append(out, fmt.Sprintf("edits: %d", len(edits)))
		}
		return out
	case "NotebookEdit":
		path, _ := input["notebook_path"].(string)
		return []string{"path: " + path}
	case "Read":
		path, _ := input["file_path"].(string)
		out := []string{"path: " + path}
		if offset, ok := input["offset"].(float64); ok {
			out = append(out, fmt.Sprintf("offset: %d", int(offset)))
		}
		if limit, ok := input["limit"].(float64); ok {
			out = append(out, fmt.Sprintf("limit: %d", int(limit)))
		}
		return out
	case "Glob":
		if pat, _ := input["pattern"].(string); pat != "" {
			return []string{"pattern: " + pat}
		}
	case "Grep":
		if pat, _ := input["pattern"].(string); pat != "" {
			out := []string{"pattern: " + pat}
			if path, _ := input["path"].(string); path != "" {
				out = append(out, "path: "+path)
			}
			return out
		}
	}

	b, _ := json.MarshalIndent(input, "", "  ")
	return strings.Split(string(b), "\n")
}

func formatClaudeUser(ev map[string]any) []string {
	msg, ok := ev["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return nil
	}
	var lines []string
	for _, c := range content {
		block, ok := c.(map[string]any)
		if !ok {
			continue
		}
		bt, _ := block["type"].(string)
		if bt != "tool_result" {
			continue
		}
		id, _ := block["tool_use_id"].(string)
		status := "ok"
		if isErr, _ := block["is_error"].(bool); isErr {
			status = "err"
		}
		header := "result " + status
		if id != "" {
			header += " (" + shortenID(id) + ")"
		}
		lines = append(lines, header)
		lines = append(lines, extractToolResultText(block["content"])...)
		lines = append(lines, "")
	}
	return lines
}

func extractToolResultText(raw any) []string {
	var out []string
	switch v := raw.(type) {
	case string:
		out = append(out, truncateLines(v, claudeMaxToolOutputLines)...)
	case []any:
		for _, part := range v {
			pm, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if pt, _ := pm["type"].(string); pt == "text" {
				if txt, _ := pm["text"].(string); txt != "" {
					out = append(out, truncateLines(txt, claudeMaxToolOutputLines)...)
				}
			}
		}
	}
	return out
}

func formatClaudeResult(ev map[string]any) []string {
	lines := []string{"--------", "result"}
	if v, _ := ev["subtype"].(string); v != "" {
		lines = append(lines, "subtype: "+v)
	}
	if v, ok := ev["duration_ms"].(float64); ok && v > 0 {
		lines = append(lines, fmt.Sprintf("duration: %.0f ms", v))
	}
	if v, ok := ev["total_cost_usd"].(float64); ok && v > 0 {
		lines = append(lines, fmt.Sprintf("cost: $%.4f", v))
	}
	if usage, ok := ev["usage"].(map[string]any); ok {
		in, _ := usage["input_tokens"].(float64)
		out, _ := usage["output_tokens"].(float64)
		cacheRead, _ := usage["cache_read_input_tokens"].(float64)
		cacheCreate, _ := usage["cache_creation_input_tokens"].(float64)
		lines = append(lines, fmt.Sprintf("tokens: in=%.0f out=%.0f cache_read=%.0f cache_create=%.0f",
			in, out, cacheRead, cacheCreate))
	}
	if v, _ := ev["result"].(string); v != "" {
		lines = append(lines, "")
		lines = append(lines, strings.Split(strings.TrimRight(v, "\n"), "\n")...)
	}
	return lines
}

// truncateLines returns at most n lines; if truncated, appends an elision
// line so a reader knows output was cut.
func truncateLines(s string, n int) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return lines
	}
	return append(lines[:n], fmt.Sprintf("... (%d more lines)", len(lines)-n))
}

// shortenID compresses a tool_use_id like "toolu_01ABcd..." to "toolu_01AB"
// for header lines. The full id stays in the JSONL stream.
func shortenID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}
