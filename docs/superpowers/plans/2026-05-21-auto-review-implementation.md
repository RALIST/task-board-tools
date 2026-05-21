# Auto-Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the opt-in auto-review stage from `docs/superpowers/specs/2026-05-21-auto-review-design.md`.

**Architecture:** Add a managed CLI pass command first, then persist the GUI preference, then add a coordinator beside `AutoGroomCoordinator` and `AutoImplementCoordinator`. Auto-review uses the existing `mode=review` agent lifecycle and only mutates board state through managed CLI pass/fail/user-attention commands.

**Tech Stack:** Go CLI, Wails3 Go services, Svelte 5 frontend, Vitest.

---

## Current Board Snapshot

- `in-progress`: empty.
- `code-review`: `TB-312`, `TB-315`.
- `ready`: `TB-269`, `TB-292`, `TB-293`, `TB-294`, `TB-295`, `TB-301`, `TB-303`, `TB-304`, `TB-305`, `TB-306`, `TB-317`.
- Auto-review implementation tasks are still in `backlog`: `TB-262`, `TB-263`, `TB-264`, `TB-265`, `TB-272`.
- Spec implementation order is `TB-272` -> `TB-263` -> `TB-264` -> `TB-265` -> close `TB-262`.
- Dirty tree before planning: `board/.next-id`, `board/BOARD.md`, `gui/build/config.yml`, untracked `board/backlog/TB-318/`, untracked `board/ready/TB-317/`. Do not stage or rewrite these unless executing the relevant task.

## Assumptions

- Execute one task at a time. Start by moving `TB-272` through the board when implementation begins.
- Do not implement auto-review before `tb review --pass` exists; prompt already references the final command.
- Treat `ReviewRef` as the machine target. `## Review Target` stays advisory.
- Use existing direct `AgentService` coordinator pattern for auto-review, not daemon worker queue.
- No new kanban status and no new `AgentStatus` value.

## File Map

- `cli/review.go`: add `--pass`, `reviewPass`, and locked pass metadata write.
- `cli/review_test.go`: pass-flow tests.
- `cli/main.go`: usage text.
- `gui/internal/cli/mutations.go`: add `ReviewPass`.
- `gui/app/board_service.go`: expose `PassReview`.
- `gui/app/preferences.go`: add `auto_review_enabled`, validation, controller hook.
- `gui/app/preferences_test.go`: defaults, persistence, validation, restart load.
- `gui/adapters.go`, `gui/main.go`: wire `AutoReviewCoordinator`.
- `gui/internal/agent/state.go`: add `InitiatorAutoReview`.
- `gui/app/agent_run.go`: add `ReviewTaskAs`.
- `gui/app/auto_review.go`: new coordinator.
- `gui/app/auto_review_test.go`: coordinator tests.
- `gui/app/stage_reconciler.go`: add conservative auto-review pass/fail reconciliation only if objective markers exist.
- `gui/frontend/src/lib/api.ts`: auto-review bindings + status wrapper.
- `gui/frontend/src/lib/stores/preferences.ts`: add preference field and setter.
- `gui/frontend/src/lib/stores/autoReview.ts`: new status/skip store.
- `gui/frontend/src/routes/+page.svelte`: header toggle and event registration.
- `gui/frontend/src/lib/components/SettingsPanel.svelte`: settings toggle.
- `gui/frontend/src/lib/components/Card.svelte`: review skip/run indicators for code-review cards.
- `gui/frontend/src/lib/components/TaskDrawer.svelte`: review skip state near review controls.
- `gui/frontend/src/**/*.test.ts`: update focused tests.
- `docs/ARCHITECTURE.md`, `docs/FEATURES.md`, `docs/IMPLEMENTATION.md`, `board/CONVENTIONS.md`, `board/SKILL.md`, `cli/templates.go`, `cli/templates_test.go`: docs/templates final sweep.

### Task 1: TB-272 Managed Review Pass

**Files:**
- Modify: `cli/review.go`
- Modify: `cli/review_test.go`
- Modify: `cli/main.go`
- Modify: `gui/internal/cli/mutations.go`
- Modify: `gui/app/board_service.go`

- [ ] **Step 1: Write CLI pass tests**

Add tests to `cli/review_test.go`:

