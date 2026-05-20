package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

func TestReconcileActiveReusesInitialSnapshot(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "calls.log")
	boardDir := t.TempDir()
	stub := makeStub(t, fmt.Sprintf(`
printf '%%s\n' "$*" >> %q
case "$1" in
  ls)
    cat <<JSON
[
  {"id":"TB-1","title":"A","status":"backlog","tags":[]},
  {"id":"TB-2","title":"B","status":"backlog","tags":[]}
]
JSON
    ;;
  board)
    echo '{}'
    ;;
  show)
    id="$2"
    cat <<JSON
{"metadata":{"id":"$id","title":"A","status":"backlog","tags":[]},"body":""}
JSON
    ;;
  *)
    echo '{}'
    ;;
esac
`, logPath))
	c := newClient(t, stub)
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	rec := NewStageReconciler(board, nil)

	if err := rec.ReconcileActive(context.Background()); err != nil {
		t.Fatalf("ReconcileActive: %v", err)
	}

	gotBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	if got := strings.Count(string(gotBytes), "ls --json --status active\n"); got != 1 {
		t.Fatalf("active board loads = %d, want 1; calls:\n%s", got, gotBytes)
	}
}

type stageReconcileFixture struct {
	t        *testing.T
	root     string
	boardDir string
	board    *BoardService
	client   *cli.Client
	rec      *StageReconciler
}

func newStageReconcileFixture(t *testing.T, config string) *stageReconcileFixture {
	t.Helper()
	tbBinary := buildTbForIntegration(t)
	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "ready", "in-progress", "code-review", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if strings.TrimSpace(config) == "" {
		config = "board: board\nprefix: TB\n"
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("999\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	return &stageReconcileFixture{
		t:        t,
		root:     root,
		boardDir: boardDir,
		board:    board,
		client:   c,
		rec:      NewStageReconciler(board, nil),
	}
}

func (f *stageReconcileFixture) writeTask(status, id, metadata, body string) {
	f.t.Helper()
	if strings.TrimSpace(body) == "" {
		body = "## Goal\n\nImplement the thing.\n\n## Acceptance Criteria\n\n- [ ] one\n- [ ] two\n\n## Log\n\n- 2026-05-20: Created\n"
	}
	content := "# " + id + ": Reconcile candidate\n\n" + metadata + "\n" + body
	path := filepath.Join(f.boardDir, status, id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
}

func reconcileMetadata(extra ...string) string {
	lines := []string{
		"**Type:** bug",
		"**Priority:** P1",
		"**Size:** S",
		"**Module:** gui",
	}
	lines = append(lines, extra...)
	lines = append(lines, "**Branch:** -")
	return strings.Join(lines, "\n")
}

func (f *stageReconcileFixture) appendRun(id, runID string, mode agent.Mode, initiator string, status agent.Status, openStarted bool) {
	f.t.Helper()
	if err := agent.AppendEvent(f.boardDir, id, agent.Event{
		TS:        time.Now().UTC().Format(time.RFC3339),
		RunID:     runID,
		TaskID:    id,
		Event:     agent.EvQueued,
		Agent:     "claude",
		Mode:      mode.String(),
		Initiator: initiator,
	}); err != nil {
		f.t.Fatalf("append queued: %v", err)
	}
	if openStarted || status != "" {
		if err := agent.AppendEvent(f.boardDir, id, agent.Event{
			TS:     time.Now().UTC().Format(time.RFC3339),
			RunID:  runID,
			TaskID: id,
			Event:  agent.EvStarted,
			Agent:  "claude",
			Mode:   mode.String(),
			PID:    99999,
		}); err != nil {
			f.t.Fatalf("append started: %v", err)
		}
	}
	if status != "" {
		if err := agent.AppendEvent(f.boardDir, id, agent.Event{
			TS:       time.Now().UTC().Format(time.RFC3339),
			RunID:    runID,
			TaskID:   id,
			Event:    agent.EvFinished,
			Agent:    "claude",
			Mode:     mode.String(),
			Status:   status,
			ExitCode: 0,
		}); err != nil {
			f.t.Fatalf("append finished: %v", err)
		}
	}
}

func (f *stageReconcileFixture) requireStatus(id, status string) {
	f.t.Helper()
	if _, err := os.Stat(filepath.Join(f.boardDir, status, id+".md")); err != nil {
		f.t.Fatalf("%s should be in %s: %v", id, status, err)
	}
}

func (f *stageReconcileFixture) requireNotStatus(id, status string) {
	f.t.Helper()
	if _, err := os.Stat(filepath.Join(f.boardDir, status, id+".md")); !os.IsNotExist(err) {
		f.t.Fatalf("%s should not be in %s: err=%v", id, status, err)
	}
}

func (f *stageReconcileFixture) skipEntries() map[string]reconcileSkipEntry {
	f.t.Helper()
	data, err := os.ReadFile(reconcileSkipPath(f.boardDir))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]reconcileSkipEntry{}
		}
		f.t.Fatalf("read skip file: %v", err)
	}
	var payload reconcileSkipFile
	if err := json.Unmarshal(data, &payload); err != nil {
		f.t.Fatalf("decode skip file: %v", err)
	}
	return payload.Entries
}

