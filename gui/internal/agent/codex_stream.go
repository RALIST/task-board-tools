package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// codexMaxToolOutputLines caps how many lines of a function_call_output
// we render. The full JSON form is still preserved by the upstream JSONL
// state writer; this limit just keeps the readable log from being
// dominated by one runaway command.
const codexMaxToolOutputLines = 200

// uuidRE matches a canonical UUIDv4-shaped string. We only fire
// OnSessionID for strings of this exact shape so a sub-message id (a
// `function_call.id` for instance) can't be mistaken for the run's
// session id.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// codexJsonTranslator wraps an io.Writer and converts each line of
// `codex exec --json` stdout into a human-readable block on the
// underlying writer. Lines that don't parse as JSON, or that don't
// match a known event shape, pass through unchanged so the runner stays
// resilient to schema drift.
//
// It also fires the optional onSessionID callback the FIRST time it
// observes a UUIDv4-shaped `session_id` field at either the top level
// or one level inside `payload`. Subsequent occurrences are ignored —
// the callback receives the session id exactly once.
//
// Contract mirrors claudeTranslator (see claude_stream.go):
//   - One Write call carries one full --json event (runExternal does
//     line-based scanning, so the upstream guarantees one line per
//     Write).
//   - Output is rendered as one or more newline-terminated lines per
//     event.
//   - Returns len(p) on the input even when the translated output is
//     longer. Keeps io.Writer accounting honest from runExternal's POV.
type codexJsonTranslator struct {
	out           io.Writer
	onSessionID   func(string)
	sessionFired  bool
}

func newCodexJsonTranslator(out io.Writer, onSessionID func(string)) io.Writer {
	return &codexJsonTranslator{out: out, onSessionID: onSessionID}
}

func (t *codexJsonTranslator) Write(p []byte) (int, error) {
	trimmed := bytes.TrimRight(p, "\r\n")
	if len(trimmed) == 0 {
		return len(p), nil
	}

	var ev map[string]any
	if err := json.Unmarshal(trimmed, &ev); err != nil {
		// Not JSON — codex --json may emit a leading banner or progress
		// line before the structured stream starts. Pass it through so
		// it still reaches the log.
		return t.passthrough(p)
	}

	t.maybeFireSessionID(ev)

	lines := formatCodexEvent(ev)
	if len(lines) == 0 {
		return len(p), nil
	}
	return t.writeLines(p, lines)
}

func (t *codexJsonTranslator) maybeFireSessionID(ev map[string]any) {
	if t.sessionFired || t.onSessionID == nil {
		return
	}
	// Top-level session_id first; some shapes nest it under payload.
	if sid, ok := uuidStringField(ev, "session_id"); ok {
		t.sessionFired = true
		t.onSessionID(sid)
		return
	}
	if payload, ok := ev["payload"].(map[string]any); ok {
		if sid, ok := uuidStringField(payload, "session_id"); ok {
			t.sessionFired = true
			t.onSessionID(sid)
			return
		}
	}
}

// uuidStringField returns the value at `key` IF it is a non-empty UUID-
// shaped string. Anything else (numeric id, short opaque id, missing
// key) returns ok=false.
func uuidStringField(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	if !ok || v == "" {
		return "", false
	}
	if !uuidRE.MatchString(v) {
		return "", false
	}
	return v, true
}