```go
func TestReviewPassMovesToDoneWithFindings(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	withStdin(t, "- No blocking findings.\n", func() {
		if _, err := reviewPass("TB-1", "-"); err != nil {
			t.Fatalf("reviewPass: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(boardDir, "done", "TB-1.md")); err != nil {
		t.Fatalf("task should be in done/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed, got err=%v", err)
	}
	content := readReviewTask(t, boardDir, "done")
	assertContains(t, content, "## Review Findings\n\n- No blocking findings.")
	assertContains(t, content, "Passed code review")
}

func TestReviewPassRejectsNonCodeReview(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "ready", reviewBaseTask)

	var err error
	withStdin(t, "- No blocking findings.\n", func() {
		_, err = reviewPass("TB-1", "-")
	})
	if err == nil || !strings.Contains(err.Error(), "only accepts tasks in code-review") {
		t.Fatalf("reviewPass error = %v, want code-review-only error", err)
	}
	if _, statErr := os.Stat(filepath.Join(boardDir, "ready", "TB-1.md")); statErr != nil {
		t.Fatalf("task should remain ready: %v", statErr)
	}
}

func TestReviewPassRejectsEmptyFindings(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	var err error
	withStdin(t, "\n  \n", func() {
		_, err = reviewPass("TB-1", "-")
	})
	if err == nil || !strings.Contains(err.Error(), "empty after trimming") {
		t.Fatalf("reviewPass error = %v, want empty-content error", err)
	}
	if _, statErr := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); statErr != nil {
		t.Fatalf("task should remain code-review: %v", statErr)
	}
}
```

- [ ] **Step 2: Run failing CLI tests**

Run:

```sh
cd cli && go test ./... -run 'TestReviewPass|TestReviewFailMovesToReadyWithMarker'
```

Expected: fail because `reviewPass` does not exist.

- [ ] **Step 3: Implement `--pass` in `cli/review.go`**

Minimal shape:

```go
passPath := fs.String("pass", "", "pass review: write findings from file|- and move task to done")
```

Include `passPath` in usage, mode counting, exclusivity errors, and switch dispatch.

Add:

```go
func reviewPass(taskID, sourcePath string) (string, error) {
	body, err := readReviewBodyInput(sourcePath, "review findings")
	if err != nil {
		return "", err
	}
	body = redactText(body)

	boardDir := cfg.BoardDir
	if err := reviewWritePassMetadata(boardDir, taskID, body); err != nil {
		return "", err
	}

	result, err := moveTaskOnBoardWithGuard(boardDir, taskID, "done", expectedSourceGuard("code-review"), func(string) string {
		return "Passed code review"
	})
	if err != nil {
		return "", err
	}
	if result.Noop {
		return fmt.Sprintf("%s is already in done — review findings were written, but no move occurred", taskID), nil
	}
	return fmt.Sprintf("Passed review for %s: moved %s -> done", taskID, result.SrcStatus), nil
}

func reviewWritePassMetadata(boardDir, taskID, findingsBody string) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return err
	}
	if ref.Status != "code-review" {
		return fmt.Errorf("tb review --pass only accepts tasks in code-review; %s is in %s", taskID, ref.Status)
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}
	content := upsertTaskSection(string(data), "## Review Findings", findingsBody)
	if err := writeFileAtomic(ref.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", ref.Path, err)
	}
	return nil
}
```

- [ ] **Step 4: Update help and GUI CLI wrapper**

Add `tb review --pass <ID> file|-` to `cli/main.go` usage.

Add to `gui/internal/cli/mutations.go`:

```go
func (c *Client) ReviewPass(ctx context.Context, id, findings string) error {
	if strings.TrimSpace(id) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "review", Stderr: "task id is required"}
	}
	if strings.TrimSpace(findings) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "review", Stderr: "review findings cannot be empty"}
	}
	args := []string{"review", "--pass", "-", id}
	_, err := c.RunWithStdin(ctx, strings.NewReader(findings), args...)
	return wrapMutation("review", args, err)
}
```

Add to `gui/app/board_service.go`:

```go
func (b *BoardService) PassReview(ctx context.Context, id, findings string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.ReviewPass(ctx, id, findings)
}
```

- [ ] **Step 5: Verify and rebuild CLI**

Run:

```sh
cd cli && go test ./...
cd cli && go build -o tb .
```

Expected: pass. Locally relink if execution workflow expects root `tb`:

```sh
cp cli/tb tb
```

Commit:

```sh
git add cli/review.go cli/review_test.go cli/main.go gui/internal/cli/mutations.go gui/app/board_service.go
git commit -m "TB-272 add managed review pass flow"
```

