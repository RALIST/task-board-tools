package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// cmdReady promotes a task from backlog to the ready column. The canonical
// kanban "commitment point": a task entering ready must be well-groomed
// (same gate `tb triage` uses), because downstream pulls assume the work is
// actionable without further triage.
//
//	tb ready <ID>
//
// If the task fails the gate, the command exits non-zero and the task stays
// in backlog. Tasks already in ready are a noop. Tasks in any other status
// are rejected so this command cannot accidentally drag a `done` task
// backwards.
func cmdReady(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb ready <ID>\n\nExample: tb ready 123\n\nPromotes a task from backlog to ready (the canonical kanban commitment column).\nThe task must pass the same gate as `tb triage` (priority + non-placeholder goal).")
		os.Exit(1)
	}
	strictWIP := false
	if args[0] == "--strict-wip" {
		strictWIP = true
		args = args[1:]
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb ready <ID>")
		os.Exit(1)
	}
	msg, err := promoteToReadyWithOptions(normalizeTaskID(args[0]), readyOptions{strictWIP: strictWIP})
	if err != nil {
		fatal("%v", err)
	}
	if msg != "" {
		fmt.Println(msg)
	}
}

// promoteToReady is the testable core of `tb ready`. Returns a status
// message on success, or an error on validation failure / WIP block. Noop
// returns ("", nil) and emits a stderr line so existing scripts that pipe
// stdout still see no spurious output.
//
// The triage gate runs OUTSIDE the lock for fast-fail UX; expected-source
// + WIP get re-checked INSIDE the lock via the guard so a concurrent move
// of the same task cannot race past the gates. The triage outcome is
// stable enough across the gap that running it outside the lock is fine —
// the worst case is rejecting a task someone else just groomed, which the
// retry will fix.
func promoteToReady(taskID string) (string, error) {
	return promoteToReadyWithOptions(taskID, readyOptions{})
}

type readyOptions struct {
	strictWIP bool
}

func promoteToReadyWithOptions(taskID string, opts readyOptions) (string, error) {
	boardDir := cfg.BoardDir

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return "", err
	}
	switch ref.Status {
	case "backlog":
		// happy path
	case "ready":
		fmt.Fprintf(os.Stderr, "%s is already in ready — nothing to do\n", taskID)
		return "", nil
	default:
		return "", fmt.Errorf("tb ready only promotes from backlog; %s is in %s. Use `tb mv` if you really mean to move it.", taskID, ref.Status)
	}

	cwd, _ := os.Getwd()
	t, err := parseTaskRef(ref, cwd)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", taskID, err)
	}
	if reasons := checkNeedsGrooming(ref.Path, t); len(reasons) > 0 {
		return "", fmt.Errorf("%s is not ready — needs grooming: %s. Fix with `tb edit %s` (or `tb triage` to see the full list).", taskID, joinReasons(reasons), taskID)
	}

	if err := enforceWipLimitForMode("ready", boardDir, opts.strictWIP); err != nil {
		return "", err
	}

	guard := composeGuards(expectedSourceGuard("backlog"), wipLimitGuardForMode(opts.strictWIP))
	result, err := moveTaskOnBoardWithGuard(boardDir, taskID, "ready", guard, func(string) string {
		return "Committed — moved to ready"
	})
	if err != nil {
		return "", err
	}
	if result.Noop {
		return "", nil
	}

	// Mirror TB-268's review-fail clear: the commitment point should
	// present a clean cursor so downstream consumers (auto-implement
	// gate, humans inspecting the file) don't see a leftover groom-mode
	// `success` or recovery-stranded `lost` as in-flight implement
	// state. Per-mode attribution (GroomStatus, ImplementStatus, etc.)
	// is preserved.
	if err := clearReadyAgentStatus(boardDir, taskID); err != nil {
		return "", fmt.Errorf("moved %s to ready but failed to clear AgentStatus: %w", taskID, err)
	}
	return fmt.Sprintf("Moved %s from %s to ready", taskID, result.SrcStatus), nil
}

// clearReadyAgentStatus removes the generic **AgentStatus:** line from the
// ready task, under the board lock so a concurrent edit cannot interleave.
// Noop if the field is already absent. Per-mode attribution lines
// (GroomStatus, ImplementStatus, ReviewStatus) are intentionally
// preserved.
func clearReadyAgentStatus(boardDir, taskID string) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, []string{"ready"})
	if err != nil {
		return err
	}
	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}
	lines := strings.Split(string(data), "\n")
	cleared := clearField(lines, "AgentStatus")
	if len(cleared) == len(lines) {
		// No change needed; avoid an unnecessary write.
		return nil
	}
	if err := writeFileAtomic(ref.Path, []byte(strings.Join(cleared, "\n")), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", ref.Path, err)
	}
	return nil
}

