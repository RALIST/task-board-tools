package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

const recoveryStatusLost = agent.Status("lost")

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
	// statusDir is the on-disk bucket the task lives in. Empty defaults
	// to "backlog" — the prior fixture behavior. Set to "code-review",
	// "in-progress", "done", or "archive" to exercise other buckets.
	statusDir string
	events    []agent.Event
}

func recoveryFixtureWithTasks(t *testing.T, tasks []recoveryTaskFixture) (*RecoveryService, *BoardService, string, *cli.Client) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only board flock")
	}
	tbBinary := buildTbForIntegration(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "ready", "in-progress", "code-review", "done", "archive"} {
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
		bucket := task.statusDir
		if bucket == "" {
			bucket = "backlog"
		}
		taskBody := recoveryTaskBody(task.id, task.agentField, status)
		switch form {
		case "folder":
			taskDir := filepath.Join(boardDir, bucket, task.id)
			if err := os.MkdirAll(taskDir, 0o755); err != nil {
				t.Fatalf("task dir %s: %v", task.id, err)
			}
			if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte(taskBody), 0o644); err != nil {
				t.Fatalf("folder task md %s: %v", task.id, err)
			}
		case "file":
			if err := os.WriteFile(filepath.Join(boardDir, bucket, task.id+".md"), []byte(taskBody), 0o644); err != nil {
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
	t.Cleanup(rec.stopRecoveredRunMonitors)
	return rec, board, boardDir, c
}

func recoveryTaskBody(id, agentField, agentStatus string) string {
	body := strings.Replace(sampleTaskBody, "# TB-1: Sample title", "# "+id+": Sample title", 1)
	return strings.Replace(body, "**Branch:** —",
		"**Branch:** —\n**Agent:** "+agentField+"\n**AgentStatus:** "+agentStatus, 1)
}

func TestRecoverStale_NoFinished_DeadPID_MarksLost(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_aaaa", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_aaaa", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 99999},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	// JSONL now has a finished{lost} line.
	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(final) < 3 {
		t.Fatalf("expected synthetic finished; have %d events", len(final))
	}
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != recoveryStatusLost {
		t.Errorf("last event: %+v", last)
	}
	if last.Reason != "stale after restart" {
		t.Errorf("reason: %q", last.Reason)
	}

	// AgentStatus on disk now reads lost.
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("AgentStatus not lost:\n%s", out)
	}
}