func TestStageReconciler_AutoGroomSuccessPromotesCleanBacklog(t *testing.T) {
	f := newStageReconcileFixture(t, "")
	f.writeTask("backlog", "TB-1", reconcileMetadata("**Agent:** claude", "**AgentStatus:** success"), "")
	f.appendRun("TB-1", "r_groom", agent.ModeGroom, agent.InitiatorAutoGroom, agent.StatusSuccess, false)

	if err := f.rec.ReconcileTask(context.Background(), "TB-1"); err != nil {
		t.Fatalf("ReconcileTask: %v", err)
	}
	f.requireStatus("TB-1", "ready")
	f.requireNotStatus("TB-1", "backlog")
}

func TestStageReconciler_AutoImplementStartPullsReadyQueuedRun(t *testing.T) {
	f := newStageReconcileFixture(t, "")
	f.writeTask("ready", "TB-2", reconcileMetadata("**Agent:** claude", "**AgentStatus:** queued"), "")
	f.appendRun("TB-2", "r_impl", agent.ModeImplement, agent.InitiatorAutoImplement, "", false)

	if err := f.rec.ReconcileTask(context.Background(), "TB-2"); err != nil {
		t.Fatalf("ReconcileTask: %v", err)
	}
	f.requireStatus("TB-2", "in-progress")
	f.requireNotStatus("TB-2", "ready")
}

func TestStageReconciler_AutoImplementSuccessSubmitsWhenReviewRefPresent(t *testing.T) {
	f := newStageReconcileFixture(t, "")
	f.writeTask("in-progress", "TB-3", reconcileMetadata(
		"**ReviewRef:** branch: feature/tb-3",
		"**Agent:** claude",
		"**AgentStatus:** success",
		"**ImplementedBy:** claude",
		"**ImplementStatus:** success",
	), "")
	f.appendRun("TB-3", "r_impl_done", agent.ModeImplement, agent.InitiatorAutoImplement, agent.StatusSuccess, false)

	if err := f.rec.ReconcileTask(context.Background(), "TB-3"); err != nil {
		t.Fatalf("ReconcileTask: %v", err)
	}
	f.requireStatus("TB-3", "code-review")
	f.requireNotStatus("TB-3", "in-progress")
}

func TestStageReconciler_AutoImplementMissingReviewRefBacksOff(t *testing.T) {
	f := newStageReconcileFixture(t, "")
	f.writeTask("in-progress", "TB-4", reconcileMetadata(
		"**Agent:** claude",
		"**AgentStatus:** success",
		"**ImplementedBy:** claude",
		"**ImplementStatus:** success",
	), "")
	f.appendRun("TB-4", "r_missing_ref", agent.ModeImplement, agent.InitiatorAutoImplement, agent.StatusSuccess, false)

	if err := f.rec.ReconcileTask(context.Background(), "TB-4"); err != nil {
		t.Fatalf("first ReconcileTask: %v", err)
	}
	if err := f.rec.ReconcileTask(context.Background(), "TB-4"); err != nil {
		t.Fatalf("second ReconcileTask: %v", err)
	}
	f.requireStatus("TB-4", "in-progress")
	entries := f.skipEntries()
	entry, ok := entries["TB-4|auto-implement-submit"]
	if !ok {
		t.Fatalf("missing durable submit skip: %+v", entries)
	}
	if !strings.Contains(entry.Reason, "ReviewRef") {
		t.Fatalf("skip reason = %q, want ReviewRef diagnostic", entry.Reason)
	}
	if entry.Attempts != 1 {
		t.Fatalf("skip attempts = %d, want 1 after same-fingerprint retry", entry.Attempts)
	}
}