// cmdPull is the canonical kanban pull action: move the highest-priority
// oldest task from ready into in-progress. With no argument it picks the
// next candidate automatically; passing an ID overrides the selection but
// still requires the task to currently live in ready.
//
//	tb pull            # auto-select
//	tb pull <ID>       # explicit task
func cmdPull(args []string) {
	var targetID string
	if len(args) >= 1 {
		targetID = normalizeTaskID(args[0])
	}
	msg, err := pullReadyTask(targetID)
	if err != nil {
		fatal("%v", err)
	}
	if msg != "" {
		fmt.Println(msg)
	}
}

// pullReadyTask is the testable core of `tb pull`. Pass "" for auto-select.
// Returns ("", nil) when ready is empty (the CLI wrapper emits a stderr
// hint and exits 0 in that case).
//
// Pre-flight checks (WIP + source-status) run outside the lock for
// fast-fail UX; the same invariants are re-validated via the guard inside
// the lock so concurrent CLI/agent invocations cannot overshoot the
// in-progress limit or pull a task that moved out of `ready` while we
// were composing the move.
func pullReadyTask(targetID string) (string, error) {
	boardDir := cfg.BoardDir

	if err := enforceWipLimit("in-progress", boardDir); err != nil {
		return "", err
	}

	if targetID == "" {
		picked, ok, err := pickNextReadyTask(boardDir)
		if err != nil {
			return "", err
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "Nothing to pull — ready column is empty. Use `tb ready <ID>` to commit a task first.")
			return "", nil
		}
		targetID = picked
	} else {
		ref, err := resolveTaskRef(boardDir, targetID, allStatusDirs)
		if err != nil {
			return "", err
		}
		if ref.Status != "ready" {
			return "", fmt.Errorf("tb pull only accepts tasks in ready; %s is in %s. Use `tb start %s` to override.", targetID, ref.Status, targetID)
		}
	}

	guard := composeGuards(expectedSourceGuard("ready"), wipLimitGuard())
	result, err := moveTaskOnBoardWithGuard(boardDir, targetID, "in-progress", guard, func(string) string {
		return "Pulled into in-progress"
	})
	if err != nil {
		return "", err
	}
	if result.Noop {
		return "", nil
	}
	return fmt.Sprintf("Pulled %s from %s to in-progress", targetID, result.SrcStatus), nil
}

// pickNextReadyTask returns the ID of the highest-priority oldest task in
// ready. Priority sort is the same as `tb ls` (P0 → P3, then unknown last),
// breaking ties by numeric ID ascending (FIFO).
func pickNextReadyTask(boardDir string) (string, bool, error) {
	tasks, err := collectTasks(boardDir, "ready")
	if err != nil {
		return "", false, err
	}
	if len(tasks) == 0 {
		return "", false, nil
	}
	sort.Slice(tasks, func(i, j int) bool {
		pi := priorityRank(tasks[i].Priority)
		pj := priorityRank(tasks[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return numericID(tasks[i].ID) < numericID(tasks[j].ID)
	})
	return tasks[0].ID, true, nil
}

// enforceWipLimit honours `wip_enforcement`: warn (stderr message, allow) or
// strict (return error so the caller aborts before mutating). Statuses
// without a configured limit are unrestricted. The check counts current
// tasks via collectTasks; callers that mutate should call this *before*
// taking the board lock or, if already inside the lock, accept the small
// TOCTOU window (concurrent CLI mutations are serialized by .board.lock).
func enforceWipLimit(status, boardDir string) error {
	return enforceWipLimitForMode(status, boardDir, false)
}

func enforceWipLimitForMode(status, boardDir string, strictOverride bool) error {
	limit, ok := cfg.wipLimitFor(status)
	if !ok {
		return nil
	}
	tasks, err := collectTasks(boardDir, status)
	if err != nil {
		return fmt.Errorf("cannot check WIP limit for %s: %w", status, err)
	}
	if len(tasks) < limit {
		return nil
	}
	if strictOverride || cfg.WipEnforcement == "strict" {
		return fmt.Errorf("WIP limit reached for %s (%d/%d) — strict mode blocks this move. Finish or move a task out before adding another, or change wip_enforcement to 'warn' in .tb.yaml.", status, len(tasks), limit)
	}
	fmt.Fprintf(os.Stderr, "warning: WIP limit reached for %s (%d/%d tasks)\n", status, len(tasks), limit)
	return nil
}

func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	out := reasons[0]
	for _, r := range reasons[1:] {
		out += ", " + r
	}
	return out
}