func TestRecoverStale_MixedFileAndFolderTasksMarkLostInOwnLayouts(t *testing.T) {
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
		if last.Event != agent.EvFinished || last.Status != recoveryStatusLost || last.Reason != "stale after restart" {
			t.Fatalf("%s last event: %+v, want stale lost", tc.id, last)
		}
		out, err := c.Run(context.Background(), "show", tc.id)
		if err != nil {
			t.Fatalf("tb show %s: %v", tc.id, err)
		}
		if !strings.Contains(string(out), "**AgentStatus:** lost") {
			t.Fatalf("%s AgentStatus not lost:\n%s", tc.id, out)
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

func TestRecoverStale_LivePIDMonitorMarksFileAndFolderRunsLostWhenPIDExits(t *testing.T) {
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{
		{
			id:          "TB-1",
			agentField:  "claude",
			agentStatus: "running",
			form:        "file",
			events: []agent.Event{
				{TS: "2026-05-19T10:00:00Z", RunID: "r_file_orphan", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
				{TS: "2026-05-19T10:00:01Z", RunID: "r_file_orphan", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 1111},
			},
		},
		{
			id:          "TB-2",
			agentField:  "codex",
			agentStatus: "running",
			form:        "folder",
			events: []agent.Event{
				{TS: "2026-05-19T10:01:00Z", RunID: "r_folder_orphan", TaskID: "TB-2", Event: agent.EvQueued, Agent: "codex"},
				{TS: "2026-05-19T10:01:01Z", RunID: "r_folder_orphan", TaskID: "TB-2", Event: agent.EvStarted, Agent: "codex", PID: 2222},
			},
		},
	})
	rec.monitorPollInterval = 5 * time.Millisecond
	var alive atomic.Bool
	alive.Store(true)
	rec.liveFn = func(pid int, expected string) bool {
		switch pid {
		case 1111:
			if expected != "claude" {
				t.Errorf("pid 1111 expectedAgent=%q, want claude", expected)
			}
		case 2222:
			if expected != "codex" {
				t.Errorf("pid 2222 expectedAgent=%q, want codex", expected)
			}
		default:
			t.Errorf("unexpected pid probe %d", pid)
		}
		return alive.Load()
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	for _, id := range []string{"TB-1", "TB-2"} {
		for _, ev := range readJSONL(t, agent.StatePath(boardDir, id)) {
			if ev.Event == agent.EvFinished {
				t.Fatalf("%s live PID should not be reconciled immediately: %+v", id, ev)
			}
		}
		out, _ := c.Run(context.Background(), "show", id)
		if !strings.Contains(string(out), "**AgentStatus:** running") {
			t.Fatalf("%s should remain running while pid is alive:\n%s", id, out)
		}
	}

	alive.Store(false)
	for _, id := range []string{"TB-1", "TB-2"} {
		waitForCondition(t, time.Second, func() bool {
			events := readJSONL(t, agent.StatePath(boardDir, id))
			last := events[len(events)-1]
			if last.Event != agent.EvFinished || last.Status != recoveryStatusLost {
				return false
			}
			if last.Reason != "orphaned process exited after restart" {
				t.Fatalf("%s reason = %q, want orphaned process exited after restart", id, last.Reason)
			}
			out, err := c.Run(context.Background(), "show", id)
			return err == nil && strings.Contains(string(out), "**AgentStatus:** lost")
		})
	}
}

func TestRecoverStale_NoJSONL_MarksLostAndEmitsLost(t *testing.T) {
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", nil)

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(final) != 1 {
		t.Fatalf("expected one synthetic finished event, got %d: %+v", len(final), final)
	}
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != recoveryStatusLost {
		t.Fatalf("last event: %+v, want finished{lost}", last)
	}
	if last.Reason != "running without JSONL" {
		t.Fatalf("reason = %q, want running without JSONL", last.Reason)
	}

	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("AgentStatus not lost:\n%s", out)
	}

	em, ok := rec.agent.emitter.(*recordingEmitter)
	if !ok {
		t.Fatalf("unexpected emitter type %T", rec.agent.emitter)
	}
	var found map[string]any
	events := em.snapshot()
	for _, ev := range events {
		if ev.Name != "agent:run-finished" || len(ev.Payload) == 0 {
			continue
		}
		p, ok := ev.Payload[0].(map[string]any)
		if ok && p["task_id"] == "TB-1" {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatalf("missing agent:run-finished event; events=%+v", events)
	}
	if found["status"] != "lost" {
		t.Fatalf("finished status payload = %#v, want lost", found["status"])
	}
	if _, hasSession := found["session_id"]; hasSession {
		t.Fatalf("lost no-JSONL payload must not carry session_id: %+v", found)
	}
}

func TestRecoverStale_LivePIDMonitorIsIdempotentAcrossRepeatedRecovery(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_once", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_once", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 3333},
	}
	rec, _, boardDir, _ := recoveryFixture(t, "TB-1", "claude", events)
	rec.monitorPollInterval = 5 * time.Millisecond
	var alive atomic.Bool
	alive.Store(true)
	rec.liveFn = func(int, string) bool { return alive.Load() }

	for i := 0; i < 3; i++ {
		if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
			t.Fatalf("RecoverStale #%d: %v", i+1, err)
		}
	}
	alive.Store(false)

	waitForCondition(t, time.Second, func() bool {
		return countFinished(readJSONL(t, agent.StatePath(boardDir, "TB-1"))) == 1
	})
	eventsAfter := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if got := countFinished(eventsAfter); got != 1 {
		t.Fatalf("finished event count = %d, want exactly one; events=%+v", got, eventsAfter)
	}
}

func TestRecoverStale_LivePIDMonitorSyncsExistingTerminalWithoutAppending(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_terminal", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_terminal", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 4444},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.monitorPollInterval = 5 * time.Millisecond
	var alive atomic.Bool
	alive.Store(true)
	rec.liveFn = func(int, string) bool { return alive.Load() }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:       "2026-05-19T10:00:02Z",
		RunID:    "r_terminal",
		TaskID:   "TB-1",
		Event:    agent.EvFinished,
		Status:   agent.StatusSuccess,
		ExitCode: 0,
	}); err != nil {
		t.Fatalf("append terminal: %v", err)
	}
	alive.Store(false)

	waitForCondition(t, time.Second, func() bool {
		out, err := c.Run(context.Background(), "show", "TB-1")
		return err == nil && strings.Contains(string(out), "**AgentStatus:** success")
	})
	eventsAfter := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if got := countFinished(eventsAfter); got != 1 {
		t.Fatalf("finished event count = %d, want original terminal only; events=%+v", got, eventsAfter)
	}
}