### Task 2: TB-263 Auto-Review Preference And Controls

**Files:**
- Modify: `gui/app/preferences.go`
- Modify: `gui/app/preferences_test.go`
- Modify: `gui/adapters.go`
- Modify: `gui/main.go`
- Modify: `gui/frontend/src/lib/api.ts`
- Modify: `gui/frontend/src/lib/stores/preferences.ts`
- Modify: `gui/frontend/src/lib/stores/preferences.test.ts`
- Modify: `gui/frontend/src/lib/components/SettingsPanel.svelte`
- Modify: `gui/frontend/src/lib/components/SettingsPanel.test.ts`
- Modify: `gui/frontend/src/routes/+page.svelte`

- [ ] **Step 1: Add backend tests**

Add tests covering:
- missing preferences default `GetAutoReviewEnabled() == false`
- `SetAutoReviewEnabled(true)` with `default_agent=none` returns `ErrAutoReviewDefaultAgentRequired`
- successful enable/disable with `default_agent=claude`
- reload from preferences file persists enabled state
- failed validation leaves file unchanged

- [ ] **Step 2: Implement preference field and validation**

In `gui/app/preferences.go`, add:

```go
const AutoReviewEnabledDefault = false

var ErrAutoReviewDefaultAgentRequired = errors.New(
	"auto-review requires Default Agent set to claude or codex")
```

Add field:

```go
AutoReviewEnabled bool `json:"auto_review_enabled"`
```

Add methods:

```go
func (s *SettingsService) GetAutoReviewEnabled() bool {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return AutoReviewEnabledDefault
	}
	return prefs.AutoReviewEnabled
}

func (s *SettingsService) SetAutoReviewEnabled(enabled bool) error {
	if err := s.updatePreferencesWithValidator(func(prefs *Preferences) error {
		if enabled && prefs.DefaultAgent != "claude" && prefs.DefaultAgent != "codex" {
			return ErrAutoReviewDefaultAgentRequired
		}
		prefs.AutoReviewEnabled = enabled
		return nil
	}); err != nil {
		return err
	}
	if controller, ok := s.activator.(AutoReviewController); ok {
		controller.NotifyAutoReviewEnabled()
	}
	return nil
}
```

Add `NotifyDefaultAgentChanged()` to `AutoReviewController`, and notify it from `SetDefaultAgent`.

- [ ] **Step 3: Leave coordinator wiring for Task 3**

Do not add production coordinator wiring in this task. The `AutoReviewController` type assertion in `SettingsService` compiles even when `boardActivator` does not implement it yet; Task 3 adds the concrete coordinator and adapter methods.

If Task 2 and Task 3 land in one branch, keep the preference commit separate from the coordinator commit.

- [ ] **Step 4: Update frontend preferences store**

In `api.ts`, add binding methods:

```ts
GetAutoReviewEnabled: () => Promise<boolean>;
SetAutoReviewEnabled: (enabled: boolean) => Promise<void>;
```

Add wrappers `getAutoReviewEnabled()` and `setAutoReviewEnabled()`.

In `preferences.ts`, add `autoReviewEnabled` to `PreferencesState`, `DEFAULT_STATE`, `loadPreferences()`, `setAutoReviewEnabled()`, and `preferencesStore`.

- [ ] **Step 5: Add UI controls**

In `SettingsPanel.svelte`, add local state/baseline field, dirty check, save block, and warning:

```svelte
let autoReviewInput = $state(false);
let autoReviewNeedsDefaultAgent = $derived(autoReviewInput && defaultAgentInput === 'none');
```

Place toggle near auto-groom/auto-implement:

```svelte
<label class="field checkbox-field">
  <span>Enable auto review</span>
  <input type="checkbox" data-testid="auto-review-toggle" bind:checked={autoReviewInput} />
  <small>
    When on, the GUI reviews code-review tasks with ReviewRef using the default agent.
  </small>
</label>

{#if autoReviewNeedsDefaultAgent}
  <p class="inline-warning" role="alert">
    Set a default agent before auto-review can run.
  </p>
{/if}
```

In `+page.svelte`, add header pill:

```svelte
let autoReviewEnabled = $derived($preferencesStore.autoReviewEnabled);
let autoReviewMissingPrereqs = $derived($preferencesStore.defaultAgent === 'none');
```

Toggle behavior mirrors auto-implement minus query: open Settings when enabling without default agent; allow disable.