func TestStageReconciler_ReviewFailedFindingsMoveToReady(t *testing.T) {
	f := newStageReconcileFixture(t, "")
	body := "## Goal\n\nImplement the thing.\n\n## Acceptance Criteria\n\n- [ ] one\n\n## Review Findings\n\n- regression found\n\n## Log\n\n- 2026-05-20: Created\n"
	f.writeTask("code-review", "TB-5", reconcileMetadata("**Tags:** review-failed", "**Agent:** claude", "**AgentStatus:** success"), body)

	if err := f.rec.ReconcileTask(context.Background(), "TB-5"); err != nil {
		t.Fatalf("ReconcileTask: %v", err)
	}
	f.requireStatus("TB-5", "ready")
	f.requireNotStatus("TB-5", "code-review")
}

func TestStageReconciler_ReviewPassWithoutManagedMarkerIsConservative(t *testing.T) {
	f := newStageReconcileFixture(t, "")
	f.writeTask("code-review", "TB-6", reconcileMetadata("**ReviewRef:** branch: feature/tb-6", "**Agent:** claude", "**AgentStatus:** success"), "")
	f.appendRun("TB-6", "r_review_pass", agent.ModeReview, agent.InitiatorUser, agent.StatusSuccess, false)

	if err := f.rec.ReconcileTask(context.Background(), "TB-6"); err != nil {
		t.Fatalf("ReconcileTask: %v", err)
	}
	f.requireStatus("TB-6", "code-review")
	f.requireNotStatus("TB-6", "done")
}

func TestStageReconciler_ProtectedAgentStatusesArePreserved(t *testing.T) {
	for _, status := range []string{"needs-user", "cancelled", "interrupted", "lost"} {
		t.Run(status, func(t *testing.T) {
			f := newStageReconcileFixture(t, "")
			f.writeTask("in-progress", "TB-7", reconcileMetadata(
				"**ReviewRef:** branch: feature/tb-7",
				"**Agent:** claude",
				"**AgentStatus:** "+status,
				"**ImplementedBy:** claude",
				"**ImplementStatus:** success",
			), "")
			f.appendRun("TB-7", "r_protected_"+status, agent.ModeImplement, agent.InitiatorAutoImplement, agent.StatusSuccess, false)

			if err := f.rec.ReconcileTask(context.Background(), "TB-7"); err != nil {
				t.Fatalf("ReconcileTask: %v", err)
			}
			f.requireStatus("TB-7", "in-progress")
			detail, err := f.board.GetTask(context.Background(), "TB-7")
			if err != nil {
				t.Fatalf("GetTask: %v", err)
			}
			if detail.Metadata.AgentStatus != status {
				t.Fatalf("AgentStatus = %q, want %q", detail.Metadata.AgentStatus, status)
			}
		})
	}
}