func TestRecoverStale_LivePIDMonitorPreservesExistingCancelledTerminal(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_monitor_cancel", TaskID: "TB-2", Event: agent.EvQueued, Agent: "codex"},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_monitor_cancel", TaskID: "TB-2", Event: agent.EvStarted, Agent: "codex", PID: 5555},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-2",
		agentField:  "codex",
		agentStatus: "running",
		form:        "folder",
		events:      events,
	}})
	rec.monitorPollInterval = 5 * time.Millisecond
	var alive atomic.Bool
	alive.Store(true)
	rec.liveFn = func(int, string) bool { return alive.Load() }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if err := agent.AppendEvent(boardDir, "TB-2", agent.Event{
		TS:       "2026-05-19T10:00:02Z",
		RunID:    "r_monitor_cancel",
		TaskID:   "TB-2",
		Event:    agent.EvFinished,
		Status:   agent.StatusCancelled,
		ExitCode: -1,
		Reason:   "user cancelled",
	}); err != nil {
		t.Fatalf("append cancelled: %v", err)
	}
	alive.Store(false)

	waitForCondition(t, time.Second, func() bool {
		out, err := c.Run(context.Background(), "show", "TB-2")
		return err == nil && strings.Contains(string(out), "**AgentStatus:** cancelled")
	})
	eventsAfter := readJSONL(t, agent.StatePath(boardDir, "TB-2"))
	if got := countFinished(eventsAfter); got != 1 {
		t.Fatalf("finished event count = %d, want original cancelled only; events=%+v", got, eventsAfter)
	}
	last := eventsAfter[len(eventsAfter)-1]
	if last.Status != agent.StatusCancelled || last.Reason != "user cancelled" {
		t.Fatalf("last event = %+v, want original cancelled terminal", last)
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

func TestRecoverStale_ShutdownCancelledCarveOut(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-21T10:00:00Z", RunID: "r_shutdown", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-21T10:00:01Z", RunID: "r_shutdown", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 999999},
		{TS: "2026-05-21T10:00:02Z", RunID: "r_shutdown", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusCancelled, ExitCode: -1, Reason: "shutdown"},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.liveFn = func(pid int, expected string) bool { return false }

	before := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	after := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(after) != len(before) {
		t.Fatalf("shutdown cancelled carve-out should not append recovery event; before=%d after=%d", len(before), len(after))
	}
	last := after[len(after)-1]
	if last.Status != agent.StatusCancelled || last.Reason != "shutdown" {
		t.Fatalf("last event = %+v, want original shutdown cancellation", last)
	}
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
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

// TestRecoverStale_CancelledAgentStatusSkipsCandidate verifies that a task
// whose .md AgentStatus is already `cancelled` is filtered out at the
// candidate-selection step (recovery only looks at AgentStatus=running). It
// does NOT exercise the cancel-carve-out at the reconciliation step — that
// is covered by TestRecoverStale_FolderCancelledCarveOut, which uses
// AgentStatus=running plus an in-JSONL finished{cancelled} event so the
// task reaches the carve-out branch.
func TestRecoverStale_CancelledAgentStatusSkipsCandidate(t *testing.T) {
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

// --- TB-137/TB-251: interrupted-vs-lost branching tests ---

// TestRecoverStale_DeadPIDWithSessionID_MarksInterrupted is the
// positive case for TB-130: when the latest run captured a SessionID
// before the daemon crashed and the PID is now dead, recovery writes
// `interrupted` (not `failed`) so the user can choose to Resume the
// captured agent session.
func TestRecoverStale_DeadPIDWithSessionID_MarksInterrupted(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_int", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_int", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 4242},
		{TS: "2026-05-14T10:00:02Z", RunID: "r_int", TaskID: "TB-1", Event: agent.EvSession,
			SessionID: "11111111-2222-4333-8444-555555555555",
			PID:       4242,
			Cwd:       "/tmp/board",
			RunEnv:    map[string]string{"TB_BOARD_PATH": "/tmp/board"},
		},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.liveFn = func(int, string) bool { return false }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusInterrupted {
		t.Errorf("last event: %+v, want finished{interrupted}", last)
	}
	if last.Reason != "interrupted by daemon restart" {
		t.Errorf("reason: %q, want %q", last.Reason, "interrupted by daemon restart")
	}
	if last.ExitCode != -1 {
		t.Errorf("exit_code: %d, want -1", last.ExitCode)
	}

	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** interrupted") {
		t.Fatalf("AgentStatus not interrupted (validator may be rejecting it):\n%s", out)
	}
}

// TestRecoverStale_DeadPIDNoSessionID_MarksLost confirms the dead-PID
// branch resolves to `lost` (with the original "stale after restart"
// reason) when no SessionID was captured. The daemon lost the run result,
// so recovery must not claim the agent itself failed.
func TestRecoverStale_DeadPIDNoSessionID_MarksLost(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_nofs", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_nofs", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 9999},
		// No EvSession line — the run died before session capture.
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.liveFn = func(int, string) bool { return false }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != recoveryStatusLost {
		t.Errorf("last event: %+v, want finished{lost}", last)
	}
	if last.Reason != "stale after restart" {
		t.Errorf("reason: %q, want %q", last.Reason, "stale after restart")
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("AgentStatus not lost:\n%s", out)
	}
}

