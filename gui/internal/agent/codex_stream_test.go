package agent

import (
	"bytes"
	"strings"
	"testing"
)

// stubWriter captures every Write call as a separate entry so tests can
// assert on per-line framing the way runExternal feeds us upstream.
type stubWriter struct {
	bytes.Buffer
	writes int
}

func (s *stubWriter) Write(p []byte) (int, error) {
	s.writes++
	return s.Buffer.Write(p)
}

func writeCodexLine(t *testing.T, w *codexJsonTranslator, line string) {
	t.Helper()
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

func TestCodexJsonTranslator_FiresOnSessionIDOnceTopLevel(t *testing.T) {
	var (
		out  stubWriter
		seen []string
	)
	tr := newCodexJsonTranslator(&out, func(sid string) { seen = append(seen, sid) }).(*codexJsonTranslator)

	writeCodexLine(t, tr, `{"type":"session_meta","session_id":"11111111-2222-4333-8444-555555555555","model":"gpt-5"}`)
	// Same session id again: must NOT re-fire.
	writeCodexLine(t, tr, `{"type":"agent_message","session_id":"11111111-2222-4333-8444-555555555555","payload":{"text":"hi"}}`)

	if len(seen) != 1 {
		t.Fatalf("OnSessionID fired %d times, want 1: %v", len(seen), seen)
	}
	if seen[0] != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("session id mismatch: got %q", seen[0])
	}
}

func TestCodexJsonTranslator_FiresOnSessionIDFromPayload(t *testing.T) {
	var (
		out  stubWriter
		seen []string
	)
	tr := newCodexJsonTranslator(&out, func(sid string) { seen = append(seen, sid) }).(*codexJsonTranslator)

	writeCodexLine(t, tr, `{"type":"session.created","payload":{"session_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","model":"gpt-5"}}`)
	if len(seen) != 1 || seen[0] != "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee" {
		t.Fatalf("payload session id capture failed: %v", seen)
	}
}

func TestCodexJsonTranslator_IgnoresNonUUIDSessionID(t *testing.T) {
	var seen []string
	tr := newCodexJsonTranslator(&stubWriter{}, func(sid string) { seen = append(seen, sid) }).(*codexJsonTranslator)

	// A function_call.id that happens to be in a "session_id" field but
	// isn't UUID-shaped must NOT trigger session capture.
	writeCodexLine(t, tr, `{"type":"function_call","session_id":"call_abc123","payload":{"name":"shell"}}`)
	if len(seen) != 0 {
		t.Fatalf("OnSessionID fired for non-UUID value: %v", seen)
	}
}

func TestCodexJsonTranslator_NilCallbackOK(t *testing.T) {
	out := &stubWriter{}
	tr := newCodexJsonTranslator(out, nil).(*codexJsonTranslator)
	// Must not panic when callback is nil and a session id appears.
	writeCodexLine(t, tr, `{"type":"session_meta","session_id":"11111111-2222-4333-8444-555555555555"}`)
}

func TestCodexJsonTranslator_RendersAgentMessage(t *testing.T) {
	out := &stubWriter{}
	tr := newCodexJsonTranslator(out, nil).(*codexJsonTranslator)
	writeCodexLine(t, tr, `{"type":"agent_message","payload":{"text":"hello world"}}`)

	got := out.String()
	if !strings.Contains(got, "codex\n") {
		t.Errorf("missing codex header in output:\n%s", got)
	}
	if !strings.Contains(got, "hello world\n") {
		t.Errorf("missing message text in output:\n%s", got)
	}
}

func TestCodexJsonTranslator_PassesThroughNonJSON(t *testing.T) {
	out := &stubWriter{}
	tr := newCodexJsonTranslator(out, nil).(*codexJsonTranslator)
	// Plain text — codex may emit a banner before the structured stream.
	writeCodexLine(t, tr, "codex-cli 0.130.0 starting...")

	if !strings.Contains(out.String(), "codex-cli 0.130.0") {
		t.Errorf("non-JSON passthrough failed:\n%s", out.String())
	}
}

func TestCodexJsonTranslator_UnknownTypeBreadcrumb(t *testing.T) {
	out := &stubWriter{}
	tr := newCodexJsonTranslator(out, nil).(*codexJsonTranslator)
	writeCodexLine(t, tr, `{"type":"future_event_type_we_dont_know","payload":{"x":1}}`)

	got := out.String()
	if !strings.Contains(got, "[future_event_type_we_dont_know]") {
		t.Errorf("unknown type breadcrumb missing:\n%s", got)
	}
}

// TestCodexJsonTranslator_RealCodex0130SchemaCapturesThreadID locks
// the schema verified against codex-cli 0.130.0 (`codex exec --json
// "say hi"` output, captured 2026-05-19). The thread.started event
// carries thread_id (NOT session_id) — the translator must extract it
// so the resume flow can feed it back via `codex exec resume <uuid>`.
func TestCodexJsonTranslator_RealCodex0130SchemaCapturesThreadID(t *testing.T) {
	var seen []string
	out := &stubWriter{}
	tr := newCodexJsonTranslator(out, func(sid string) { seen = append(seen, sid) }).(*codexJsonTranslator)

	// Real lines from codex-cli 0.130.0.
	fixture := []string{
		`{"type":"thread.started","thread_id":"019e3f96-8149-7ef0-a669-75570bca53e8"}`,
		`{"type":"turn.started"}`,
		`{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":27783,"cached_input_tokens":3456,"output_tokens":48,"reasoning_output_tokens":41}}`,
	}
	for _, line := range fixture {
		writeCodexLine(t, tr, line)
	}

	if len(seen) != 1 {
		t.Fatalf("OnSessionID fired %d times, want 1: %v", len(seen), seen)
	}
	if seen[0] != "019e3f96-8149-7ef0-a669-75570bca53e8" {
		t.Fatalf("captured thread id = %q, want the canonical UUID from the fixture", seen[0])
	}

	rendered := out.String()
	if !strings.Contains(rendered, "session id: 019e3f96-8149-7ef0-a669-75570bca53e8") {
		t.Errorf("thread.started should render the session id line:\n%s", rendered)
	}
	if !strings.Contains(rendered, "done") {
		t.Errorf("agent_message item text should reach the log:\n%s", rendered)
	}
}

func TestCodexJsonTranslator_OnSessionIDWritesEmptyLineNotPanic(t *testing.T) {
	out := &stubWriter{}
	tr := newCodexJsonTranslator(out, nil).(*codexJsonTranslator)
	// Empty stdin: zero bytes must not cause an error or downstream
	// write.
	n, err := tr.Write(nil)
	if err != nil || n != 0 {
		t.Fatalf("empty write: n=%d err=%v", n, err)
	}
	if out.writes != 0 {
		t.Fatalf("downstream should not receive writes for empty input; writes=%d", out.writes)
	}
}
