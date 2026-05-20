package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
)

// fakeAPISecret is a never-real credential pattern used to assert that
// redaction reaches every sink without ever committing or paging a real
// token. The form mimics an OPENAI_API_KEY assignment that agents commonly
// echo during smoke runs.
const fakeAPISecret = "sk-fake-NOT-A-REAL-KEY-1234567890"

func TestRunAgent_RedactsSecretsAcrossAllSinks(t *testing.T) {
	leaking := []string{
		"normal output before secret",
		"OPENAI_API_KEY=" + fakeAPISecret,
		"Authorization: Bearer " + fakeAPISecret,
		"trailing log line",
	}
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: leaking,
		stderrLines: []string{"warn password=" + fakeAPISecret},
		exitCode:    0,
	}

	// Build the service with our own captured emitter so we can inspect the
	// Wails payloads after the run completes.
	var em *recordingEmitter
	svc, boardDir := realTbBoardForRunWithOptions(t, "claude", stub, func(opts *AgentServiceOptions) {
		em = newRecordingEmitter()
		opts.Emitter = em
	})

	runID, err := svc.RunAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	// 1. The on-disk log file must not contain the raw secret.
	logBytes, err := os.ReadFile(agent.LogPath(boardDir, "TB-1", runID))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	logText := string(logBytes)
	if strings.Contains(logText, fakeAPISecret) {
		t.Fatalf("log file leaked raw secret:\n%s", logText)
	}
	if !strings.Contains(logText, "[REDACTED]") {
		t.Fatalf("log file missing [REDACTED] marker:\n%s", logText)
	}
	// Non-secret context is preserved.
	if !strings.Contains(logText, "normal output before secret") {
		t.Fatalf("log file dropped non-secret context:\n%s", logText)
	}

	// 2. The JSONL state must not contain the raw secret in any event.line.
	events := readEvents(t, boardDir, "TB-1")
	for _, ev := range events {
		if strings.Contains(ev.Line, fakeAPISecret) {
			t.Fatalf("jsonl event %+v leaked raw secret", ev)
		}
	}
	// Among the stdout/stderr events there must be at least one [REDACTED].
	sawRedactedEvent := false
	for _, ev := range events {
		if strings.Contains(ev.Line, "[REDACTED]") {
			sawRedactedEvent = true
			break
		}
	}
	if !sawRedactedEvent {
		t.Fatalf("no jsonl event captured the redacted marker; events=%+v", events)
	}

	// 3. The Wails events emitted to the frontend must not carry the secret.
	if em == nil {
		t.Fatalf("emitter was not captured; configure callback did not run")
	}
	sawRedactedEmit := false
	emitted := em.snapshot()
	for _, e := range emitted {
		if e.Name != "agent:run-log" {
			continue
		}
		for _, payload := range e.Payload {
			m, ok := payload.(map[string]any)
			if !ok {
				continue
			}
			line, _ := m["line"].(string)
			if strings.Contains(line, fakeAPISecret) {
				t.Fatalf("agent:run-log emit leaked raw secret: %+v", m)
			}
			if strings.Contains(line, "[REDACTED]") {
				sawRedactedEmit = true
			}
		}
	}
	if !sawRedactedEmit {
		t.Fatalf("no agent:run-log emit carried [REDACTED]; events=%+v", emitted)
	}

	// 4. GetRunLog readback must serve the redacted text (since the on-disk
	//    bytes are already redacted at the sink, readback inherits that).
	got, err := svc.GetRunLog(context.Background(), "TB-1", runID)
	if err != nil {
		t.Fatalf("GetRunLog: %v", err)
	}
	if strings.Contains(got, fakeAPISecret) {
		t.Fatalf("GetRunLog leaked raw secret:\n%s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("GetRunLog missing [REDACTED] marker:\n%s", got)
	}
}