func TestRecoverStale_TerminalizesOlderStartedOnlyRunsBeforeListRuns(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_old_session", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeReview.String()},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_old_session", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeReview.String(), PID: 11111},
		{TS: "2026-05-19T10:00:02Z", RunID: "r_old_session", TaskID: "TB-1", Event: agent.EvSession, SessionID: "11111111-2222-4333-8444-555555555555", PID: 11111},
		{TS: "2026-05-19T10:10:00Z", RunID: "r_new_no_session", TaskID: "TB-1", Event: agent.EvQueued, Agent: "codex", Mode: agent.ModeReview.String()},
		{TS: "2026-05-19T10:10:01Z", RunID: "r_new_no_session", TaskID: "TB-1", Event: agent.EvStarted, Agent: "codex", Mode: agent.ModeReview.String(), PID: 22222},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "codex", events)
	rec.liveFn = func(int, string) bool { return false }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	runs, err := rec.agent.ListRuns(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	statuses := map[string]string{}
	for _, run := range runs {
		statuses[run.RunID] = run.Status
	}
	if statuses["r_old_session"] != string(agent.StatusInterrupted) {
		t.Fatalf("old run status = %q, want interrupted (all runs: %+v)", statuses["r_old_session"], runs)
	}
	if statuses["r_new_no_session"] != string(recoveryStatusLost) {
		t.Fatalf("new run status = %q, want lost (all runs: %+v)", statuses["r_new_no_session"], runs)
	}
	for _, run := range runs {
		if run.Status == "running" || run.Status == "queued" {
			t.Fatalf("stale run still surfaced as active: %+v", run)
		}
	}

	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("latest run should drive task AgentStatus=lost:\n%s", out)
	}
}

func TestRecoverStale_ReadyFolderInterruptedUsesTaskLocalState(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_ready", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_ready", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 12345},
		{TS: "2026-05-19T10:00:02Z", RunID: "r_ready", TaskID: "TB-1", Event: agent.EvSession, SessionID: "11111111-2222-4333-8444-555555555555", PID: 12345},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-1",
		agentField:  "claude",
		agentStatus: "running",
		form:        "folder",
		statusDir:   "ready",
		events:      events,
	}})
	rec.liveFn = func(int, string) bool { return false }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	readyState := filepath.Join(boardDir, "ready", "TB-1", ".agent-state.jsonl")
	if got := agent.StatePath(boardDir, "TB-1"); got != readyState {
		t.Fatalf("StatePath = %s, want %s", got, readyState)
	}
	final := readJSONL(t, readyState)
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusInterrupted {
		t.Fatalf("last event: %+v, want finished{interrupted}", last)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-1.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("ready folder task should not create board-root state, err=%v", err)
	}
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** interrupted") {
		t.Fatalf("AgentStatus not interrupted:\n%s", out)
	}
}

// TestRecoverStale_CancelledCarveOutBeatsInterrupted is the ordering
// invariant from spec § 7: a user-cancelled task with a captured
// SessionID still becomes `cancelled` on recovery, never `interrupted`.
// The carve-out short-circuits the dead-PID/SessionID branch above.
func TestRecoverStale_CancelledCarveOutBeatsInterrupted(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_cint", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_cint", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 4242},
		{TS: "2026-05-14T10:00:02Z", RunID: "r_cint", TaskID: "TB-1", Event: agent.EvSession,
			SessionID: "11111111-2222-4333-8444-555555555555",
			PID:       4242,
		},
		{TS: "2026-05-14T10:00:03Z", RunID: "r_cint", TaskID: "TB-1", Event: agent.EvFinished,
			Status: agent.StatusCancelled, ExitCode: -1, Reason: "user cancelled"},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.liveFn = func(int, string) bool { return false }

	before := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	after := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(after) != len(before) {
		t.Fatalf("cancelled carve-out should not append; before=%d after=%d", len(before), len(after))
	}
	last := after[len(after)-1]
	if last.Status != agent.StatusCancelled {
		t.Errorf("last status: %s, want cancelled (carve-out must win over interrupted)", last.Status)
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled:\n%s", out)
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

func cleanupAuditEvents(t *testing.T, path, runID string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("decode %q: %v", sc.Text(), err)
		}
		if ev["event"] == "cleanup" && ev["run_id"] == runID {
			out = append(out, ev)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return out
}

func requireCleanupAudit(t *testing.T, path, runID string, pid int, signal, target, reason string) {
	t.Helper()
	for _, ev := range cleanupAuditEvents(t, path, runID) {
		gotPID, _ := ev["pid"].(float64)
		if int(gotPID) == pid && ev["signal"] == signal && ev["target"] == target && ev["reason"] == reason {
			return
		}
	}
	t.Fatalf("missing cleanup audit event run=%s pid=%d signal=%s target=%s reason=%q in %s; got %+v",
		runID, pid, signal, target, reason, path, cleanupAuditEvents(t, path, runID))
}

func countFinished(events []agent.Event) int {
	n := 0
	for _, ev := range events {
		if ev.Event == agent.EvFinished {
			n++
		}
	}
	return n
}

func waitForCondition(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ok() {
		return
	}
	t.Fatalf("condition not met within %s", timeout)
}

func startSleepProcessGroup(t *testing.T) (*exec.Cmd, int, <-chan struct{}) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "sleep 60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleep: %v", err)
	}
	pid := cmd.Process.Pid
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cmd.Wait()
	}()
	return cmd, pid, done
}

// --- TB-130: resumableSessionID tests ---
//
// These exercise the helper directly without a tb binary fixture — only
// AppendEvent + os scaffolding. resumableSessionID is the gate for the
// Resume button and recovery's interrupted-vs-lost branch (TB-137/TB-251),
// so its "latest run only" invariant is the contract we lock down here.

func resumableSessionTestBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "backlog", "TB-1"), 0o755); err != nil {
		t.Fatalf("mkdir backlog/TB-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backlog", "TB-1", "TASK.md"), []byte("# TB-1: t\n"), 0o644); err != nil {
		t.Fatalf("write TASK.md: %v", err)
	}
	return dir
}

func TestResumableSessionID_NoJSONLFile(t *testing.T) {
	dir := resumableSessionTestBoard(t)
	_, ok, err := resumableSessionID(dir, "TB-1")
	if err != nil {
		t.Fatalf("resumableSessionID: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing JSONL")
	}
}

func TestResumableSessionID_LatestRunHasSession(t *testing.T) {
	dir := resumableSessionTestBoard(t)
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_a", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_a", Event: agent.EvStarted, Agent: "claude", PID: 1000},
		{TS: "2026-05-14T10:00:02Z", RunID: "r_a", Event: agent.EvSession,
			SessionID: "uuid-aaaa",
			PID:       1000,
			Cwd:       "/tmp/wt/TB-1",
			RunEnv:    map[string]string{"TB_BOARD_PATH": "/tmp/board"},
		},
	}
	for _, ev := range events {
		if err := agent.AppendEvent(dir, "TB-1", ev); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	cand, ok, err := resumableSessionID(dir, "TB-1")
	if err != nil {
		t.Fatalf("resumableSessionID: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := ResumeCandidate{
		SessionID: "uuid-aaaa",
		RunID:     "r_a",
		Cwd:       "/tmp/wt/TB-1",
		Env:       map[string]string{"TB_BOARD_PATH": "/tmp/board"},
	}
	if cand.SessionID != want.SessionID || cand.RunID != want.RunID || cand.Cwd != want.Cwd {
		t.Fatalf("ResumeCandidate mismatch:\n got %+v\nwant %+v", cand, want)
	}
	if got := cand.Env["TB_BOARD_PATH"]; got != "/tmp/board" {
		t.Fatalf("Env TB_BOARD_PATH: got %q, want %q", got, "/tmp/board")
	}
}

// A task that the agent moved to code-review while a daemon-tracked run
// was still in flight is reconciled the same way as a backlog/in-progress/
// done task. Prior to this fix RecoverStale's bucket walk skipped
// CodeReview entirely, so such tasks stayed at AgentStatus=running across
// a daemon restart and blocked the drawer's Run/Groom controls until
// manually cleared.
func TestRecoverStale_CodeReviewBucket_ReconcilesRunning(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T13:35:00Z", RunID: "r_cr", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-19T13:35:01Z", RunID: "r_cr", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 99999},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-1",
		agentField:  "claude",
		agentStatus: "running",
		form:        "folder",
		statusDir:   "code-review",
		events:      events,
	}})

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(final) < 3 {
		t.Fatalf("expected synthetic finished for code-review task; have %d events", len(final))
	}
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != recoveryStatusLost {
		t.Errorf("last event: %+v; want finished{lost}", last)
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("AgentStatus not reconciled to lost:\n%s", out)
	}
}

// Mirror of the above for the archive bucket — a closed task with a
// dangling running run still needs reconciliation so the next daemon
// session doesn't leak the stale state into resume probes.
func TestRecoverStale_ArchiveBucket_ReconcilesRunning(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T13:35:00Z", RunID: "r_arc", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-19T13:35:01Z", RunID: "r_arc", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 99999},
	}
	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-1",
		agentField:  "claude",
		agentStatus: "running",
		form:        "file",
		statusDir:   "archive",
		events:      events,
	}})

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	final := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	last := final[len(final)-1]
	if last.Event != agent.EvFinished || last.Status != recoveryStatusLost {
		t.Errorf("last event: %+v; want finished{lost}", last)
	}
	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("AgentStatus not reconciled to lost:\n%s", out)
	}
}

// TB-176: when recovery decides the orphaned PID is still alive, it
// must adopt a stub activeRun in s.active so the drawer's Cancel
// button reaches the kill cascade in CancelRun instead of returning
// ErrNotRunning. Also confirms ListRuns reports Detached=false for
// the adopted run — the frontend uses that to keep showing Cancel as
// an enabled action.
func TestRecoverStale_LivePIDAdoptsStubForCancel(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_adopt", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeImplement.String()},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_adopt", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeImplement.String(), PID: 7777},
	}
	rec, board, boardDir, _ := recoveryFixture(t, "TB-1", "claude", events)
	rec.liveFn = func(int, string) bool { return true }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	ar := rec.agent.getActiveRun("TB-1")
	if ar == nil {
		t.Fatalf("expected stub activeRun adopted for live recovered PID; s.active was empty")
	}
	if ar.Pid != 7777 {
		t.Errorf("stub Pid = %d, want 7777", ar.Pid)
	}
	if ar.RunID != "r_adopt" {
		t.Errorf("stub RunID = %q, want r_adopt", ar.RunID)
	}
	if ar.Agent != "claude" {
		t.Errorf("stub Agent = %q, want claude", ar.Agent)
	}
	if ar.Mode != agent.ModeImplement.String() {
		t.Errorf("stub Mode = %q, want %q", ar.Mode, agent.ModeImplement.String())
	}

	runs, err := rec.agent.ListRuns(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	var found bool
	for _, r := range runs {
		if r.RunID != "r_adopt" {
			continue
		}
		found = true
		if r.Detached {
			t.Errorf("ListRuns Detached = true; adopt should keep the run attached so Cancel is enabled")
		}
		if r.Status != "running" {
			t.Errorf("ListRuns Status = %q, want running", r.Status)
		}
	}
	if !found {
		t.Errorf("ListRuns did not include r_adopt; got %+v", runs)
	}
	_ = board
}