- [ ] **Step 6: Verify**

Run:

```sh
cd gui && go test ./...
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

Commit:

```sh
git add gui/app/preferences.go gui/app/preferences_test.go gui/adapters.go gui/main.go gui/frontend/src/lib/api.ts gui/frontend/src/lib/stores/preferences.ts gui/frontend/src/lib/stores/preferences.test.ts gui/frontend/src/lib/components/SettingsPanel.svelte gui/frontend/src/lib/components/SettingsPanel.test.ts gui/frontend/src/routes/+page.svelte
git commit -m "TB-263 add auto-review preference controls"
```

### Task 3: TB-264 Auto-Review Coordinator

**Files:**
- Create: `gui/app/auto_review.go`
- Create: `gui/app/auto_review_test.go`
- Modify: `gui/app/agent_run.go`
- Modify: `gui/internal/agent/state.go`
- Modify: `gui/app/board_service.go`
- Modify: `gui/internal/cli/mutations.go`
- Modify: `gui/adapters.go`
- Modify: `gui/main.go`

- [ ] **Step 1: Add initiator and agent entry point tests**

Add test in `gui/app/agent_run_test.go` that `ReviewTaskAs(ctx, "TB-1", agent.InitiatorAutoReview)` queues `mode=review` and `initiator=auto-review`.

- [ ] **Step 2: Add `InitiatorAutoReview` and `ReviewTaskAs`**

In `gui/internal/agent/state.go`:

```go
InitiatorAutoReview = "auto-review"
```

In `gui/app/agent_run.go`:

```go
func (s *AgentService) ReviewTaskAs(ctx context.Context, id, initiator string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeReview, "", initiator)
}
```

- [ ] **Step 3: Create coordinator skeleton**

`gui/app/auto_review.go` should mirror `AutoImplementCoordinator` shape:

```go
type AutoReviewStatus struct {
	Enabled           bool              `json:"enabled"`
	DefaultAgent      string            `json:"default_agent"`
	NeedsDefaultAgent bool              `json:"needs_default_agent"`
	LastScanAt        string            `json:"last_scan_at,omitempty"`
	LastSkipReasons   map[string]string `json:"last_skip_reasons,omitempty"`
}

type AutoReviewCoordinator struct {
	board    *BoardService
	agent    *AgentService
	settings *SettingsService
	emitter  Emitter
	logger   *slog.Logger
	budget   AutomationWorkerBudget
	now      func() time.Time

	mu             sync.Mutex
	boardDir       string
	activated      bool
	lastNeedsDef   bool
	lastScanAt     time.Time
	lastSkip       map[string]string
	debounceTimer  *time.Timer
	resumeAttempts map[string]time.Time
	closed         bool
}

type AutoReviewCoordinatorOptions struct {
	Board        *BoardService
	Agent        *AgentService
	Settings     *SettingsService
	Emitter      Emitter
	Logger       *slog.Logger
	WorkerBudget AutomationWorkerBudget
	Now          func() time.Time
}
```

Add `Activate`, `Deactivate`, `Emit`, `Status`, `NotifyAutoReviewEnabled`, `NotifyDefaultAgentChanged`, `NotifyWorkerBudgetChanged`, `recordSkip`, `transitionNeedsDefault`, and `remainingWorkerCapacity` using auto-implement names with `auto-review:*` events.

- [ ] **Step 4: Candidate gates**

Implement scan rules:

```go
func autoReviewGateBlocker(t Task) string {
	switch t.AgentStatus {
	case "queued", "running", "needs-user", "cancelled":
		return "agent-status " + t.AgentStatus
	case "interrupted", "lost":
		return "agent-status " + t.AgentStatus
	}
	switch t.ReviewStatus {
	case "queued", "running":
		return "review-status " + t.ReviewStatus
	}
	if strings.TrimSpace(t.ReviewRef) == "" {
		return "missing ReviewRef"
	}
	return ""
}
```

Special case: `AgentStatus=success` from implement must not block first review.

- [ ] **Step 5: Missing ReviewRef handoff**

For code-review tasks with missing `ReviewRef`, write `## User Attention` then set `AgentStatus=needs-user` through `BoardService.EditTask` / CLI edit:

```markdown
Reason: missing review target.
Question/Action: set ReviewRef to a branch, PR URL, commit SHA, worktree path, or other machine-readable target.
Attempted context: auto-review found this task in code-review but could not determine a safe target.
Unblock condition: set ReviewRef, then clear AgentStatus with `tb edit <ID> --agent-status none`.
```

