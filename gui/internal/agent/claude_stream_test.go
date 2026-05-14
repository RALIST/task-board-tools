package agent

import (
	"bytes"
	"strings"
	"testing"
)

// writeLine feeds one stream-json event into the translator the same way
// runExternal's streamLines does: one Write per line, line ends with '\n'.
func writeLine(t *testing.T, w *claudeTranslator, line string) {
	t.Helper()
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatalf("translator.Write: %v", err)
	}
}

func TestClaudeTranslator_Init(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"system","subtype":"init","cwd":"/work","model":"claude-opus-4-7","permissionMode":"acceptEdits","session_id":"s_abc","tools":["Bash","Edit"]}`)

	out := buf.String()
	for _, want := range []string{
		"Claude Code",
		"workdir: /work",
		"model: claude-opus-4-7",
		"permission mode: acceptEdits",
		"session id: s_abc",
		"tools: Bash, Edit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("init output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestClaudeTranslator_AssistantText(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"assistant","message":{"content":[{"type":"text","text":"hello world\nline two"}]}}`)

	out := buf.String()
	if !strings.Contains(out, "claude\nhello world\nline two\n") {
		t.Errorf("text block not rendered as Codex-style:\n%s", out)
	}
}

func TestClaudeTranslator_BashToolUse(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_01ABCDEFGHIJ","name":"Bash","input":{"command":"tb show TB-1"}}]}}`)

	out := buf.String()
	if !strings.Contains(out, "exec Bash") {
		t.Errorf("missing exec header:\n%s", out)
	}
	if !strings.Contains(out, "$ tb show TB-1") {
		t.Errorf("missing $ command line:\n%s", out)
	}
}

func TestClaudeTranslator_EditToolUse(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_01EDIT","name":"Edit","input":{"file_path":"/work/a.go","old_string":"foo","new_string":"bar"}}]}}`)

	out := buf.String()
	for _, want := range []string{
		"exec Edit",
		"path: /work/a.go",
		"--- old ---",
		"foo",
		"+++ new +++",
		"bar",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("edit output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestClaudeTranslator_WriteToolUse(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_01WRITE","name":"Write","input":{"file_path":"/work/new.go","content":"package main\nfunc main(){}\n"}}]}}`)

	out := buf.String()
	for _, want := range []string{
		"exec Write",
		"path: /work/new.go",
		"+++ content +++",
		"package main",
		"func main(){}",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("write output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestClaudeTranslator_ToolResultStringContent(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_01ABCDEFGHIJ","content":"hello\nworld","is_error":false}]}}`)

	out := buf.String()
	if !strings.Contains(out, "result ok") {
		t.Errorf("missing 'result ok' header:\n%s", out)
	}
	if !strings.Contains(out, "hello\nworld") {
		t.Errorf("missing string content:\n%s", out)
	}
}

func TestClaudeTranslator_ToolResultArrayContent(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_01X","is_error":true,"content":[{"type":"text","text":"oops"}]}]}}`)

	out := buf.String()
	if !strings.Contains(out, "result err") {
		t.Errorf("missing 'result err' header:\n%s", out)
	}
	if !strings.Contains(out, "oops") {
		t.Errorf("missing array text content:\n%s", out)
	}
}

func TestClaudeTranslator_Result(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"result","subtype":"success","duration_ms":12345,"total_cost_usd":0.0123,"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":30,"cache_creation_input_tokens":40},"result":"all done"}`)

	out := buf.String()
	for _, want := range []string{
		"result",
		"subtype: success",
		"duration: 12345 ms",
		"cost: $0.0123",
		"tokens: in=10 out=20 cache_read=30 cache_create=40",
		"all done",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("result output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestClaudeTranslator_NonJSONPassthrough(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	noise := []byte("plain stderr-like line\n")
	if _, err := tr.Write(noise); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := buf.String(); got != "plain stderr-like line\n" {
		t.Errorf("non-JSON not passed through verbatim: got %q", got)
	}
}

func TestClaudeTranslator_UnknownEventDropped(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	writeLine(t, tr, `{"type":"future_event","payload":"whatever"}`)
	if got := buf.String(); got != "" {
		t.Errorf("unknown JSON event should be dropped, got: %q", got)
	}
}

func TestClaudeTranslator_OneWritePerLine(t *testing.T) {
	// Each output line goes through one Write so lineSink can record it
	// as one JSONL stdout event. Use a counting writer to assert.
	var counter countingWriter
	tr := &claudeTranslator{out: &counter}
	writeLine(t, tr, `{"type":"assistant","message":{"content":[{"type":"text","text":"a\nb\nc"}]}}`)

	if counter.writes < 3 {
		t.Errorf("expected >=3 writes (one per output line), got %d", counter.writes)
	}
}

type countingWriter struct {
	writes int
	buf    bytes.Buffer
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	return c.buf.Write(p)
}

func TestClaudeTranslator_LongToolResultTruncated(t *testing.T) {
	var buf bytes.Buffer
	tr := &claudeTranslator{out: &buf}
	var sb strings.Builder
	for i := 0; i < claudeMaxToolOutputLines+50; i++ {
		sb.WriteString("line\n")
	}
	payload := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_01ABCDEFGHIJ","content":` + jsonString(sb.String()) + `}]}}`
	writeLine(t, tr, payload)

	out := buf.String()
	if !strings.Contains(out, "more lines)") {
		t.Errorf("expected truncation elision, got:\n%s", out)
	}
}

// jsonString returns a JSON-encoded string literal for embedding in raw
// JSON fixtures.
func jsonString(s string) string {
	b, _ := jsonMarshalString(s)
	return string(b)
}

func jsonMarshalString(s string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
	return buf.Bytes(), nil
}