// TB-176: a CancelRun against a stub adopted for a live recovered PID
// must signal the orphaned process group, wait for it to die, and
// write the cancelled terminal record. End-to-end with a real /bin/sh
// process so the SIGTERM path is exercised against the kernel.
func TestCancelRun_RecoveredLivePID_KillsAndWritesCancelled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
	}
	// Spawn a real subprocess that exits on SIGTERM, in its own pgrp so
	// the kill cascade works the same way as a real recovered orphan.
	cmd := exec.Command("/bin/sh", "-c", "sleep 60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleep: %v", err)
	}
	pid := cmd.Process.Pid
	// Reap the process asynchronously so kill(pid, 0) flips to ESRCH
	// the moment the kernel finishes tearing it down. Without this the
	// kernel keeps the slot as a zombie, the monitor's liveFn keeps
	// reporting "alive", and CancelRun's wait-on-Done never resolves.
	reaped := make(chan struct{})
	go func() {
		defer close(reaped)
		_ = cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-reaped
	})

	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_live", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeImplement.String()},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_live", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeImplement.String(), PID: pid},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.monitorPollInterval = 10 * time.Millisecond
	rec.liveFn = func(probePid int, _ string) bool {
		return syscall.Kill(probePid, 0) == nil
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if rec.agent.getActiveRun("TB-1") == nil {
		t.Fatalf("recovery did not adopt a stub for the live PID")
	}

	start := time.Now()
	if err := rec.agent.CancelRun(context.Background(), "TB-1"); err != nil {
		t.Fatalf("CancelRun on recovered live PID: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("CancelRun took too long: %v", elapsed)
	}

	// Kernel beat to reap, then verify the orphan is dead.
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("recovered pid %d still alive after CancelRun", pid)
	}

	events2 := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	finished := 0
	for _, ev := range events2 {
		if ev.RunID != "r_live" || ev.Event != agent.EvFinished {
			continue
		}
		finished++
		if ev.Status != agent.StatusCancelled {
			t.Errorf("terminal status = %s, want cancelled", ev.Status)
		}
		if ev.Reason != "user cancelled" {
			t.Errorf("terminal reason = %q, want user cancelled", ev.Reason)
		}
	}
	if finished != 1 {
		t.Errorf("got %d finished events for r_live, want exactly 1", finished)
	}
	requireCleanupAudit(t, agent.StatePath(boardDir, "TB-1"), "r_live", pid, "SIGTERM", "pid", "user cancelled")

	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Errorf("AgentStatus not cancelled:\n%s", out)
	}

	// s.active must be cleaned up: a follow-up cancel returns ErrNotRunning.
	if err := rec.agent.CancelRun(context.Background(), "TB-1"); err == nil {
		t.Errorf("second CancelRun expected error, got nil")
	}
}

func TestCancelRun_RecoveredLivePID_RechecksIdentityBeforeKilling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
	}
	cmd, pid, waitDone := startSleepProcessGroup(t)
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-waitDone
	})

	events := []agent.Event{
		{TS: "2026-05-20T10:00:00Z", RunID: "r_identity", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeImplement.String()},
		{TS: "2026-05-20T10:00:01Z", RunID: "r_identity", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeImplement.String(), PID: pid},
	}
	rec, _, boardDir, _ := recoveryFixture(t, "TB-1", "claude", events)
	rec.monitorPollInterval = 10 * time.Millisecond
	var identityOK atomic.Bool
	identityOK.Store(true)
	rec.liveFn = func(probePid int, expected string) bool {
		return probePid == pid && expected == "claude" && identityOK.Load()
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if rec.agent.getActiveRun("TB-1") == nil {
		t.Fatalf("recovery did not adopt a stub for the live PID")
	}

	identityOK.Store(false)
	if err := rec.agent.CancelRun(context.Background(), "TB-1"); err == nil {
		t.Fatalf("CancelRun succeeded after identity mismatch; want refusal without signalling")
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("process was signalled despite identity mismatch: %v", err)
	}
	if got := cleanupAuditEvents(t, agent.StatePath(boardDir, "TB-1"), "r_identity"); len(got) != 0 {
		t.Fatalf("identity mismatch should not write cleanup audit events: %+v", got)
	}
}