Do this once per task/status epoch; do not repeatedly rewrite the task on every scan.

- [ ] **Step 6: Submission-epoch dedupe**

Use JSONL history, not in-memory state. Add helpers in `gui/internal/agent/state.go` or `gui/app/auto_review.go`:

```go
func autoReviewFingerprint(t Task, body string) string {
	return hashString(strings.Join([]string{
		t.ID,
		t.ReviewRef,
		markdownSectionBodyText(body, "## Review Target"),
		lastCodeReviewLogEntry(body),
	}, "\x00"))
}
```

Queued auto-review events need the fingerprint. Prefer adding `ReviewFingerprint string` to `agent.Event` only if accepted by existing JSONL schema conventions; otherwise write a deterministic `Target` field for review runs if existing event schema already treats it generically. Dedupe rule:
- skip if latest auto-review queued/finished event for same fingerprint exists in current code-review epoch
- allow if `ReviewRef` changes
- allow after resubmit because `Submitted to code-review` log timestamp/entry changes

- [ ] **Step 7: Start eligible review**

For each eligible code-review task:
- respect explicit `Agent`
- if blank, write default agent with `Edit`
- validate runner via `runnerFor`
- reserve worker budget through `AgentService` by calling `ReviewTaskAs`
- emit `auto-review:queued`

Call:

```go
runID, err := c.agent.ReviewTaskAs(ctx, task.ID, agent.InitiatorAutoReview)
```

- [ ] **Step 8: Recovery sweep**

Mirror auto-implement but over `snap.CodeReview`:
- `interrupted` + latest queued initiator `auto-review` -> `ResumeAgentAs(ctx, id, agent.InitiatorAutoReview)`
- `lost` + latest queued initiator `auto-review` -> `ReviewTaskAs(ctx, id, agent.InitiatorAutoReview)`
- user initiator -> skip
- worker budget and cooldown apply

- [ ] **Step 9: Wire production**

In `gui/main.go`:
- construct `autoReview`
- include in watcher tee
- add Wails service
- call `autoReview.SetSettings(settingsService)`
- add terminal hook if coordinator needs event-driven scan after run finish

In `gui/adapters.go`:
- add field
- activate/deactivate sequence after daemon, auto-groom, auto-implement
- notify default-agent, max-workers, auto-review enabled

- [ ] **Step 10: Verify**

Run:

```sh
cd gui && go test ./...
```

Commit:

```sh
git add gui/app/auto_review.go gui/app/auto_review_test.go gui/app/agent_run.go gui/internal/agent/state.go gui/app/board_service.go gui/internal/cli/mutations.go gui/adapters.go gui/main.go
git commit -m "TB-264 enqueue auto-review runs"
```

### Task 4: TB-265 Frontend Runtime State

**Files:**
- Create: `gui/frontend/src/lib/stores/autoReview.ts`
- Create: `gui/frontend/src/lib/stores/autoReview.test.ts`
- Modify: `gui/frontend/src/lib/api.ts`
- Modify: `gui/frontend/src/routes/+page.svelte`
- Modify: `gui/frontend/src/lib/components/Card.svelte`
- Modify: `gui/frontend/src/lib/components/TaskDrawer.svelte`
- Modify: focused component tests

- [ ] **Step 1: Add status binding wrapper**

In `api.ts`, import generated `Status as AutoReviewStatusBinding` and export:

```ts
export async function getAutoReviewStatus(): Promise<AutoReviewStatus> {
	return await AutoReviewStatusBinding();
}
```

- [ ] **Step 2: Add `autoReview` store**

Mirror `autoGroom.ts`:

```ts
export interface AutoReviewState {
	enabled: boolean;
	defaultAgent: string;
	needsDefaultAgent: boolean;
	lastScanAt: string;
	lastSkipReasons: Record<string, string>;
	loaded: boolean;
}
```

Register events:
- `auto-review:needs-default-agent`
- `auto-review:default-agent-cleared`
- `auto-review:queued`
- `auto-review:skipped`
- `auto-review:resumed`
- `auto-review:resume-failed`
- `auto-review:scan-complete`

- [ ] **Step 3: Register store in `+page.svelte`**

Call `refreshAutoReview()` on mount and register event handlers. Header toggle should use preference state plus coordinator status for missing-default warnings.

