# Auto-Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the opt-in auto-review stage that reviews `code-review` tasks, moves clean work to `done`, and returns blocking findings to `ready` with `review-failed`.

**Architecture:** Add the missing managed CLI pass flow first, then expose `auto_review_enabled`, then add an `AutoReviewCoordinator` beside auto-groom and auto-implement. The coordinator selects only `code-review` tasks, starts `mode=review` runs with `initiator=auto-review`, writes `needs-user` for missing `ReviewRef`, and uses durable submission-epoch dedupe instead of prose inference.

**Tech Stack:** Go CLI and Wails backend, Svelte 5 frontend, existing markdown board format, existing JSONL agent state, existing `tb` CLI mutation wrappers, Vitest, `svelte-check`.

**Spec:** [docs/superpowers/specs/2026-05-21-auto-review-design.md](../specs/2026-05-21-auto-review-design.md)

---

## Current Board Preflight

The auto-review implementation cards are still in backlog: `TB-262`, `TB-263`, `TB-264`, `TB-265`, and `TB-272`. Before executing code work through the canonical board flow, promote the first owned card with `./cli/tb ready TB-272` or explicitly accept a push-style override. The current strict board pull target is `TB-269`, because it is the only P1 ready card and is tagged `review-failed`; that is separate from this auto-review spec.

Ready is currently over its configured limit (`11/10`, warn mode). `in-progress` is empty. `code-review` has `TB-312` and `TB-315`.

Unrelated dirty tree exists and must stay out of auto-review commits unless the user explicitly owns it: `board/.next-id`, `board/BOARD.md`, `gui/build/config.yml`, `board/ready/TB-317/`, and `board/backlog/TB-318/`.

## File Structure

**CLI pass flow**
- Modify: `cli/review.go` — add `--pass`, `reviewPass`, and pass metadata helper.
- Modify: `cli/review_test.go` — add pass tests and keep fail regression.
- Modify: `cli/main.go` — add `tb review --pass` help text.
- Modify: `gui/internal/agent/prompts/review.md` — remove temporary pass fallback after `--pass` exists.
- Modify: `gui/internal/agent/runner_test.go` — assert the review prompt names `tb review --pass` and no longer names the temporary fallback.

**Backend settings and bindings**
- Modify: `gui/app/preferences.go` — add persisted `AutoReviewEnabled`, default, getter, setter, typed error.
- Modify: `gui/app/preferences_test.go` — cover default, round-trip, validation failure, restart load.
- Modify: `gui/app/settings_service.go` — add the `AutoReviewController` runtime settings hook beside `AutoGroomController`.
- Modify: `gui/main.go` — construct and register `AutoReviewCoordinator`.
- Modify: `gui/adapters.go` — activate/deactivate/notify auto-review alongside other coordinators.
- Regenerate: `gui/frontend/bindings/tools/tb-gui/app/*` after Wails service changes.

**Backend coordinator**
- Create: `gui/app/auto_review.go` — coordinator, status snapshot, candidate scan, skip ledger, recovery, dedupe.
- Create: `gui/app/auto_review_test.go` — integration-style coordinator tests.
- Modify: `gui/app/agent_run.go` — add `ReviewTaskAs(ctx, id, initiator)` and use it for auto-review.
- Modify: `gui/internal/agent/state.go` — add `InitiatorAutoReview`; document queued schema initiator list.
- Modify: `gui/internal/cli/mutations.go` — add `ReviewPass` and `SetUserAttention` wrappers.
- Modify: `gui/app/board_service.go` — expose `PassReview` and `SetUserAttention` service methods.

**Frontend**
- Modify: `gui/frontend/src/lib/api.ts` — add settings wrappers and `AutoReviewCoordinator.Status`.
- Modify: `gui/frontend/src/lib/stores/preferences.ts` and `preferences.test.ts` — load and set `autoReviewEnabled`.
- Create: `gui/frontend/src/lib/stores/autoReview.ts` and test — mirror `autoGroom` status/skip store.
- Modify: `gui/frontend/src/lib/components/SettingsPanel.svelte` and test — add auto-review toggle and missing-default feedback.
- Modify: `gui/frontend/src/routes/+page.svelte` — add compact header toggle and event handler registration.
- Modify: `gui/frontend/src/lib/components/Card.svelte` and test — show review skip/queued state on code-review cards.
- Modify: `gui/frontend/src/lib/components/TaskDrawer.svelte` and test — show skip reasons, preserve manual Review fallback, refresh pass/fail outcomes.