func TestCancelRun_RecoveredLivePID_DoesNotKillRunWithTerminalEvent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
	}
	cmd, pid, waitDone := startSleepProcessGroup(t)
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-waitDone
	})

	events := []agent.Event{
		{TS: "2026-05-20T10:10:00Z", RunID: "r_terminal_live", TaskID: "TB-1", Event: agent.EvQueued, Agent: "codex", Mode: agent.ModeImplement.String()},
		{TS: "2026-05-20T10:10:01Z", RunID: "r_terminal_live", TaskID: "TB-1", Event: agent.EvStarted, Agent: "codex", Mode: agent.ModeImplement.String(), PID: pid},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "codex", events)
	rec.monitorPollInterval = 10 * time.Millisecond
	rec.liveFn = func(probePid int, expected string) bool {
		return probePid == pid && expected == "codex"
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:       "2026-05-20T10:10:02Z",
		RunID:    "r_terminal_live",
		TaskID:   "TB-1",
		Event:    agent.EvFinished,
		Status:   agent.StatusSuccess,
		ExitCode: 0,
	}); err != nil {
		t.Fatalf("append finished: %v", err)
	}
	waitForCondition(t, time.Second, func() bool {
		out, err := c.Run(context.Background(), "show", "TB-1")
		return err == nil && strings.Contains(string(out), "**AgentStatus:** success")
	})

	if err := rec.agent.CancelRun(context.Background(), "TB-1"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("CancelRun after terminal event: got %v, want ErrNotRunning", err)
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("terminal run process should not be signalled by cleanup: %v", err)
	}
	if got := cleanupAuditEvents(t, agent.StatePath(boardDir, "TB-1"), "r_terminal_live"); len(got) != 0 {
		t.Fatalf("terminal run should not get cleanup audit events: %+v", got)
	}
}

func TestCancelRun_RecoveredLivePID_DoneTaskWritesCleanupAudit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
	}
	_, pid, waitDone := startSleepProcessGroup(t)
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-waitDone
	})

	rec, _, boardDir, _ := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-1",
		agentField:  "claude",
		agentStatus: "running",
		statusDir:   "done",
		form:        "folder",
		events: []agent.Event{
			{TS: "2026-05-20T10:20:00Z", RunID: "r_done_orphan", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeImplement.String()},
			{TS: "2026-05-20T10:20:01Z", RunID: "r_done_orphan", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeImplement.String(), PID: pid},
		},
	}})
	rec.monitorPollInterval = 10 * time.Millisecond
	rec.liveFn = func(probePid int, expected string) bool {
		return probePid == pid && expected == "claude"
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if err := rec.agent.CancelRun(context.Background(), "TB-1"); err != nil {
		t.Fatalf("CancelRun on done orphan: %v", err)
	}
	requireCleanupAudit(t, agent.StatePath(boardDir, "TB-1"), "r_done_orphan", pid, "SIGTERM", "pid", "user cancelled")
}

// TB-176: same end-to-end cancel-kill-terminal contract, but for a
// folder-form task. State paths differ
// (`<status>/<ID>/.agent-state.jsonl`), so a routing regression that
// mis-targets the file-form path would slip past the file-form test
// above without this coverage.
func TestCancelRun_RecoveredLivePID_FolderForm_KillsAndWritesCancelled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
	}
	cmd := exec.Command("/bin/sh", "-c", "sleep 60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleep: %v", err)
	}
	pid := cmd.Process.Pid
	reaped := make(chan struct{})
	go func() {
		defer close(reaped)
		_ = cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-reaped
	})

	rec, _, boardDir, c := recoveryFixtureWithTasks(t, []recoveryTaskFixture{{
		id:          "TB-1",
		agentField:  "codex",
		agentStatus: "running",
		form:        "folder",
		events: []agent.Event{
			{TS: "2026-05-19T10:00:00Z", RunID: "r_live_folder", TaskID: "TB-1", Event: agent.EvQueued, Agent: "codex", Mode: agent.ModeImplement.String()},
			{TS: "2026-05-19T10:00:01Z", RunID: "r_live_folder", TaskID: "TB-1", Event: agent.EvStarted, Agent: "codex", Mode: agent.ModeImplement.String(), PID: pid},
		},
	}})
	rec.monitorPollInterval = 10 * time.Millisecond
	rec.liveFn = func(probePid int, _ string) bool {
		return syscall.Kill(probePid, 0) == nil
	}

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	if rec.agent.getActiveRun("TB-1") == nil {
		t.Fatalf("recovery did not adopt a stub for the live PID")
	}
	if err := rec.agent.CancelRun(context.Background(), "TB-1"); err != nil {
		t.Fatalf("CancelRun on folder-form recovered live PID: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("recovered pid %d still alive after CancelRun", pid)
	}

	folderState := filepath.Join(boardDir, "backlog", "TB-1", ".agent-state.jsonl")
	if agent.StatePath(boardDir, "TB-1") != folderState {
		t.Fatalf("folder-form StatePath = %s, want %s", agent.StatePath(boardDir, "TB-1"), folderState)
	}

	finished := 0
	for _, ev := range readJSONL(t, folderState) {
		if ev.RunID != "r_live_folder" || ev.Event != agent.EvFinished {
			continue
		}
		finished++
		if ev.Status != agent.StatusCancelled {
			t.Errorf("terminal status = %s, want cancelled", ev.Status)
		}
		if ev.Reason != "user cancelled" {
			t.Errorf("terminal reason = %q, want user cancelled", ev.Reason)
		}
	}
	if finished != 1 {
		t.Errorf("got %d finished events for r_live_folder, want exactly 1", finished)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-1.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("folder-form cancel should not write to board-root state, err=%v", err)
	}

	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Errorf("AgentStatus not cancelled:\n%s", out)
	}
}

