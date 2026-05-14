package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
	return recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          id,
		agentField:  agentField,
		agentStatus: "running",
		form:        "file",
		events:      events,
	}})
}

type recoveryTaskFixture struct {
	id          string
	agentField  string
	agentStatus string
	form        string
	events      []agent.Event
}

func recoveryFixtureWithTasks(t *testing.T, tasks []recoveryTaskFixture) (*RecoveryService, *BoardService, string, *cli.Client) {
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
	nextID := len(tasks) + 1
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte(fmt.Sprintf("%d\n", nextID)), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}

	for _, task := range tasks {
		status := task.agentStatus
		if status == "" {
			status = "running"
		}
		form := task.form
		if form == "" {
			form = "file"
		}
		taskBody := recoveryTaskBody(task.id, task.agentField, status)
		switch form {
		case "folder":
			taskDir := filepath.Join(boardDir, "backlog", task.id)
			if err := os.MkdirAll(taskDir, 0o755); err != nil {
				t.Fatalf("task dir %s: %v", task.id, err)
			}
			if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte(taskBody), 0o644); err != nil {
				t.Fatalf("folder task md %s: %v", task.id, err)
			}
		case "file":
			if err := os.WriteFile(filepath.Join(boardDir, "backlog", task.id+".md"), []byte(taskBody), 0o644); err != nil {
				t.Fatalf("file task md %s: %v", task.id, err)
			}
		default:
			t.Fatalf("unknown task form %q", form)
		}

		for _, ev := range task.events {
			if err := agent.AppendEvent(boardDir, task.id, ev); err != nil {
				t.Fatalf("append jsonl %s: %v", task.id, err)
			}
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

func recoveryTaskBody(id, agentField, agentStatus string) string {
	body := strings.Replace(sampleTaskBody, "# TB-1: Sample title", "# "+id+": Sample title", 1)
	return strings.Replace(body, "**Branch:** —",
		"**Branch:** —\n**Agent:** "+agentField+"\n**AgentStatus:** "+agentStatus, 1)
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

func TestRecoverStale_MixedFileAndFolderTasksMarkFailedInOwnLayouts(t *testing.T) {
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{
		{
			id:          "TB-1",
			agentField:  "claude",
			agentStatus: "running",
			form:        "file",
			events: []agent.Event{
				{TS: "2026-05-14T10:00:00Z", RunID: "r_file", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
				{TS: "2026-05-14T10:00:01Z", RunID: "r_file", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 1001},
			},
		},
		{
			id:          "TB-2",
			agentField:  "codex",
			agentStatus: "running",
			form:        "folder",
			events: []agent.Event{
				{TS: "2026-05-14T10:01:00Z", RunID: "r_folder", TaskID: "TB-2", Event: agent.EvQueued, Agent: "codex"},
				{TS: "2026-05-14T10:01:01Z", RunID: "r_folder", TaskID: "TB-2", Event: agent.EvStarted, Agent: "codex", PID: 2002},
			},
		},
	})
	seen := map[int]string{}
	rec.liveFn = func(pid int, expected string) bool {
		seen[pid] = expected
		return false
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if seen[1001] != "claude" || seen[2002] != "codex" {
		t.Fatalf("liveFn calls: got %+v, want pid 1001=claude and 2002=codex", seen)
	}

	fileState := filepath.Join(boardDir, ".agent-state", "TB-1.jsonl")
	folderState := filepath.Join(boardDir, "backlog", "TB-2", ".agent-state.jsonl")
	if agent.StatePath(boardDir, "TB-1") != fileState {
		t.Fatalf("file StatePath = %s, want %s", agent.StatePath(boardDir, "TB-1"), fileState)
	}
	if agent.StatePath(boardDir, "TB-2") != folderState {
		t.Fatalf("folder StatePath = %s, want %s", agent.StatePath(boardDir, "TB-2"), folderState)
	}

	for _, tc := range []struct {
		id   string
		path string
	}{
		{id: "TB-1", path: fileState},
		{id: "TB-2", path: folderState},
	} {
		events := readJSONL(t, tc.path)
		last := events[len(events)-1]
		if last.Event != agent.EvFinished || last.Status != agent.StatusFailed || last.Reason != "stale after restart" {
			t.Fatalf("%s last event: %+v, want stale failed", tc.id, last)
		}
		out, err := c.Run(context.Background(), "show", tc.id)
		if err != nil {
			t.Fatalf("tb show %s: %v", tc.id, err)
		}
		if !strings.Contains(string(out), "**AgentStatus:** failed") {
			t.Fatalf("%s AgentStatus not failed:\n%s", tc.id, out)
		}
	}

	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-2.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("folder task should not create board-root state, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-logs", "TB-2")); !os.IsNotExist(err) {
		t.Fatalf("folder task should not create board-root logs, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-1", ".agent-state.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("file task should not create task-local state, err=%v", err)
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

func TestRecoverStale_FolderLivePID_SkipsRecovery(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_folderlive", TaskID: "TB-2", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_folderlive", TaskID: "TB-2", Event: agent.EvStarted, Agent: "claude", PID: 12345},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-2",
		agentField:  "claude",
		agentStatus: "running",
		form:        "folder",
		events:      events,
	}})
	rec.liveFn = func(pid int, expected string) bool { return true }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	final := readJSONL(t, agent.StatePath(boardDir, "TB-2"))
	for _, ev := range final {
		if ev.Event == agent.EvFinished {
			t.Fatalf("live PID should not synthesise finished: %+v", final)
		}
	}
	out, _ := c.Run(context.Background(), "show", "TB-2")
	if !strings.Contains(string(out), "**AgentStatus:** running") {
		t.Fatalf("AgentStatus changed despite live PID:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-2.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("folder live recovery should not touch board-root state, err=%v", err)
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

func TestRecoverStale_FolderCancelledCarveOut(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_foldercancel", TaskID: "TB-2", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_foldercancel", TaskID: "TB-2", Event: agent.EvStarted, Agent: "claude", PID: 12345},
		{TS: "2026-05-14T10:00:02Z", RunID: "r_foldercancel", TaskID: "TB-2", Event: agent.EvFinished, Status: agent.StatusCancelled, ExitCode: -1, Reason: "user cancelled"},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-2",
		agentField:  "claude",
		agentStatus: "running",
		form:        "folder",
		events:      events,
	}})
	rec.liveFn = func(pid int, expected string) bool { return false }

	before := readJSONL(t, agent.StatePath(boardDir, "TB-2"))
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	after := readJSONL(t, agent.StatePath(boardDir, "TB-2"))
	if len(after) != len(before) {
		t.Fatalf("expected no failed line for cancelled carve-out; before=%d after=%d", len(before), len(after))
	}
	last := after[len(after)-1]
	if last.Status != agent.StatusCancelled {
		t.Fatalf("last status: %s, want cancelled", last.Status)
	}
	out, _ := c.Run(context.Background(), "show", "TB-2")
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-2.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("folder cancel recovery should not touch board-root state, err=%v", err)
	}
}

func TestRecoverStale_DurableCancelledTaskIgnored(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_durablecancel", TaskID: "TB-2", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_durablecancel", TaskID: "TB-2", Event: agent.EvStarted, Agent: "claude", PID: 12345},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-2",
		agentField:  "claude",
		agentStatus: "cancelled",
		form:        "folder",
		events:      events,
	}})
	rec.liveFn = func(pid int, expected string) bool {
		t.Fatalf("cancelled task should not be probed for stale recovery")
		return false
	}

	before := readJSONL(t, agent.StatePath(boardDir, "TB-2"))
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	after := readJSONL(t, agent.StatePath(boardDir, "TB-2"))
	if len(after) != len(before) {
		t.Fatalf("durable cancelled task should not get synthetic failure; before=%d after=%d", len(before), len(after))
	}
	out, _ := c.Run(context.Background(), "show", "TB-2")
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus changed away from cancelled:\n%s", out)
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