**Docs closeout**
- Modify: `docs/ARCHITECTURE.md`, `docs/FEATURES.md`, `docs/IMPLEMENTATION.md`, `board/CONVENTIONS.md`, `board/SKILL.md` only where current state changes from planned to shipped.
- Modify: `cli/templates.go` and `cli/templates_test.go` to keep generated board guidance in sync with `board/CONVENTIONS.md` and `board/SKILL.md`.

---

### Task 1: Implement `TB-272` Managed Review Pass

**Files:**
- Modify: `cli/review.go`
- Modify: `cli/review_test.go`
- Modify: `cli/main.go`
- Modify: `gui/internal/agent/prompts/review.md`
- Modify: `gui/internal/agent/runner_test.go`

- [ ] **Step 1: Write failing CLI pass tests**

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
	if !strings.Contains(content, "## Review Findings\n\n- No blocking findings.") {
		t.Fatalf("expected pass findings section, got:\n%s", content)
	}
	if !strings.Contains(content, "Passed code review") {
		t.Fatalf("expected pass log entry, got:\n%s", content)
	}
}

func TestReviewPassRejectsNonCodeReview(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "ready", reviewBaseTask)

	var err error
	withStdin(t, "- No blocking findings.\n", func() {
		_, err = reviewPass("TB-1", "-")
	})
	if err == nil || !strings.Contains(err.Error(), "only accepts tasks in code-review") {
		t.Fatalf("expected code-review-only error, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(boardDir, "ready", "TB-1.md")); statErr != nil {
		t.Fatalf("task should stay in ready/: %v", statErr)
	}
}

func TestReviewPassRejectsEmptyFindings(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	var err error
	withStdin(t, "   \n\n", func() {
		_, err = reviewPass("TB-1", "-")
	})
	if err == nil || !strings.Contains(err.Error(), "empty after trimming") {
		t.Fatalf("expected empty content error, got: %v", err)
	}
}