// TB-176: a CancelRun that races a natural orphan exit must still
// produce exactly one terminal record, and the cancelled carve-out
// must beat the monitor's "orphaned process exited" failure record
// when the user marked the run cancelled first.
func TestRecoverStale_LivePIDCancelBeatsMonitorOnRaceWithExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
	}
	events := []agent.Event{
		{TS: "2026-05-19T10:00:00Z", RunID: "r_race", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeImplement.String()},
		{TS: "2026-05-19T10:00:01Z", RunID: "r_race", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeImplement.String(), PID: 12321},
	}
	rec, _, boardDir, c := recoveryFixture(t, "TB-1", "claude", events)
	rec.monitorPollInterval = 5 * time.Millisecond

	var alive atomic.Bool
	alive.Store(true)
	rec.liveFn = func(int, string) bool { return alive.Load() }

	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	ar := rec.agent.getActiveRun("TB-1")
	if ar == nil {
		t.Fatalf("recovery did not adopt a stub for the live PID")
	}

	// Simulate the user clicking Cancel: mark cancelled BEFORE the PID
	// exits. This is the order CancelRun guarantees (markCancelled
	// fires before killActiveRun). The monitor's next poll will see
	// the PID gone but defer the terminal to the cancel path.
	ar.markCancelled()
	alive.Store(false)

	// Wait for the monitor to observe the exit and unblock Done.
	waitForCondition(t, time.Second, func() bool {
		select {
		case <-ar.Done:
			return true
		default:
			return false
		}
	})

	// CancelRun is what writes the terminal record. We can't run the
	// real CancelRun here because the PID never existed (12321 is
	// synthetic) — exercise finishCancelled directly, which is what
	// CancelRun calls after killActiveRun returns.
	if err := rec.agent.finishCancelled(rec.agent.board.snapshot(), ar, boardDir, "user cancelled"); err != nil {
		t.Fatalf("finishCancelled: %v", err)
	}

	finished := 0
	last := agent.Event{}
	for _, ev := range readJSONL(t, agent.StatePath(boardDir, "TB-1")) {
		if ev.RunID != "r_race" || ev.Event != agent.EvFinished {
			continue
		}
		finished++
		last = ev
	}
	if finished != 1 {
		t.Errorf("got %d finished events, want exactly 1 (cancelled carve-out should beat monitor)", finished)
	}
	if last.Status != agent.StatusCancelled {
		t.Errorf("terminal status = %s, want cancelled", last.Status)
	}
	if last.Reason != "user cancelled" {
		t.Errorf("terminal reason = %q, want user cancelled", last.Reason)
	}

	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Errorf("AgentStatus not cancelled:\n%s", out)
	}
}

// Spec § 5 invariant: helper looks at the latest run ONLY. If the latest
// run failed before capturing a session id, resume is disabled — even
// when an older run did capture one. Walking backward would resume a
// stale conversation.
func TestResumableSessionID_LatestRunHasNoSession_OlderRunDoes(t *testing.T) {
	dir := resumableSessionTestBoard(t)
	events := []agent.Event{
		// Older run with session captured (would be resumable on its own).
		{TS: "2026-05-14T10:00:00Z", RunID: "r_old", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_old", Event: agent.EvStarted, Agent: "claude", PID: 1000},
		{TS: "2026-05-14T10:00:02Z", RunID: "r_old", Event: agent.EvSession, SessionID: "uuid-old", PID: 1000},
		{TS: "2026-05-14T10:00:03Z", RunID: "r_old", Event: agent.EvFinished, Status: agent.StatusInterrupted, ExitCode: -1},
		// Newer run failed before session capture (the "stale" case).
		{TS: "2026-05-14T11:00:00Z", RunID: "r_new", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T11:00:01Z", RunID: "r_new", Event: agent.EvStarted, Agent: "claude", PID: 2000},
		{TS: "2026-05-14T11:00:02Z", RunID: "r_new", Event: agent.EvFinished, Status: agent.StatusFailed, ExitCode: -1, Reason: "binary not found"},
	}
	for _, ev := range events {
		if err := agent.AppendEvent(dir, "TB-1", ev); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	_, ok, err := resumableSessionID(dir, "TB-1")
	if err != nil {
		t.Fatalf("resumableSessionID: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false: latest run has no session event, walking backward must not resume the older one")
	}
}