func TestStageReconciler_WIPBlockedRepairsBackOff(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		taskStatus string
		taskID     string
		metadata   string
		body       string
		runMode    agent.Mode
		runStatus  agent.Status
		initiator  string
		openRun    bool
		fillStatus string
		key        string
	}{
		{
			name:       "ready limit blocks groom promotion",
			config:     "board: board\nprefix: TB\nwip_limit_ready: 1\nwip_enforcement: strict\n",
			taskStatus: "backlog",
			taskID:     "TB-8",
			metadata:   reconcileMetadata("**Agent:** claude", "**AgentStatus:** success"),
			runMode:    agent.ModeGroom,
			runStatus:  agent.StatusSuccess,
			initiator:  agent.InitiatorAutoGroom,
			fillStatus: "ready",
			key:        "TB-8|auto-groom-ready",
		},
		{
			name:       "in-progress limit blocks implement pull",
			config:     "board: board\nprefix: TB\nwip_limit_in_progress: 1\nwip_enforcement: strict\n",
			taskStatus: "ready",
			taskID:     "TB-9",
			metadata:   reconcileMetadata("**Agent:** claude", "**AgentStatus:** queued"),
			runMode:    agent.ModeImplement,
			initiator:  agent.InitiatorAutoImplement,
			fillStatus: "in-progress",
			key:        "TB-9|auto-implement-start",
		},
		{
			name:       "code-review limit blocks implement submit",
			config:     "board: board\nprefix: TB\nwip_limit_code_review: 1\nwip_enforcement: strict\n",
			taskStatus: "in-progress",
			taskID:     "TB-10",
			metadata: reconcileMetadata(
				"**ReviewRef:** branch: feature/tb-10",
				"**Agent:** claude",
				"**AgentStatus:** success",
				"**ImplementedBy:** claude",
				"**ImplementStatus:** success",
			),
			runMode:    agent.ModeImplement,
			runStatus:  agent.StatusSuccess,
			initiator:  agent.InitiatorAutoImplement,
			fillStatus: "code-review",
			key:        "TB-10|auto-implement-submit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newStageReconcileFixture(t, tt.config)
			f.writeTask(tt.fillStatus, "TB-900", reconcileMetadata(), "")
			f.writeTask(tt.taskStatus, tt.taskID, tt.metadata, tt.body)
			f.appendRun(tt.taskID, "r_wip", tt.runMode, tt.initiator, tt.runStatus, tt.openRun)

			if err := f.rec.ReconcileTask(context.Background(), tt.taskID); err != nil {
				t.Fatalf("first ReconcileTask: %v", err)
			}
			if err := f.rec.ReconcileTask(context.Background(), tt.taskID); err != nil {
				t.Fatalf("second ReconcileTask: %v", err)
			}
			f.requireStatus(tt.taskID, tt.taskStatus)
			entry, ok := f.skipEntries()[tt.key]
			if !ok {
				t.Fatalf("missing durable WIP skip for %s: %+v", tt.key, f.skipEntries())
			}
			if !strings.Contains(entry.Reason, "WIP limit") {
				t.Fatalf("skip reason = %q, want WIP limit", entry.Reason)
			}
			if entry.Attempts != 1 {
				t.Fatalf("skip attempts = %d, want 1", entry.Attempts)
			}
		})
	}
}

func TestStageReconciler_ReviewFailedStrictWIPBacksOffPartialWrite(t *testing.T) {
	f := newStageReconcileFixture(t, "board: board\nprefix: TB\nwip_limit_ready: 1\nwip_enforcement: strict\n")
	f.writeTask("ready", "TB-901", reconcileMetadata(), "")
	body := "## Goal\n\nImplement the thing.\n\n## Acceptance Criteria\n\n- [ ] one\n\n## Review Findings\n\n- still broken\n\n## Log\n\n- 2026-05-20: Created\n"
	f.writeTask("code-review", "TB-11", reconcileMetadata("**Tags:** review-failed", "**Agent:** claude", "**AgentStatus:** success"), body)

	if err := f.rec.ReconcileTask(context.Background(), "TB-11"); err != nil {
		t.Fatalf("first ReconcileTask: %v", err)
	}
	info, err := os.Stat(filepath.Join(f.boardDir, "code-review", "TB-11.md"))
	if err != nil {
		t.Fatalf("stat partial task: %v", err)
	}
	firstMod := info.ModTime()
	time.Sleep(20 * time.Millisecond)
	if err := f.rec.ReconcileTask(context.Background(), "TB-11"); err != nil {
		t.Fatalf("second ReconcileTask: %v", err)
	}
	info, err = os.Stat(filepath.Join(f.boardDir, "code-review", "TB-11.md"))
	if err != nil {
		t.Fatalf("stat partial task after retry: %v", err)
	}
	if !info.ModTime().Equal(firstMod) {
		t.Fatalf("second same-fingerprint reconcile rewrote partial review-fail file: before=%s after=%s", firstMod, info.ModTime())
	}
	f.requireStatus("TB-11", "code-review")
	entry, ok := f.skipEntries()["TB-11|review-failed-ready"]
	if !ok {
		t.Fatalf("missing review-failed skip: %+v", f.skipEntries())
	}
	if !strings.Contains(entry.Reason, "WIP limit") {
		t.Fatalf("skip reason = %q, want WIP limit", entry.Reason)
	}
}
