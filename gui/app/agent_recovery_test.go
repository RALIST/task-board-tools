package app

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// recoveryFixture builds a real `tb` board with one task whose .md
// says AgentStatus=running. The caller supplies JSONL events that
// should be present at .agent-state/<ID>.jsonl. Returns the wired
// services + boardDir for further mutation.
func recoveryFixture(t *testing.T, id, agentField string, events []agent.Event) (*RecoveryService, *BoardService, string, *cli.Client) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only board flock")
	}
	tbBinary := buildTbForIntegration(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}

	taskBody := strings.Replace(sampleTaskBody, "**Branch:** —",
		"**Branch:** —\n**Agent:** "+agentField+"\n**AgentStatus:** running", 1)
	if err := os.WriteFile(filepath.Join(boardDir, "backlog", id+".md"), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("task md: %v", err)
	}

	// Write the JSONL with the supplied events.
	for _, ev := range events {
		if err := agent.AppendEvent(boardDir, id, ev); err != nil {
			t.Fatalf("append jsonl: %v", err)
		}
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	em := newRecordingEmitter()
	svc := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})

	rec := NewRecoveryService(board, svc,
		// Default: PID is dead (recover).
		func(pid int, expected string) bool { return false },
		slog.Default(),
	)
	return rec, board, boardDir, c
}

func TestRecoverStale_NoFinished_DeadPID_MarksFailed(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_aaaa", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_aaaa", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 99999},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	// JSONL now has a finished{failed} line.
	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(final) < 3 {
		t.Fatalf("expected synthetic finished; have %d events", len(final))
	}
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusFailed {
		t.Errorf("last event: %+v", last)
	}
	if last.Reason != "stale after restart" {
		t.Errorf("reason: %q", last.Reason)
	}

	// AgentStatus on disk now reads failed.
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** failed") {
		t.Fatalf("AgentStatus not failed:\n%s", out)
	}
}

func TestRecoverStale_LivePID_SkipsRecovery(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_bbbb", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_bbbb", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 12345},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	// Override the liveFn to "alive".
	rec.liveFn = func(pid int, expected string) bool { return true }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	for _, ev := range final {
		if ev.Event == agent.EvFinished {
			t.Fatalf("live PID should not synthesise finished: %+v", final)
		}
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** running") {
		t.Fatalf("AgentStatus changed despite live PID:\n%s", out)
	}
}

// TB-61 carve-out: latest event is finished{cancelled} → reconcile to
// cancelled, NOT failed; JSONL must not get an extra failed line.
func TestRecoverStale_CancelledCarveOut(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_cccc", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_cccc", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 12345},
		{TS: "2026-05-13T10:00:02Z", RunID: "r_cccc", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusCancelled, ExitCode: -1, Reason: "user cancelled"},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	// PID is dead — but JSONL says cancelled, so the carve-out triggers.
	rec.liveFn = func(pid int, expected string) bool { return false }

	before := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	after := readJSONL(t, agent.StatePath(boardDir, "TB-1"))

	if len(after) != len(before) {
		t.Errorf("expected no new JSONL line (cancelled carve-out); before=%d, after=%d",
			len(before), len(after))
	}
	last := after[len(after)-1]
	if last.Status != agent.StatusCancelled {
		t.Errorf("last status: %s, want cancelled", last.Status)
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled:\n%s", out)
	}
}

// JSONL finished naturally with success/failed but the .md AgentStatus
// is stale (write failed). Recovery should just sync the .md.
func TestRecoverStale_FinishedButMdStale_Syncs(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_dddd", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_dddd", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 12345},
		{TS: "2026-05-13T10:00:02Z", RunID: "r_dddd", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusSuccess, ExitCode: 0},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.liveFn = func(pid int, expected string) bool { return false }

	before := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	after := readJSONL(t, agent.StatePath(boardDir, "TB-1"))

	if len(after) != len(before) {
		t.Errorf("finished already in JSONL — no new line expected; before=%d after=%d",
			len(before), len(after))
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** success") {
		t.Fatalf("AgentStatus not synced:\n%s", out)
	}
}

// When the queued event carries `agent` and started does NOT (old M4
// schema), the recovery reader still finds the expected agent name.
func TestRecoverStale_OldStartedSchema_FallsBackToQueuedAgent(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_eeee", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_eeee", TaskID: "TB-1", Event: agent.EvStarted, PID: 99999}, // no Agent
	}
	rec, _, boardDir, _ := recoveryFixture(t, "TB-1", "claude", events)
	called := false
	rec.liveFn = func(pid int, expected string) bool {
		called = true
		if expected != "claude" {
			t.Errorf("expectedAgent=%q, want claude", expected)
		}
		return false
	}
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if !called {
		t.Errorf("liveFn never invoked")
	}
}

func readJSONL(t *testing.T, path string) []agent.Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var out []agent.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev agent.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("decode %q: %v", sc.Text(), err)
		}
		out = append(out, ev)
	}
	return out
}