func (t *codexJsonTranslator) passthrough(p []byte) (int, error) {
	if _, err := t.out.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// writeLines emits each rendered line followed by '\n' as a separate
// Write to the underlying sink. Per-line writes keep the lineSink JSONL
// events one-line-per-event.
func (t *codexJsonTranslator) writeLines(orig []byte, lines []string) (int, error) {
	for _, line := range lines {
		if _, err := t.out.Write([]byte(line + "\n")); err != nil {
			return 0, err
		}
	}
	return len(orig), nil
}

// formatCodexEvent renders a parsed codex --json event into one or more
// human-readable lines. The mapping is best-effort and tolerant of
// schema drift: unknown event types fall back to a `[type] {json-ish}`
// one-liner so the log still carries a breadcrumb.
func formatCodexEvent(ev map[string]any) []string {
	typ, _ := ev["type"].(string)
	if typ == "" {
		return nil
	}
	payload, _ := ev["payload"].(map[string]any)

	switch typ {
	case "session_meta", "session.created":
		lines := []string{"codex", "--------"}
		if v, _ := stringField(ev, "model"); v != "" {
			lines = append(lines, "model: "+v)
		}
		if v, _ := stringField(payload, "model"); v != "" && !contains(lines, "model: "+v) {
			lines = append(lines, "model: "+v)
		}
		if v, _ := stringField(ev, "session_id"); v != "" {
			lines = append(lines, "session id: "+v)
		} else if v, _ := stringField(payload, "session_id"); v != "" {
			lines = append(lines, "session id: "+v)
		}
		if v, _ := stringField(ev, "cwd"); v != "" {
			lines = append(lines, "workdir: "+v)
		}
		lines = append(lines, "--------")
		return lines

	case "agent_message", "agent.message", "assistant_message":
		text := codexExtractText(ev, payload)
		if text == "" {
			return nil
		}
		out := []string{"codex"}
		out = append(out, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
		out = append(out, "")
		return out

	case "agent_message_delta", "agent.delta":
		// Streaming deltas — render in-place; one delta per line keeps
		// the log honest with what the CLI emitted.
		text := codexExtractText(ev, payload)
		if text == "" {
			return nil
		}
		return strings.Split(strings.TrimRight(text, "\n"), "\n")

	case "agent_reasoning", "agent.reasoning":
		text := codexExtractText(ev, payload)
		if text == "" {
			return nil
		}
		out := []string{"reasoning"}
		out = append(out, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
		out = append(out, "")
		return out

	case "function_call", "tool_call", "exec_command_begin":
		name, _ := stringField(payload, "name")
		if name == "" {
			name, _ = stringField(ev, "name")
		}
		header := "exec " + name
		if id, _ := stringField(payload, "call_id"); id != "" {
			header += " (" + shortenID(id) + ")"
		}
		out := []string{header}
		if cmd, _ := stringField(payload, "command"); cmd != "" {
			out = append(out, "$ "+cmd)
		} else if args, _ := stringField(payload, "arguments"); args != "" {
			out = append(out, args)
		}
		out = append(out, "")
		return out

	case "function_call_output", "tool_result", "exec_command_end":
		status := "ok"
		if isErr, _ := payload["success"].(bool); !isErr {
			if _, present := payload["success"]; present {
				status = "err"
			}
		}
		if code, _ := payload["exit_code"].(float64); code != 0 {
			status = fmt.Sprintf("exit %d", int(code))
		}
		header := "result " + status
		if id, _ := stringField(payload, "call_id"); id != "" {
			header += " (" + shortenID(id) + ")"
		}
		out := []string{header}
		if outText, _ := stringField(payload, "output"); outText != "" {
			out = append(out, truncateLines(outText, codexMaxToolOutputLines)...)
		}
		out = append(out, "")
		return out

	case "token_count", "token.count":
		if payload == nil {
			return nil
		}
		in, _ := payload["input_tokens"].(float64)
		outTok, _ := payload["output_tokens"].(float64)
		return []string{fmt.Sprintf("tokens: in=%.0f out=%.0f", in, outTok)}

	case "exit", "session.completed":
		out := []string{"--------", "result"}
		if v, _ := stringField(payload, "reason"); v != "" {
			out = append(out, "reason: "+v)
		}
		if code, ok := payload["exit_code"].(float64); ok {
			out = append(out, fmt.Sprintf("exit_code: %d", int(code)))
		}
		return out
	}

	// Unknown event type — emit a one-line breadcrumb so the log still
	// records that something happened, and pass the JSON through.
	b, _ := json.Marshal(ev)
	return []string{"[" + typ + "] " + string(b)}
}

// codexExtractText pulls a human-readable text body out of an event.
// Codex --json places the text at different paths depending on the
// event type; check the common ones in order of likelihood.
func codexExtractText(ev, payload map[string]any) string {
	if v, _ := stringField(payload, "text"); v != "" {
		return v
	}
	if v, _ := stringField(payload, "message"); v != "" {
		return v
	}
	if v, _ := stringField(payload, "content"); v != "" {
		return v
	}
	if v, _ := stringField(ev, "text"); v != "" {
		return v
	}
	if v, _ := stringField(ev, "message"); v != "" {
		return v
	}
	return ""
}

func stringField(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key].(string)
	return v, ok && v != ""
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