func TestReviewPassFolderFormMovesToDone(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := filepath.Join(boardDir, "code-review", "TB-1")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("mkdir task folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte(reviewBaseTask), 0644); err != nil {
		t.Fatalf("write folder task: %v", err)
	}

	withStdin(t, "- No blocking findings.\n", func() {
		if _, err := reviewPass("TB-1", "-"); err != nil {
			t.Fatalf("reviewPass: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(boardDir, "done", "TB-1", "TASK.md")); err != nil {
		t.Fatalf("folder task should move to done/: %v", err)
	}
}
```

- [ ] **Step 2: Run failing tests**

```bash
cd cli && go test ./... -run 'TestReviewPass|TestReviewFailMovesToReadyWithMarker' -count=1
```

Expected: compile fails because `reviewPass` is undefined.

- [ ] **Step 3: Add `--pass` command surface**

In `cli/review.go`, add a `passPath` flag, usage line, mode counting, error text, and switch case:

```go
passPath := fs.String("pass", "", "pass review: write findings from file|- and move task to done")
```

Mode counting includes:

```go
if *passPath != "" {
	modeCount++
}
```

The no-mode and multi-mode error strings should include `--pass`.

Switch case:

```go
case *passPath != "":
	if msg, err := reviewPass(taskID, *passPath); err != nil {
		fatal("%v", err)
	} else {
		fmt.Println(msg)
	}
```

- [ ] **Step 4: Implement pass metadata helper and flow**

Add near `reviewFail` in `cli/review.go`:

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

	result, err := moveTaskOnBoardWithGuard(boardDir, taskID, "done",
		expectedSourceGuard("code-review"),
		func(string) string { return "Passed code review — moved to done" },
	)
	if err != nil {
		return "", err
	}
	if result.Noop {
		return fmt.Sprintf("%s is already in done — review pass findings were written, but no move occurred", taskID), nil
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

This matches the existing `reviewFail` two-step pattern while the guarded move rechecks the source under the move lock.

- [ ] **Step 5: Update CLI help**

In `cli/main.go`, add:

```text
tb review --pass <ID> file|-                                           Pass review: write findings, move to done
```

- [ ] **Step 6: Collapse review prompt pass guidance**

In `gui/internal/agent/prompts/review.md`, remove the temporary fallback block and leave only:

```sh
tb review --pass {{TASK_ID}} - <<'EOF'
- No blocking findings.
EOF
```

- [ ] **Step 7: Run CLI verification and rebuild binary**

```bash
cd cli && go test ./...
cd cli && go build -o tb .
```

Expected: all CLI tests pass. The rebuilt `cli/tb` exists; do not stage unrelated board churn.

- [ ] **Step 8: Commit TB-272**

```bash
git add cli/review.go cli/review_test.go cli/main.go gui/internal/agent/prompts/review.md gui/internal/agent/runner_test.go cli/tb
git commit -m "TB-272 add managed review pass flow"
```

If `cli/tb` is ignored or intentionally untracked, omit it from `git add` but still rebuild it locally.

---

### Task 2: Implement `TB-263` Auto-Review Preference And Controls

**Files:**
- Modify: `gui/app/preferences.go`
- Modify: `gui/app/preferences_test.go`
- Modify: `gui/app/settings_service.go`
- Modify: `gui/frontend/src/lib/api.ts`
- Modify: `gui/frontend/src/lib/stores/preferences.ts`
- Modify: `gui/frontend/src/lib/stores/preferences.test.ts`
- Modify: `gui/frontend/src/lib/components/SettingsPanel.svelte`
- Modify: `gui/frontend/src/lib/components/SettingsPanel.test.ts`
- Modify: `gui/frontend/src/routes/+page.svelte`
- Regenerate: `gui/frontend/bindings/tools/tb-gui/app/settingsservice*`

- [ ] **Step 1: Write backend preference tests**

Add to `gui/app/preferences_test.go`:

```go
func TestAutoReviewEnabled_DefaultFalse(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if got := s.GetAutoReviewEnabled(); got != false {
		t.Fatalf("auto_review_enabled default = %v, want false", got)
	}
}

func TestSetAutoReviewEnabled_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("codex"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoReviewEnabled(true); err != nil {
		t.Fatalf("SetAutoReviewEnabled: %v", err)
	}
	if got := s.GetAutoReviewEnabled(); !got {
		t.Fatalf("after set: got false, want true")
	}
	s2 := NewSettingsService(SettingsOptions{Logger: slog.Default(), PrefsPath: path})
	if got := s2.GetAutoReviewEnabled(); !got {
		t.Fatalf("fresh read: got false, want true")
	}
}

func TestSetAutoReviewEnabled_RequiresDefaultAgent(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetAutoReviewEnabled(true); !errors.Is(err, ErrAutoReviewDefaultAgentRequired) {
		t.Fatalf("SetAutoReviewEnabled error = %v, want ErrAutoReviewDefaultAgentRequired", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read prefs: %v", err)
	}
	if strings.Contains(string(data), "auto_review_enabled") {
		t.Fatalf("validation failure should not persist auto_review_enabled, got:\n%s", data)
	}
}
```

- [ ] **Step 2: Run failing backend tests**

```bash
cd gui && go test ./app/... -run 'TestAutoReview|TestSetAutoReview' -count=1
```

Expected: compile fails for missing getter/setter/error.

- [ ] **Step 3: Add backend preference field and validation**

In `gui/app/preferences.go`, add:

```go
const AutoReviewEnabledDefault = false

var ErrAutoReviewDefaultAgentRequired = errors.New(
	"auto-review requires Default Agent set to claude or codex")
```

Add to `Preferences`:

```go
AutoReviewEnabled bool `json:"auto_review_enabled"`
```

Add to `defaultPreferences`:

```go
AutoReviewEnabled: AutoReviewEnabledDefault,
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
		if enabled && !isValidDefaultAgent(prefs.DefaultAgent) {
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

Add interface:

```go
type AutoReviewController interface {
	NotifyAutoReviewEnabled()
	NotifyDefaultAgentChanged()
}
```

Also notify auto-review from `SetDefaultAgent` after the auto-implement notification.

- [ ] **Step 4: Run backend preference tests**

```bash
cd gui && go test ./app/... -run 'TestAutoReview|TestSetAutoReview|TestPreferences_MissingFileReturnsDefaults' -count=1
```

Expected: pass.

- [ ] **Step 5: Add frontend API and preferences store tests**

Update `gui/frontend/src/lib/stores/preferences.test.ts` mocks and default expectations to include:

```ts
const getAutoReviewEnabled = vi.fn<() => Promise<boolean>>();
const setAutoReviewEnabled = vi.fn<(enabled: boolean) => Promise<void>>();
```

Add a test:

```ts
it('loads and writes auto-review enabled', async () => {
  getAutoReviewEnabled.mockResolvedValue(true);
  await loadPreferences();
  expect(get(preferencesStore)).toMatchObject({ autoReviewEnabled: true });

  await storeSetAutoReviewEnabled(false);
  expect(setAutoReviewEnabled).toHaveBeenCalledWith(false);
  expect(get(preferencesStore)).toMatchObject({ autoReviewEnabled: false });
});
```

- [ ] **Step 6: Implement frontend store/API field**

In `gui/frontend/src/lib/api.ts`, extend `SettingsServiceBindings` and add:

```ts
export async function getAutoReviewEnabled(): Promise<boolean> {
  return await requireSettingsMethod('GetAutoReviewEnabled')();
}

export async function setAutoReviewEnabled(enabled: boolean): Promise<void> {
  await requireSettingsMethod('SetAutoReviewEnabled')(enabled);
}
```

In `preferences.ts`, add `autoReviewEnabled` to state, load `getAutoReviewEnabled()`, and add:

```ts
export async function setAutoReviewEnabled(value: boolean): Promise<void> {
  await optimisticWrite('autoReviewEnabled', value, 'auto-review', () =>
    apiSetAutoReviewEnabled(value),
  );
}
```

- [ ] **Step 7: Add SettingsPanel toggle**

In `SettingsPanel.svelte`, add `autoReviewInput` to editable state, dirty detection, reset logic, and save logic. Render it near auto-groom/auto-implement:

```svelte
<label class="toggle-row" class:blocked={autoReviewNeedsDefaultAgent}>
  <input type="checkbox" bind:checked={autoReviewInput} />
  <span>Auto-review</span>
</label>
{#if autoReviewNeedsDefaultAgent}
  <p class="field-hint error">Choose claude or codex as Default Agent before enabling auto-review.</p>
{/if}
```

Use the existing visual pattern and class names already present for auto-groom/auto-implement instead of inventing a new block.

- [ ] **Step 8: Add board-header compact toggle**

In `routes/+page.svelte`, add derived state and handler:

```ts
let autoReviewEnabled = $derived($preferencesStore.autoReviewEnabled);
let autoReviewNeedsDefaultAgent = $derived($preferencesStore.defaultAgent === 'none');

async function toggleAutoReview() {
  if (autoReviewToggleBusy) return;
  if (autoReviewNeedsDefaultAgent && !autoReviewEnabled) {
    pushToast('Choose claude or codex as Default Agent before enabling auto-review');
    return;
  }
  autoReviewToggleBusy = true;
  try {
    await preferencesStore.setAutoReviewEnabled(!autoReviewEnabled);
  } finally {
    autoReviewToggleBusy = false;
  }
}
```

Render beside the other automation toggles with a compact label and `aria-pressed`.

- [ ] **Step 9: Regenerate bindings**

```bash
cd gui && wails3 generate bindings -ts
```

Expected: generated settings bindings/model types include `GetAutoReviewEnabled` and `SetAutoReviewEnabled`.

- [ ] **Step 10: Verify TB-263**

```bash
cd gui && go test ./...
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

Expected: all pass.

- [ ] **Step 11: Commit TB-263**

```bash
git add gui/app/preferences.go gui/app/preferences_test.go gui/app/settings_service.go gui/main.go gui/adapters.go gui/frontend/src/lib/api.ts gui/frontend/src/lib/stores/preferences.ts gui/frontend/src/lib/stores/preferences.test.ts gui/frontend/src/lib/components/SettingsPanel.svelte gui/frontend/src/lib/components/SettingsPanel.test.ts gui/frontend/src/routes/+page.svelte gui/frontend/bindings
git commit -m "TB-263 add auto-review preference controls"
```

---

### Task 3: Implement `TB-264` AutoReviewCoordinator

**Files:**
- Create: `gui/app/auto_review.go`
- Create: `gui/app/auto_review_test.go`
- Modify: `gui/app/agent_run.go`
- Modify: `gui/internal/agent/state.go`
- Modify: `gui/internal/cli/mutations.go`
- Modify: `gui/app/board_service.go`
- Modify: `gui/main.go`
- Modify: `gui/adapters.go`
- Regenerate: `gui/frontend/bindings/tools/tb-gui/app/*`

- [ ] **Step 1: Write coordinator fixture and disabled/no-default tests**

Create `gui/app/auto_review_test.go` mirroring the `autoImplementFixture`, but seed `code-review` tasks with `ReviewRef`.

First tests:

```go
func TestAutoReviewCoordinator_DisabledNoMutation(t *testing.T) {
	f := newAutoReviewFixture(t, "claude", []reviewTaskSpec{{ID: "TB-1", ReviewRef: "feat/x"}}, nil)
	if err := f.settings.SetAutoReviewEnabled(false); err != nil {
		t.Fatalf("SetAutoReviewEnabled: %v", err)
	}

	f.coord.scan(context.Background(), f.boardDir)

	assertNoQueuedEvents(t, f.boardDir, "TB-1")
	content := readTaskFile(t, filepath.Join(f.boardDir, "code-review", "TB-1.md"))
	if strings.Contains(content, "AgentStatus") {
		t.Fatalf("disabled auto-review should not mutate task:\n%s", content)
	}
}

func TestAutoReviewCoordinator_NoDefaultNoMutation(t *testing.T) {
	f := newAutoReviewFixture(t, "claude", []reviewTaskSpec{{ID: "TB-1", ReviewRef: "feat/x"}}, nil)
	mustSetAutoReviewEnabledForTest(t, f.settings, true)

	f.coord.scan(context.Background(), f.boardDir)

	if got := f.coord.Status(); !got.NeedsDefaultAgent {
		t.Fatalf("NeedsDefaultAgent = false, want true")
	}
	assertNoQueuedEvents(t, f.boardDir, "TB-1")
}
```

Use a test-only helper to seed enabled preferences if validation blocks `default_agent=none`, or set default then clear it before scan.

- [ ] **Step 2: Add `InitiatorAutoReview` and `ReviewTaskAs` failing tests**

Add tests expecting queued JSONL rows with:

```json
{"mode":"review","initiator":"auto-review"}
```

Run:

```bash
cd gui && go test ./app/... -run 'TestAutoReviewCoordinator' -count=1
```

Expected: compile fails for missing coordinator.

- [ ] **Step 3: Add agent initiator and review entry point**

In `gui/internal/agent/state.go`, add:

```go
InitiatorAutoReview = "auto-review"
```

In `gui/app/agent_run.go`, add:

```go
func (s *AgentService) ReviewTaskAs(ctx context.Context, id, initiator string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeReview, "", initiator)
}
```

- [ ] **Step 4: Add CLI wrappers for pass and user attention**

In `gui/internal/cli/mutations.go`, add:

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

func (c *Client) SetUserAttention(ctx context.Context, id, body string) error {
	if strings.TrimSpace(id) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "edit", Stderr: "task id is required"}
	}
	if strings.TrimSpace(body) == "" {
		return &MutationError{Kind: ErrKindValidation, Op: "edit", Stderr: "user attention cannot be empty"}
	}
	args := []string{"edit", id, "--user-attention", "-", "--agent-status", "needs-user"}
	_, err := c.RunWithStdin(ctx, strings.NewReader(body), args...)
	return wrapMutation("edit", args, err)
}
```

In `BoardService`, expose `PassReview` and `SetUserAttention` wrappers if coordinator uses the service instead of raw client snapshot.

- [ ] **Step 5: Implement `AutoReviewStatus` and coordinator skeleton**

In `gui/app/auto_review.go`, define:

```go
type AutoReviewStatus struct {
	Enabled           bool              `json:"enabled"`
	DefaultAgent      string            `json:"default_agent"`
	NeedsDefaultAgent bool              `json:"needs_default_agent"`
	LastScanAt        string            `json:"last_scan_at,omitempty"`
	LastSkipReasons   map[string]string `json:"last_skip_reasons,omitempty"`
}
```

Mirror `AutoImplementCoordinator` lifecycle methods:

```go
func NewAutoReviewCoordinator(opts AutoReviewCoordinatorOptions) *AutoReviewCoordinator
func (c *AutoReviewCoordinator) SetSettings(s *SettingsService)
func (c *AutoReviewCoordinator) Activate(ctx context.Context, boardDir string) error
func (c *AutoReviewCoordinator) Deactivate() error
func (c *AutoReviewCoordinator) NotifyAutoReviewEnabled()
func (c *AutoReviewCoordinator) NotifyDefaultAgentChanged()
func (c *AutoReviewCoordinator) NotifyWorkerBudgetChanged()
func (c *AutoReviewCoordinator) Emit(name string, _ ...any)
func (c *AutoReviewCoordinator) Status() AutoReviewStatus
```

- [ ] **Step 6: Implement candidate gates**

Candidate scan rules in `scan`:

```go
if !settingsEnabled {
	transitionNeedsDefault(false)
	emit("auto-review:scan-complete", map[string]any{})
	return
}
if !isValidDefaultAgent(defaultAgent) {
	transitionNeedsDefault(true)
	emit("auto-review:scan-complete", map[string]any{})
	return
}
```

Load board and iterate `snap.CodeReview`. Skip:

```go
AgentStatus == "queued"
AgentStatus == "running"
AgentStatus == "needs-user"
AgentStatus == "cancelled"
AgentStatus == "interrupted" // handled in recovery branch only
AgentStatus == "lost"        // handled in recovery branch only
agent.HasActiveRun(id)
unsupported explicit Agent
duplicate current epoch
worker capacity full
```

Do not block on `AgentStatus == "success"` from prior implement run.

- [ ] **Step 7: Missing `ReviewRef` handoff**

For otherwise in-scope `code-review` task with empty normalized `ReviewRef`, write user attention:

```markdown
Reason: missing review target.

Question/Action: Set ReviewRef to a concrete branch, PR URL, commit SHA, worktree path, or other machine-readable target.

Attempted context: Auto-review found the task in code-review but could not determine a safe target.

Unblock condition: Set ReviewRef, then clear AgentStatus with `tb edit <ID> --agent-status none`.
```

Then set `AgentStatus: needs-user` through one managed edit wrapper. Emit:

```go
c.recordSkip(task.ID, "missing ReviewRef")
c.emit("auto-review:needs-user", map[string]any{"task_id": task.ID, "reason": "missing ReviewRef"})
```

- [ ] **Step 8: Add submission-epoch dedupe**

Implement a helper that derives a fingerprint from objective state, not prose:

```go
func autoReviewFingerprint(t Task, latestSubmitLog string) string
```

Minimum acceptable V1 fingerprint:

```text
taskID + "\x00" + reviewRef + "\x00" + latest "Submitted to code-review" log line timestamp/index
```

Store the fingerprint on auto-review queued/finished JSONL rows with a new optional `review_fingerprint` field on `agent.Event`. The coordinator skips when the latest current-epoch auto-review queued or finished event has the same fingerprint. Add tests for same-`ReviewRef` resubmission and changed-`ReviewRef` requeue.

Do not use only `ReviewRef`: stable branch names must review again after rework resubmits.

- [ ] **Step 9: Queue fresh review**

For eligible task:

1. If `Agent` blank, set it to default through `BoardService.Edit`.
2. Verify `runnerFor(agentName)` succeeds.
3. Reserve worker capacity through `AgentService`/budget path by calling `ReviewTaskAs`; handle `ErrWorkerCapacityFull`.
4. Emit:

```go
c.emit("auto-review:queued", map[string]any{
	"task_id": task.ID,
	"run_id": runID,
	"agent": agentName,
})
```

- [ ] **Step 10: Implement recovery sweep**

Before fresh candidates, inspect `snap.CodeReview` for `AgentStatus` `interrupted` or `lost`. For each, require:

```go
initiator, err := agent.LatestQueuedInitiator(boardDir, t.ID)
initiator == agent.InitiatorAutoReview
```

Then:

```go
interrupted -> c.agent.ResumeAgentAs(ctx, id, agent.InitiatorAutoReview)
lost        -> c.agent.ReviewTaskAs(ctx, id, agent.InitiatorAutoReview)
```

Use the same `resumeAttemptCooldown` map pattern as auto-implement.

- [ ] **Step 11: Wire coordinator in production**

In `gui/main.go`, construct:

```go
autoReview := tbapp.NewAutoReviewCoordinator(tbapp.AutoReviewCoordinatorOptions{
	Board: boardService, Agent: agentService, Settings: nil,
	Emitter: emitter, Logger: logger, WorkerBudget: d,
})
```

Add to tee emitter after auto-implement, to Wails services, and late-bind settings.

In `gui/adapters.go`, add field, activate/deactivate, max-worker/default-agent notifications, and `NotifyAutoReviewEnabled`.

- [ ] **Step 12: Complete coordinator tests**

Cover:

- disabled no mutation
- no default no mutation
- eligible code-review queues `mode=review`, `initiator=auto-review`
- explicit agent override
- default-agent fallback persists `Agent`
- missing `ReviewRef` writes `User Attention` and `needs-user`
- `AgentStatus=success` still eligible
- wrong columns skipped
- `needs-user` skipped
- unsupported explicit agent skips without overwrite
- duplicate watcher/restart dedupe
- same `ReviewRef` after resubmit eligible again
- changed `ReviewRef` while in code-review eligible again
- `interrupted` auto-review resumes
- `lost` auto-review restarts review
- user-initiated interrupted/lost not auto-resumed
- worker budget gates fresh and recovery runs

- [ ] **Step 13: Verify TB-264**

```bash
cd gui && go test ./...
cd gui && wails3 generate bindings -ts
```

Expected: all backend tests pass and bindings compile.

- [ ] **Step 14: Commit TB-264**

```bash
git add gui/app/auto_review.go gui/app/auto_review_test.go gui/app/agent_run.go gui/internal/agent/state.go gui/internal/cli/mutations.go gui/app/board_service.go gui/main.go gui/adapters.go gui/frontend/bindings
git commit -m "TB-264 enqueue auto-review runs"
```

---

### Task 4: Implement `TB-265` Auto-Review UI State

**Files:**
- Create: `gui/frontend/src/lib/stores/autoReview.ts`
- Create: `gui/frontend/src/lib/stores/autoReview.test.ts`
- Modify: `gui/frontend/src/lib/api.ts`
- Modify: `gui/frontend/src/routes/+page.svelte`
- Modify: `gui/frontend/src/lib/components/Card.svelte`
- Modify: `gui/frontend/src/lib/components/Card.test.ts`
- Modify: `gui/frontend/src/lib/components/TaskDrawer.svelte`
- Modify: `gui/frontend/src/lib/components/TaskDrawer.test.ts`

- [ ] **Step 1: Add auto-review store tests**

Create `autoReview.test.ts` for:

- maps backend `AutoReviewStatus`
- derives `needsDefaultAgent`
- exposes `skipReasonFor(id)`
- refreshes on `auto-review:*` events

- [ ] **Step 2: Implement `autoReview.ts`**

Mirror `autoGroom.ts` but with simpler fields:

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

Event names to register:

```ts
auto-review:needs-default-agent
auto-review:default-agent-cleared
auto-review:queued
auto-review:needs-user
auto-review:guarded-skip
auto-review:resume-failed
auto-review:scan-complete
```

- [ ] **Step 3: Render board-header and card state from auto-review store**

In `routes/+page.svelte`, call `refreshAutoReview()` on mount and register event handlers beside auto-groom. Keep the preference toggle from Task 2, but use coordinator status for skip/needs-default state when available.

In `Card.svelte`, for `status === "code-review"` render a compact existing-style pill when:

```ts
reviewSkipReason
review run queued/running from per-mode `reviewStatus`
```

Use existing run-history labels; do not expose raw JSONL.

- [ ] **Step 4: Render TaskDrawer fallback and skip details**

In `TaskDrawer.svelte`, show an actionable line in the review/action area:

- disabled: manual Review remains visible
- missing default agent: point to Settings
- missing target: point to `ReviewRef`
- needs-user: show existing user-attention state
- duplicate/active: show current review run state

Manual Review button stays usable when auto-review is disabled or skipped by non-terminal guards.

- [ ] **Step 5: Add pass/fail refresh tests**

Frontend tests should assert:

- pass removes stale code-review row after board refresh
- fail shows ready card with `review-failed`
- findings remain visible in drawer after fail
- manual Review remains when auto-review disabled

Use existing board store refresh test patterns. Do not add polling.

- [ ] **Step 6: Verify TB-265**

```bash
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

Expected: pass.

- [ ] **Step 7: Commit TB-265**

```bash
git add gui/frontend/src/lib/api.ts gui/frontend/src/lib/stores/autoReview.ts gui/frontend/src/lib/stores/autoReview.test.ts gui/frontend/src/routes/+page.svelte gui/frontend/src/lib/components/Card.svelte gui/frontend/src/lib/components/Card.test.ts gui/frontend/src/lib/components/TaskDrawer.svelte gui/frontend/src/lib/components/TaskDrawer.test.ts
git commit -m "TB-265 surface auto-review state"
```

---

### Task 5: Close `TB-262` Epic And Docs

**Files:**
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/FEATURES.md`
- Modify: `docs/IMPLEMENTATION.md`
- Modify: `board/CONVENTIONS.md`
- Modify: `board/SKILL.md`
- Modify: `cli/templates.go`
- Modify: `cli/templates_test.go`
- Modify board cards through `./cli/tb` only

- [ ] **Step 1: Update shipped-state docs**

Change planned auto-review text to present-tense shipped behavior:

- `auto_review_enabled` exists and defaults off
- pass uses `tb review --pass`
- missing `ReviewRef` writes `needs-user`
- recovery scopes to JSONL `initiator=auto-review`
- no prose pass/fail inference

Fix known stale docs line in `docs/IMPLEMENTATION.md` that still says `--fail` returns to backlog.

- [ ] **Step 2: Update generated board guidance**

Mirror the shipped auto-review wording into `cli/templates.go` so `tb init` creates current `board/CONVENTIONS.md` and `board/SKILL.md` content. Update `cli/templates_test.go` snapshots/assertions for the changed generated text.

- [ ] **Step 3: Run full verification**

```bash
cd cli && go test ./...
cd gui && go test ./...
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

Expected: all pass. If frontend deadcode is run, compare against the existing TB-247 baseline instead of treating known findings as new.

- [ ] **Step 4: Record manual smoke notes**

Use the GUI and record evidence in `TB-262`:

- human-only review
- auto-review pass
- auto-review fail
- cancel during review run
- app restart with queued/running review
- toggle auto-review while tasks already exist in `code-review`

- [ ] **Step 5: Move board tasks honestly**

Use `./cli/tb review --submit`, `./cli/tb done`, or task-board review commands according to actual commit/review outcome. Do not move `TB-262` to done until child cards and evidence are complete.

- [ ] **Step 6: Commit docs/board closeout**

```bash
git add docs/ARCHITECTURE.md docs/FEATURES.md docs/IMPLEMENTATION.md board/CONVENTIONS.md board/SKILL.md cli/templates.go cli/templates_test.go
git commit -m "TB-262 document shipped auto-review"
```

Only stage files actually changed.

---

## Plan Self-Review

Spec coverage:

- Managed pass command: Task 1.
- Persisted setting and controls: Task 2.
- Coordinator, initiator, recovery, dedupe, missing `ReviewRef`: Task 3.
- Visible runtime state and pass/fail UI refresh: Task 4.
- Docs and epic closeout: Task 5.

Known plan risks:

- Submission epoch is the hardest part. Implement it from objective move/log/JSONL markers and prove same-`ReviewRef` resubmission in tests.
- `review --pass` mirrors the existing fail two-step write-then-move pattern. If review demands one lock over both operations, refactor pass/fail together instead of adding a special one-off path.
- Route-level header tests may be awkward. If `+page.svelte` tests get brittle, extract a small automation-toggle component and test it directly.