- [ ] **Step 4: Render card/drawer feedback**

Card:
- code-review + latest review run queued/running -> show compact `R` per-action status chip
- code-review + skip reason -> show small warning chip with title

Drawer:
- show auto-review enabled row near Review button
- show missing default, missing ReviewRef, duplicate, active run, needs-user, worker capacity messages from `lastSkipReasons`
- keep manual `Review` button visible when skipped/off

- [ ] **Step 5: Verify**

Run:

```sh
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

Commit:

```sh
git add gui/frontend/src/lib/api.ts gui/frontend/src/lib/stores/autoReview.ts gui/frontend/src/lib/stores/autoReview.test.ts gui/frontend/src/routes/+page.svelte gui/frontend/src/lib/components/Card.svelte gui/frontend/src/lib/components/TaskDrawer.svelte gui/frontend/src/lib/**/*.test.ts
git commit -m "TB-265 surface auto-review state"
```

### Task 5: TB-262 Docs, Templates, Full Verification

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/FEATURES.md`
- Modify: `docs/IMPLEMENTATION.md`
- Modify: `board/CONVENTIONS.md`
- Modify: `board/SKILL.md`
- Modify: `cli/templates.go`
- Modify: `cli/templates_test.go`
- Modify: `gui/internal/agent/prompts/review.md`

- [ ] **Step 1: Remove TB-272 fallback wording**

In `gui/internal/agent/prompts/review.md`, delete temporary pass fallback block. Keep one pass path:

```sh
tb review --pass {{TASK_ID}} - <<'EOF'
- No blocking findings.
EOF
```

- [ ] **Step 2: Update docs**

Docs must state:
- `auto_review_enabled` default false
- valid default agent required
- missing `ReviewRef` -> `needs-user`
- pass -> `done` through `tb review --pass`
- fail -> `ready` through `tb review --fail`
- no semantic inference from prose
- recovery only for JSONL queued initiator `auto-review`

- [ ] **Step 3: Fix known stale docs line**

`docs/IMPLEMENTATION.md` old TB-194 log still says fail moves to backlog. Update that historical paragraph with a superseding note or corrected wording so source-of-truth docs no longer contradict ready-based review failure.

- [ ] **Step 4: Update generated templates**

In `cli/templates.go`, add `tb review --pass` to minimal commands and review loop. Update `cli/templates_test.go` expected content if tests pin generated docs.

- [ ] **Step 5: Run full verification**

Run:

```sh
cd cli && go test ./...
cd gui && go test ./...
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

Expected: all pass. If Wails bindings changed, run the repo's binding generation command used locally and include generated binding files in the same task commit.

- [ ] **Step 6: Board closure**

Move tasks honestly:
- `TB-272` done after Task 1 commit and evidence.
- `TB-263` done after Task 2 commit and evidence.
- `TB-264` done after Task 3 commit and evidence.
- `TB-265` done after Task 4 commit and evidence.
- `TB-262` done only after Task 5 verification plus manual smoke notes or explicit deferral captured in task log.

Commit docs:

```sh
git add docs/ARCHITECTURE.md docs/FEATURES.md docs/IMPLEMENTATION.md board/CONVENTIONS.md board/SKILL.md cli/templates.go cli/templates_test.go gui/internal/agent/prompts/review.md
git commit -m "TB-262 document auto-review completion"
```

## Review Risks

- `reviewPass` writes findings under one lock, then moves under another. This matches existing `reviewFail`; if stricter atomicity is required, refactor both pass and fail into a single locked mutation instead of changing pass alone.
- Dedupe fingerprint must use objective state only. Do not infer from `## Review Findings` prose.
- `AgentStatus=success` from implement is allowed for first review, but `cancelled`, `needs-user`, `queued`, `running`, unresolved `interrupted`, and unrelated `lost` remain blockers.
- Missing `ReviewRef` handoff mutates the task; validation failures for missing default agent must not.
- Frontend must keep manual Review visible. Auto-review is an opt-in helper, not a replacement for human review.

## Final Verification Checklist

- [ ] `cd cli && go test ./...`
- [ ] `cd cli && go build -o tb .`
- [ ] `cd gui && go test ./...`
- [ ] `cd gui/frontend && npm run check`
- [ ] `cd gui/frontend && npm test -- --run`
- [ ] Manual smoke: human review, auto-review pass, auto-review fail, cancel during review, app restart with queued/running review, toggle while code-review tasks exist.
