# Worktree Isolation for Task Execution

Status: Draft — pending user review.
Owner: tb-tools.
Target milestone: new M-WT (slots in before any multi-agent / epic-orchestration work).

## 1. Goal

Every task that involves code changes runs inside its own `git worktree` on a dedicated branch. On task completion the work is rebased onto the configured merge target and fast-forwarded back; the worktree and branch are cleaned up. On conflict the worktree is preserved for manual resolution.

This is the foundational primitive that later enables:

- Multiple agents (or humans) working on different tasks in parallel without trampling each other's working tree.
- Epic fan-out with `max_workers > 1` (per the prior brainstorming): each child task lives in its own worktree, so parallel execution becomes safe at the source-tree level.

## 2. Non-goals

- **No automatic conflict resolution.** Conflicts surface to the user; the system never invents merges.
- **No re-attach to live runs across restarts.** M5 carve-out is unchanged.
- **No epic-level worktrees.** Epics don't write code; they plan. They run in the main checkout.
- **No multi-board / cross-repo support** beyond what `tb` already does.
- **No removal of the existing single-worktree flow.** Tasks can still run "in place" if a board opts out (see § 7 config).

## 3. The fundamental decision: board is canonical, worktrees carry code only

The board (`board/` directory with task `.md` files, `BOARD.md`, `.board.lock`, `.next-id`, `.agent-state/`, `.agent-logs/`, attachments of folder-form tasks) lives in **one canonical location** — the main checkout's `board/`. Worktrees are *only* a separate view of the repo's source code on a feature branch.

This is the load-bearing decision. Reasoning:

- Each git worktree has its own working copy of every tracked file. If `board/` were tracked, each worktree would have its own `board/`, its own `.board.lock`, its own `.next-id`. Mutations in one worktree wouldn't be visible to the daemon, UI, or other agents until merge. The board would diverge across worktrees.
- The daemon, GUI, and `tb` CLI all assume a single authoritative board. Honoring that means decoupling board state from per-worktree git state.

### 3.1 How this is enforced

A linked worktree's `.git` is a **pointer file**, not a directory. The per-worktree git metadata (hooks, info/exclude, rebase state, etc.) lives at `<main_repo>/.git/worktrees/<ID>/`. Resolve real paths with `git -C <worktree> rev-parse --git-path <subpath>` — e.g., `git -C .tb-worktrees/PR-100 rev-parse --git-path info/exclude` yields the actual exclude file location. Every reference in this spec to "the worktree's `.git/...`" means *that resolved path*, not the literal pointer.

**Three enforcement layers, defense in depth:**

1. **Write-time exclusion.** When a worktree is created, `tb` appends the configured board path (`<board>`, resolved from `.tb.yaml` — defaults to `board/`) to the worktree's `info/exclude` (resolved per the linked-worktree rules above) and runs `git update-index --skip-worktree` for every tracked file under `<board>/` inside the worktree, marking them locally ignored. Combined, this prevents accidental edits from being committable.

2. **Resolution sentinel.** `tb` writes `.tb-worktree.json` at the worktree root containing the absolute path to the canonical board:
   ```json
   { "canonical_board": "/abs/path/to/main/checkout/board", "task_id": "PR-100" }
   ```
   This file is added to the worktree's `info/exclude` (not committed). `tb`'s board-resolution algorithm is updated to consult sentinels **before** `.tb.yaml`:
   1. `TB_BOARD_PATH` env var (highest priority).
   2. Walking up from cwd, first match wins: `.tb-worktree.json` (use `canonical_board`) OR `.tb.yaml` (use the path it configures).
   3. Error if nothing found.

   Critically, this means a **human** running `cd .tb-worktrees/PR-100 && tb edit PR-100` resolves to the canonical board even without `TB_BOARD_PATH`. The sentinel beats the worktree-local `.tb.yaml` (which exists because it was committed at the merge target's HEAD).

3. **Agent env var.** The daemon still injects `TB_BOARD_PATH` for agent processes — belt and braces, and useful because it makes the resolution explicit in logs/troubleshooting.

Folder-form task contents (`board/<status>/<ID>/attachments/`, task-local `.agent-state.jsonl`, `.agent-logs/`) live in the canonical board, resolved through the same mechanism. Non-folder-form `.agent-state/` and `.agent-logs/` likewise stay in the canonical board.

**Invariant:** worktrees never write to `board/`. The sentinel + env var ensure all `tb` invocations from a worktree target the canonical board; the exclude + skip-worktree ensure that any direct file writes to the worktree's `board/` are git-invisible and discarded at merge time.

## 4. Worktree lifecycle

### 4.1 Layout

Worktrees live at `<repo root>/.tb-worktrees/<task_id>/`. Path is configurable per board via `.tb.yaml` → `worktrees.path` (relative to repo root). Default `.tb-worktrees`.

**Gitignore additions (added at `tb init` and when `worktrees.enabled` first flips to true):**

Patterns are resolved against the configured board path (from `.tb.yaml` — defaults to `board/` but may be customized). Letting `<board>` stand for that resolved relative path:

- `<worktrees.path>/` (e.g. `.tb-worktrees/`) — the worktrees themselves are managed by `git worktree`, not by source-tree tracking.
- `<board>/.worktrees/` — sidecar JSON files; machine-local runtime state containing absolute paths.
- `<board>/.worktree-locks/` — per-task flock files; ephemeral.
- `<board>/.merge.lock` — global merge flock; ephemeral.

`tb init` materializes these into the repo's `.gitignore` using the resolved paths (creating the file if absent and deduplicating). Without them the sidecars and lock files appear as untracked candidates in the main checkout (where `<board>/` is otherwise tracked) and risk leaking into commits — they contain machine-local absolute paths that have no meaning on another developer's machine.

### 4.2 Branch naming

Per-task branch: `task/<task_id>` (e.g. `task/PR-100`). Configurable via `worktrees.branch_prefix` (default `task/`).

### 4.3 Base branch

Configurable per board via `.tb.yaml` → `worktrees.merge_target`. Default `main`.

Optionally per-task override via a new metadata field `**MergeTarget:** <branch>` — if absent, board default is used.

### 4.4 States and transitions

```
                              ┌───────────────────────┐
                              │ no worktree (default) │
                              └───────────┬───────────┘
                                          │ tb start <ID>   (or daemon, before agent run)
                                          ▼
                              ┌───────────────────────┐
              ┌── cancel ────►│      worktree-live    │
              │               │  (agent or human in   │
              │               │   .tb-worktrees/<ID>) │
              │               └───────────┬───────────┘
              │                           │ tb done <ID>
              │                           ▼
              │               ┌───────────────────────┐
              │               │  rebase + merge under │──► merged → cleanup → status=done
              │               │     .merge.lock       │
              │               └───────────┬───────────┘
              │                           │ conflict
              │                           ▼
              │               ┌───────────────────────┐
              │               │   MergeStatus=conflict│
              │               │  (task stays in       │
              │               │   in-progress;        │
              │               │   worktree preserved) │
              │               └───────────┬───────────┘
              │                           │ user resolves in worktree,
              │                           │ then `tb merge <ID>`
              │                           ▼
              │               ┌───────────────────────┐
              └─────────────► │   cleanup-only        │──► worktree + branch removed,
                              │ (cancel / fail keeps  │    task stays where it is
                              │  worktree until user  │    (done / failed / etc).
                              │  explicitly cleans)   │
                              └───────────────────────┘
```

### 4.5 Creation: `tb start <ID>` and daemon agent run

`tb start <ID>` (today: just moves the file to `in-progress/`) gains a step: if the board has worktree mode enabled and no worktree exists for `<ID>`, create it.

**Preflight (once per board, at `tb init` or first time `worktrees.enabled` flips to true):**

- Detect submodules: if `.gitmodules` exists at the repo root, refuse to enable worktree mode with a clear error. (Submodule state in `.git/modules/` is shared across worktrees, which breaks isolation.) Force-override: `worktrees.allow_submodules: true` in `.tb.yaml` — opts the user in at their own risk.
- Detect path conflict: if `<repo root>/<worktrees.path>` exists and is not an empty directory previously created by `tb`, refuse with instructions to rename or remove. The path must be either absent or a directory containing only `tb`-managed worktrees.

**Atomicity goal:** if any step below fails, the task does **not** move out of `backlog/`. The file-system move is the *last* thing we do, after the worktree is fully alive. This satisfies AC9's "no partial state" and the existing `Directory = status` invariant.

Algorithm:

1. Take per-task `.worktree.lock` at `<board>/.worktree-locks/<ID>.lock` (distinct from `.board.lock` to avoid blocking board mutations during git operations). Per-task locks allow concurrent `tb start` on *different* tasks. Throughout the spec `<board>` means the configured board path (`.tb.yaml` → board path, defaults to `board/`).
2. Resolve merge target: `git rev-parse --verify <merge_target>` — error if missing.
3. Resolve branch name. If `task/<ID>` already exists locally, fail loudly — do not auto-overwrite. (Failed/abandoned earlier runs left a branch; user must explicitly `tb worktree clean <ID>` first.) Cross-check `git worktree list` to detect orphan worktrees without sidecars (see § 4.12 recovery).
4. **Write sidecar first** at `<board>/.worktrees/<ID>.json` with `state: "creating"`, using the same atomic write discipline as task files (`writeFileAtomic`: temp file in same directory → `fsync` → `os.Rename`). See § 4.13.
   ```json
   { "state": "creating", "path": ".tb-worktrees/PR-100", "branch": "task/PR-100", "mergeTarget": "main", "baseSHA": "abc123…", "createdAt": "…" }
   ```
   This is what lets recovery clean up partial state (§ 4.12). The sidecar is written *before* git work so it always exists if the worktree exists.
5. `git worktree add -b task/<ID> <worktrees.path>/<ID> <merge_target>`.
6. Inside the worktree, resolve git metadata paths with `git rev-parse --git-path` (linked worktrees have `.git` as a pointer file, not a directory — see § 3.1):
   - Append `<board>/` and `.tb-worktree.json` to `$(git -C <wt> rev-parse --git-path info/exclude)`.
   - Apply `--skip-worktree` to every tracked file under `<board>/`: `git -C <wt> ls-files -- <board>/ | git -C <wt> update-index --skip-worktree --stdin`.
7. Write the resolution sentinel `<wt>/.tb-worktree.json` per § 3.1 (already in exclude from step 6).
8. **Install `pre-push` hook** at `$(git -C <wt> rev-parse --git-path hooks/pre-push)`. The hook rejects any push of refs matching `<branch_prefix>*` (default `task/*`). Mode `0755`. Blocks an agent (or absent-minded human inside the worktree) from publishing the working branch to a remote.
9. Update sidecar to `state: "active"` and refresh `baseSHA` to the current `merge_target` HEAD.
10. **Now** take `.board.lock`. Re-verify the task file is still in `backlog/<ID>.md` (or folder form). Move it to `in-progress/<ID>.md`. Append Log line. Regenerate `BOARD.md`. Release `.board.lock`.
11. Release `.worktree.lock`.

If step 10's re-verification fails (someone moved/closed the task during steps 2–9 via a parallel `tb` invocation), the worktree is already created but the task is not where we expect. Roll back: `git worktree remove --force`, `git branch -D task/<ID>`, delete sidecar, release `.worktree.lock`, return error. This rollback path is the rare exception — the common case is success.

**Idempotency for daemon use:** if a worktree already exists for `<ID>` (sidecar present, `state: "active"`, worktree path resolves via `git worktree list`), skip steps 2–9 entirely and only do step 10 if the task is still in `backlog/`. The daemon's `EnsureWorktree(ID)` is exactly this fast path. If sidecar shows `state: "creating"`, treat as orphan and let recovery handle it before retrying.

### 4.6 Agent execution inside the worktree

When the daemon spawns an agent for a task whose worktree exists, it sets:

- `cwd = <repo root>/<worktrees.path>/<ID>`
- `TB_BOARD_PATH = <absolute path to canonical board>` (resolved from main checkout's `.tb.yaml`; defaults to `<repo root>/board`)
- (Optional, M-WT.2) `TB_TASK_ID = <ID>` for prompt scaffolding.

The agent prompt gains a small footer reminding it: "You are operating inside a git worktree at `<cwd>`. Commit your changes to the current branch. Do not switch branches. Do not run `git push`. The canonical task board is at `$TB_BOARD_PATH` — use `tb` normally for task operations."

The agent commits to `task/<ID>` as it works. No push, no rebase mid-run, no branch switching.

### 4.7 Completion: `tb done <ID>`

`tb done` becomes a multi-step operation when worktree mode is on. Two key changes from the naive flow:

- **FF-merge is a three-case split** on (i) whether the main checkout's HEAD equals `<merge_target>` and (ii) whether any *external* worktree (outside `<worktrees.path>/`) has `<merge_target>` checked out. Cases A (merge in main) and B (ref-push) succeed; case C refuses with a clear error pointing at the conflicting external worktree. We do **not** unconditionally `git push . task/<ID>:<merge_target>` — Git's `receive.denyCurrentBranch=refuse` default rejects pushes to the checked-out branch in any non-bare worktree. We also do **not** require a blanket-clean main working tree: the task branch has `skip-worktree` on `board/`, so the FF diff never touches our canonical board state. See § 4.7 step 9 for the full algorithm.
- **On rebase conflict we do NOT abort the rebase.** It's left mid-flight in the worktree. The user resolves files, `git add`, `git rebase --continue`, then re-runs `tb merge <ID>`. The retry sees no in-progress rebase and proceeds with the FF-push step. This aligns the documented recovery flow with what `git` actually requires.

The sidecar carries a **phase** field that recovery (§ 4.12) uses to figure out where a crash happened:

```
sidecar.phase: "active" → "rebasing" → "finalizing" → "cleanup" → (deleted)
```

Phase transitions are written *before* the corresponding git/filesystem action, so a crash leaves the phase pointing at the work that *may not have completed*.

Algorithm:

1. Take per-task `.worktree.lock(<ID>)`.
2. Read sidecar. Resolve worktree path, branch, merge target. Validate `state: "active"`.
3. Check worktree cleanliness: `git -C <wt> status --porcelain` empty? If not → fail with `dirty_worktree`. No state change. Additionally probe for an in-flight rebase via both `git -C <wt> rev-parse --git-path rebase-merge` and `--git-path rebase-apply` (Git uses one or the other depending on the rebase backend). If either dir exists, fail with `rebase_in_progress` — the user is expected to finish it via `git rebase --continue` first.
4. Take global `.merge.lock`.
5. Write sidecar `phase: "rebasing"`.
6. Inside the worktree:
   - Optional `git fetch` if `worktrees.fetch_before_merge: true`.
   - `git rebase <merge_target>`. If conflict → **do not abort**. Leave the rebase in flight. Take `.board.lock`, set `**MergeStatus:** conflict` on the task via the normal `tb edit` path, append Log "Merge conflict — resolve in worktree, then `tb merge <ID>`", regenerate, release `.board.lock`. Release `.merge.lock` and `.worktree.lock`. Return error to caller. Worktree + branch preserved with rebase mid-flight.
7. Rebase clean (either fresh or resumed). Write sidecar `phase: "finalizing"`.
8. Take `.board.lock`. **Re-validate** task state under the lock:
   - Task file still at `in-progress/<ID>.md` (or folder form)? If not → `state_drift` error; release locks; return. The rebase already happened; user can inspect and decide.
   - Sidecar's recorded `branch` matches the task's `**Branch:**` field if set? (If user manually edited Branch to something else, surface and refuse.)
9. Still under `.board.lock`, run the FF-finalize. The algorithm splits on whether `<merge_target>` is currently checked out in *any* worktree visible to git:

   ```
   ffSafe := `git -C <main> merge-base --is-ancestor <merge_target> task/<ID>` succeeds
   if not ffSafe → error not_fast_forwardable (shouldn't happen post-rebase; bug if it does)

   # Enumerate ALL worktrees, not just main. Modern git enforces denyCurrentBranch
   # across all linked worktrees, not only the main checkout.
   mainHead   := `git -C <main> symbolic-ref --short HEAD`   # "" if detached
   externals  := every worktree from `git worktree list --porcelain` whose:
                     - path is NOT the main checkout
                     - path is NOT inside <worktrees.path>/   (those are our own task worktrees, always on task/*)
                     - HEAD branch == <merge_target>

   case A — mainHead == <merge_target> AND externals is empty:
       # Main is on the target and nobody else has it. Use merge.
       # We deliberately do NOT pre-check `git status --porcelain` clean:
       #   - The main checkout's board/ is dirty *by design* during a tb done flow
       #     (sidecar phase writes, the prior tb start file move, log appends).
       #   - The task branch has skip-worktree on board/ in its worktree, so the FF
       #     diff never touches board/ in the main checkout. No conflict from our
       #     own state.
       #   - Any unrelated user dirt outside board/ is the user's problem; let
       #     `git merge --ff-only` itself refuse if the FF diff would overwrite it.
       `git -C <main> merge --ff-only task/<ID>`
       → on git error (e.g. "would be overwritten by merge"): release `.board.lock`,
         error `merge_blocked_by_main_changes: <git stderr>`, leave sidecar at
         `phase: "finalizing"` for `tb merge <ID>` retry.

   case B — externals is empty (and mainHead != <merge_target>):
       # Target is not checked out anywhere. Ref-push is safe.
       `git -C <main> push . task/<ID>:<merge_target>`

   case C — externals is non-empty (regardless of mainHead):
       # Some other worktree (user-created, outside our control) has <merge_target>
       # checked out. denyCurrentBranch will reject the push; we also can't merge
       # remotely without intruding on the user's workspace.
       release `.board.lock`, error
       `merge_target_checked_out_externally: paths=[<list>]; please switch those
        worktrees to a different branch and retry tb merge <ID>`.
   ```

   In all three cases, the sidecar stays `phase: "finalizing"` until success — failures leave a recoverable state behind. Recovery (§ 4.12) reaches the same step on the next start.
10. FF succeeded: still under `.board.lock`, move task file `in-progress/<ID>.md` → `done/<ID>.md`. Append Log "Done — merged at `<merge_target SHA>`". Clear `**MergeStatus:**` if set. Regenerate `BOARD.md`. Release `.board.lock`.
11. Write sidecar `phase: "cleanup"`.
12. `git worktree remove <worktrees.path>/<ID>` (use `--force` only if the worktree was already removed — defensive). `git branch -d task/<ID>` (force `-D` only if `-d` refuses with "already merged" being false-positive; should not happen post-FF).
13. Delete sidecar `<board>/.worktrees/<ID>.json`.
14. Release `.merge.lock`. Release `.worktree.lock`.

**Lock scope.** `.merge.lock` is held for steps 4–14 (typically seconds). `.board.lock` is held briefly in step 6 (only on conflict, for the `tb edit` setting MergeStatus) and across steps 8–10 (re-validation + FF-push + file move). Other processes' `tb edit`/`tb create`/`tb mv` block only during the brief 8–10 critical section, not for the rebase.

### 4.8 Retry after conflict: `tb merge <ID>` (new command)

When a task has `MergeStatus: conflict`, the worktree still has a rebase in flight. The user:

1. Resolves files in the worktree (typical: open `.tb-worktrees/<ID>` in an editor, fix conflict markers).
2. `git add <resolved files>`.
3. `git rebase --continue` — repeat for each conflicting commit until rebase reports success.
4. Runs `tb merge <ID>`.

`tb merge <ID>` algorithm:

1. Take `.worktree.lock(<ID>)`.
2. Read sidecar; validate `state: "active"` (regardless of `phase`).
3. Check rebase state: `git -C <wt> rev-parse --git-path rebase-merge` OR `--git-path rebase-apply` exists? Both directories signal an in-flight rebase (Git uses one or the other depending on backend and config; recovery must check both). If yes → fail with `rebase_in_progress` instructing the user to finish `git rebase --continue` or `git rebase --abort` first. (Refusing to silently abort is the right call — we never throw away the user's commits.)
4. Worktree clean? If not → `dirty_worktree`.
5. From here on, identical to `tb done` step 4 onward — and **we always re-run the rebase**, never skip it. Reason: `<merge_target>` may have advanced between when the user finished `git rebase --continue` and when they called `tb merge`. The re-rebase is a no-op when the branch is already up to date with `<merge_target>`, and surfaces a fresh conflict otherwise (looping back into the § 4.7 step 6 conflict path, which is the correct outcome — the user must rebase onto the new tip too).

This is also the idempotent finalization path: if the user just wants to merge an already-clean rebased branch (no conflict was ever set), `tb merge <ID>` works the same way.

### 4.9 Cancellation

Cancelling an agent run (existing F4.4 flow) is unchanged: process is killed, `AgentStatus: cancelled`. The worktree is **not** auto-removed — uncommitted state may be valuable. User cleans up explicitly with `tb worktree clean <ID>`.

### 4.10 Failure

If the agent run ends with `AgentStatus: failed`, the daemon does **not** attempt to merge. Worktree remains. User decides: retry (restart agent in same worktree — the branch already has partial commits), abandon (`tb worktree clean <ID>`), or finalize manually.

### 4.11 Cleanup: `tb worktree clean <ID>` (new command)

Removes worktree + branch for a task. Requires either:

- `--force`, or
- The task is in `done/` or `archive/` (i.e., already finalized via another path), or
- The branch has no commits beyond the recorded `baseSHA` (nothing to lose).

Without `--force` and outside those conditions, refuses and reports what would be lost.

### 4.12 Recovery on daemon startup

Recovery runs as an extension of existing M5 stale-recovery. **Two scans, then per-task reconciliation:**

**Scan A — sidecar scan.** For each `<board>/.worktrees/<ID>.json`, reconcile by `state` and `phase` using the table below. Do **not** short-circuit on "worktree path missing" — the table is the single source of truth for what to do; the worktree-missing case is handled inside each row that needs to know.

**Scan B — orphan worktree scan.** For each entry in `git worktree list` under `<worktrees.path>/<ID>` that has no matching sidecar → unmanaged worktree (sidecar deleted out of band, or some external process created a worktree at our path). Log to `tb doctor` for human attention. Do **not** auto-delete (it could contain uncommitted work).

**Helpers used in the table:**

- `wtExists(ID)` — the path recorded in sidecar appears in `git worktree list`.
- `branchExists(ID)` — `git rev-parse --verify --quiet task/<ID>` succeeds.
- `taskDir(ID)` — the canonical board status directory the task file currently lives in (`backlog`, `in-progress`, `done`, `archive`, or `missing`).
- `mergeContainsBranch(ID)` — `git merge-base --is-ancestor task/<ID> <merge_target>` succeeds. This is the **ancestry** check, not equality: it stays true even if other tasks have advanced `<merge_target>` afterward.
- `rebaseInFlight(ID)` — either `git -C <wt> rev-parse --git-path rebase-merge` OR `--git-path rebase-apply` resolves to an existing directory.

**Reconciliation table** for sidecars found in Scan A:

| `state`   | `phase`       | `taskDir`      | Recovery action |
|-----------|---------------|----------------|-----------------|
| `creating` | (any)        | (any)          | Step 5–9 of § 4.5 didn't complete. Clean up everything that may exist: `git worktree remove --force <path>` if `wtExists`, `git branch -D task/<ID>` if `branchExists`, delete sidecar. Task stays where it is (still in `backlog/` because the file move is § 4.5 step 10, which `creating` never reached). |
| `active`   | `active`      | `backlog`      | § 4.5 crashed *after* step 9 (`state: active`) but before step 10 (the file move). Idempotently complete the move under `.board.lock`: `backlog/<ID>.md` → `in-progress/<ID>.md`, append Log "Recovered: completing tb start file move", regenerate. Worktree + branch are intact and reused. |
| `active`   | `active`      | `in-progress`  | Normal steady state. If `wtExists` is false → worktree was deleted out of band; this is unusual but recoverable: log to `tb doctor`, clean up branch + sidecar if user confirms via `tb worktree clean <ID> --force`. Otherwise no action; existing M5 PID-liveness handles stale agent runs. |
| `active`   | `active`      | `done`/`archive`/`missing` | Inconsistent: an active worktree exists for a finalized task. Log to `tb doctor`; do not auto-act. |
| `active`   | `rebasing`    | (any)          | A merge crashed during or before rebase. If `rebaseInFlight(ID)` → set `**MergeStatus:** conflict`, append Log "Recovered: rebase mid-flight, resolve and `tb merge <ID>`", leave worktree. If NOT `rebaseInFlight` → rebase either completed or never started; reset sidecar to `phase: "active"` so the next `tb merge` retries cleanly. (Either way, no FF happened — the phase change to `finalizing` would have been written first.) |
| `active`   | `finalizing`  | `in-progress`  | Mid-finalize crash, FF status unknown. If `mergeContainsBranch(ID)` true → FF happened; under `.board.lock` move task file to `done/`, clear `**MergeStatus:**`, regenerate; transition to `phase: "cleanup"` and apply that row's actions. If false → FF didn't happen; reset sidecar to `phase: "active"`, leave for `tb merge <ID>` retry. |
| `active`   | `finalizing`  | `done`         | If `mergeContainsBranch(ID)` true → user/earlier-retry already moved the task; FF really happened; transition to `phase: "cleanup"`. If false → inconsistent (task is in `done/` but FF didn't happen) — log to `tb doctor` and do nothing. **Do not auto-move the task back.** |
| `active`   | `finalizing`  | `archive`      | User archived the task during the recovery window. If `mergeContainsBranch(ID)` true → finish branch+worktree+sidecar cleanup (the `cleanup` row's git/sidecar actions), but **do not touch the task file** — respect the user's archive action. If false → log to `tb doctor`. |
| `active`   | `finalizing`  | `backlog`/`missing` | Inconsistent — the task was in `in-progress/` when finalize began. User intervened oddly or filesystem is corrupt. Log to `tb doctor`, take no auto action. |
| `active`   | `cleanup`     | `done`/`archive` | Crash during worktree/branch/sidecar removal. Idempotent retry: `git worktree remove --force` if `wtExists`, `git branch -d task/<ID>` (or `-D` if `-d` refuses; safe here because we already verified `mergeContainsBranch` was true to reach `cleanup`), delete sidecar. No `.board.lock` needed — task is already finalized. |
| `active`   | `cleanup`     | other          | Inconsistent — `cleanup` is only entered after the task file moved to `done/`. Log to `tb doctor`. |

All recovery actions that take `.board.lock` (the `active/backlog` row's file move, and the `finalizing` post-FF case) acquire locks per § 6 ordering: `.worktree.lock(ID)` → `.board.lock`. Never `.merge.lock` during recovery — recovery cannot run a fresh rebase or FF-push (those need user-clean state), only finalize a crash that already passed those steps.

**Lock state on startup.** All file locks are POSIX `flock` — the OS releases them when the holder process dies. Recovery doesn't need to "release" anything before scanning. The reconciliation actions themselves take the appropriate locks (`.worktree.lock(ID)` and, where relevant, `.board.lock`) in the same order as the normal flow.

`tb doctor` (M-WT.2 deferred command) surfaces any unresolvable state from these scans — primarily Scan B's unmanaged worktrees and the `rebasing` cases that need human conflict resolution.

### 4.13 Sidecar atomicity

All sidecar writes (`<board>/.worktrees/<ID>.json`, including phase transitions during § 4.7) use the same `writeFileAtomic` discipline mandated for task `.md` files (see `cli/atomicfs.go`): write to a temp file in the same directory, `fsync`, `os.Rename` over the target. This guarantees readers never see a half-written sidecar.

A corrupted or unreadable sidecar (file exists but JSON parse fails) is **not** auto-repaired by recovery. § 4.12 treats it as an unresolvable case: log to `tb doctor`, skip the entry, leave any worktree/branch intact for human inspection. A corrupt sidecar is much rarer than a crash mid-phase, and a wrong guess (auto-deleting a branch that contained work) is worse than a noisy doctor entry.

The same rule applies to `.tb-worktree.json` inside each worktree: it's written atomically at creation (§ 4.5 step 7) and never rewritten thereafter — there are no phase transitions for that file.

## 5. CLI surface changes

New commands:

- `tb merge <ID>` — finalize a task whose merge previously conflicted (or any in-progress task with a worktree).
- `tb worktree list` — show all active worktrees (path, branch, task, base SHA).
- `tb worktree clean <ID> [--force]` — remove worktree + branch.
- `tb worktree status <ID>` — show worktree + git state for one task.

Modified commands:

- `tb start <ID>` — additionally creates worktree if mode is enabled.
- `tb done <ID>` — additionally rebases + merges + cleans worktree.
- `tb mv <ID> backlog` (move back) — does **not** auto-remove worktree. User must `tb worktree clean` explicitly. Reason: moving an in-progress task back to backlog is rare and usually means "I'll come back to it"; preserving the worktree matches intent.

Unchanged commands: `tb edit`, `tb create`, `tb show`, `tb ls`, `tb close`, `tb archive`, `tb regenerate`.

New task metadata fields:

- `**MergeTarget:** <branch>` — optional, overrides board default. Settable via `tb edit --merge-target=<branch>`.
- `**MergeStatus:** none | conflict` — managed by `tb`, not user-settable directly. (Set to `conflict` by `tb done` failure; cleared by `tb merge` success.)

New `.tb.yaml` keys:

```yaml
worktrees:
  enabled: true            # default true on new boards; default false on existing boards (opt-in for migration)
  path: .tb-worktrees      # relative to repo root
  branch_prefix: task/
  merge_target: main
  fetch_before_merge: false
```

## 6. Concurrency model

Three locks, distinct purposes:

| Lock | Scope | Held during | Blocks |
|---|---|---|---|
| `.board.lock` | board-wide | every structured mutation (`tb edit/create/mv`/etc.) | all other board mutations |
| `<board>/.worktree-locks/<ID>.lock` | per-task | worktree create / destroy / merge of *this* task | other worktree ops on the same task |
| `<board>/.merge.lock` | board-wide | rebase + FF-merge | other merges (across all tasks) |

Why three:

- `.board.lock` is short-lived (milliseconds — a single file move + regenerate). It must not be held during long git operations.
- Per-task `.worktree-locks/<ID>.lock` lets two different tasks have their worktrees created/destroyed in parallel.
- `.merge.lock` is global and held during the (potentially slow) rebase+merge. It serializes merges into `merge_target` so we never have two FF-merges racing for the same branch tip.

Ordering rules:

1. **Never acquire `.merge.lock` while holding `.board.lock`.** The merge code (§ 4.7) takes locks in order `.worktree.lock(ID)` → `.merge.lock` → (briefly, at steps 8–10) `.board.lock` for re-validation + FF-push + file move, releasing each in reverse. Every other path that touches `.board.lock` (`tb edit/create/mv/etc.`) never reaches for `.merge.lock`. This rules out the only possible cycle.

2. **`.worktree.lock(ID)` is always taken before `.board.lock` in worktree-aware flows** (§ 4.5, § 4.7, § 4.8). `tb start` no longer takes `.board.lock` first; the file move is the last step, performed under `.board.lock` while `.worktree.lock` is still held. This ordering rules out a creation/finalization race on the same task.

3. **Re-validation under `.board.lock` is mandatory in the merge finalization path** (§ 4.7 step 8). The task's status directory, ID, and `**Branch:**` field are re-read inside the lock immediately before the FF-push. If state has drifted (concurrent `tb mv`, `tb close`, manual file edit), the merge aborts with `state_drift` and leaves the rebased branch for human inspection. This closes the "stale assumptions" gap Codex flagged.

Consequence of holding `.merge.lock` for the rebase + cleanup: other `tb done`/`tb merge` calls block (intended — serial merges into `merge_target`). Unrelated board mutations from other processes are *not* blocked by `.merge.lock`; they queue briefly on `.board.lock` only during the 8–10 critical section of the merging task (typically tens of milliseconds: ref update + file move + regenerate).

## 7. UI surface (sketched only — GUI work is downstream)

- Task drawer shows worktree status: path, branch, "open in editor" button.
- New badge on cards: "merge conflict" when `MergeStatus: conflict`.
- "Retry merge" button in drawer when `MergeStatus: conflict`.
- Settings: `worktrees.enabled` toggle; merge-target chooser.
- `tb-gui` honors `TB_BOARD_PATH` env var if set (matches the agent's resolution rule).

Detailed GUI design deferred to a separate spec once this lands.

## 8. Migration

Worktree mode is **opt-in** on existing boards via `worktrees.enabled: true` in `.tb.yaml`. Default for fresh `tb init` is `enabled: true`.

For an existing board enabling the mode:

- Tasks currently in `in-progress/` continue to run without worktrees (their workflow predates the mode). They merge "in place" via a degraded path: `tb done` skips steps 4–8 and just moves the file.
- New `tb start` invocations create worktrees normally.
- A new `tb worktree adopt <ID> [--branch <existing-branch>]` command associates a pre-existing branch with an in-progress task, creating the worktree and sidecar. Useful for retrofitting.

## 9. Risks & open questions

- **Disk usage.** N worktrees = N working trees. For large monorepos this is painful. Mitigations: `git worktree --no-checkout` then sparse-checkout, but that's a follow-up. For the first cut, accept the disk cost and document it. Hard upper bound is `max_workers` (default 1) so under default config the worst case is 1 extra worktree.

- **Long-lived branches & base drift.** A task open for a week sees its base drift far from the current `merge_target`. The rebase at `tb done` time may be ugly. Mitigation (M-WT.2): a periodic `tb worktree refresh <ID>` that rebases proactively while the agent isn't running. Out of scope for the first cut.

- **LFS.** `git worktree` + Git LFS works but doubles LFS bandwidth on creation. Document, don't auto-handle. Power users can set `GIT_LFS_SKIP_SMUDGE=1` per-worktree as a follow-up.

- **Agent in worktree creates a sibling task via `tb create`**. The new task's file lands in canonical board (good). The agent's branch doesn't see it (good — sibling tasks are independent). If the new task is a child of an in-progress epic, the epic's child list now includes it — daemon picks it up normally.

- **What if two agents need the same dependency, both edit `package.json`?** Worktree isolates code edits — both agents commit non-conflicting edits to their own branches. At merge time the second one to merge rebases on top of the first; trivial JSON conflicts can still arise. This is by design — the system surfaces those rather than silently merging.

## 10. Acceptance criteria

Functional:

- **AC1**: Fresh board with `worktrees.enabled: true`. `tb create "x"` then `tb start PR-1`. Expected: `<worktrees.path>/PR-1/` exists, branch `task/PR-1` exists at `merge_target`'s HEAD, sidecar `<board>/.worktrees/PR-1.json` present, task file in `in-progress/`.

- **AC2**: Inside the worktree, `cd .tb-worktrees/PR-1 && unset TB_BOARD_PATH && tb edit PR-1 -d "new desc"`. Verify: the canonical board's task file is updated (visible from main checkout), the worktree's `board/` is unchanged (`git -C .tb-worktrees/PR-1 status --porcelain` does not list anything under `board/`), and `.tb-worktree.json` is present at the worktree root. Repeat with `TB_BOARD_PATH` set — same outcome.

- **AC3a**: Main checkout is on `<merge_target>` (the common case). The only working-tree state is whatever `tb start` introduced under `<board>/` (uncommitted file move + sidecar) — no other dirt. Commit a change in the task worktree, run `tb done PR-1`. Expected: task moves to `done/`, main checkout's `HEAD` advances to the task branch's HEAD (FF-merged into the currently checked-out branch), worktree removed, branch deleted, sidecar removed. The lingering board/ working-tree changes from before AC3a started are unaffected. Verifies case A of § 4.7 step 9 — and that the design does NOT require a blanket-clean main working tree (no pre-check on `git status --porcelain`).

- **AC3b**: Main checkout is on an unrelated branch (NOT `<merge_target>`). Commit a change in the worktree, run `tb done PR-1`. Expected: task moves to `done/`, `<merge_target>` ref advances to the task branch's HEAD, the main checkout's `HEAD` (the unrelated branch) is **unchanged**, worktree removed, branch deleted, sidecar removed. Verifies the ref-push variant of § 4.7 step 9 and that `denyCurrentBranch` is not triggered.

- **AC3c**: Main checkout is on `<merge_target>` with dirty board/ (always true after `tb start` — the file move + sidecar writes dirty `board/` in main). Run `tb done PR-1`. Expected: succeeds. The `merge --ff-only` does not refuse because the task branch's FF diff doesn't touch `board/` (skip-worktree at create time). Task moves to `done/`, `<merge_target>` advances, worktree cleaned up. The lingering uncommitted board/ changes (which are exactly the new task file at `done/<ID>.md` and the `BOARD.md` regeneration) remain in the working tree — they'll be committed by whatever flow normally commits board changes. Verifies the "no blanket clean check" rule.

- **AC3d**: Main checkout is on `<merge_target>` AND has uncommitted edits in a file that the task branch also modified. Run `tb done PR-1`. Expected: `git merge --ff-only` refuses with "would be overwritten by merge"; tb propagates as `merge_blocked_by_main_changes`. Task stays in `in-progress/`, sidecar stays `phase: "finalizing"`. User stashes/commits, runs `tb merge PR-1`, succeeds.

- **AC3e**: External worktree (user-created via `git worktree add ../other-checkout main`) has `<merge_target>` checked out. Main is on a different branch. Run `tb done PR-1`. Expected: exit non-zero with `merge_target_checked_out_externally`, listing the external path. No state change. User can switch that worktree to another branch and retry. Verifies that the all-worktrees scan in § 4.7 step 9 catches external `denyCurrentBranch` risks.

- **AC4**: Set up a conflict: in main, advance `merge_target` with a conflicting change after `tb start`. Commit work in worktree. Run `tb done PR-1`. Expected: exit code non-zero, `MergeStatus: conflict` on task, task stays in `in-progress/`, worktree preserved, branch preserved with **rebase in-flight** (verified: `git rev-parse --git-path rebase-merge` exists). Log line appended.

- **AC5**: Resolve AC4's conflict by fixing files in the worktree, `git add`, `git rebase --continue` (verify: rebase completes, no more `rebase-merge` dir). Run `tb merge PR-1`. Expected: task moves to `done/`, `merge_target` ref now points to (formerly) `task/PR-1` HEAD, worktree removed, branch deleted, sidecar deleted, `MergeStatus` cleared.

- **AC6**: Two tasks, `max_workers = 2`, both run agents in parallel. Verify: two worktrees created, agents run concurrently without touching each other's code; both merge serially via `.merge.lock`; both end in `done/`.

- **AC7**: Cancel an agent mid-run (F4.4 flow). Expected: process killed, `AgentStatus: cancelled`, worktree preserved with whatever commits the agent made.

- **AC8a**: Kill the daemon mid-rebase. Sidecar at crash time has `phase: "rebasing"`, rebase-merge dir present in worktree. Restart. Recovery: `MergeStatus: conflict`, task stays `in-progress/`, worktree preserved with rebase mid-flight. `tb merge` after manual `git rebase --continue` finishes the task cleanly.

- **AC8b**: Kill the daemon between FF-push and file move. Sidecar has `phase: "finalizing"`, `merge_target` already contains the task branch HEAD. Restart. Recovery detects the FF happened, idempotently moves the task file to `done/`, runs the cleanup phase. No `MergeStatus: conflict` is ever set. (This is the case that the original spec mis-mapped to "conflict".)

- **AC8c**: Kill the daemon during cleanup (worktree/branch removed but sidecar still present, or partial). Restart. Recovery idempotently completes cleanup. Task is already in `done/`. The `cleanup` row of the § 4.12 table runs `git branch -d`/`-D` regardless of whether the worktree path is still on disk — verifies the previous "delete sidecar early, skip branch" bug is gone.

- **AC8d**: Kill the process between § 4.5 step 9 (sidecar → `state: active`) and step 10 (file move). Restart. Recovery sees `state: active`, `phase: active`, task still in `backlog/`. Reconciliation moves the task to `in-progress/` under `.board.lock` idempotently. Worktree + branch are intact and immediately usable.

- **AC8e**: Subsequent merges advance `<merge_target>` after a successful FF for PR-1 but before recovery runs (simulate: complete `tb done PR-1`, then complete `tb done PR-2`; manually corrupt PR-1's sidecar to look as if `phase: "finalizing"` crashed before cleanup). Recovery's `finalizing` row uses **ancestry** (`merge-base --is-ancestor`), not equality, to recognize PR-1's FF actually happened. Result: PR-1's task file is finalized to `done/`, sidecar+worktree+branch cleaned. Verifies the equality-vs-ancestry fix.

- **AC8f**: `tb merge <ID>` after a long-running manual conflict resolution during which `<merge_target>` advanced (e.g., another task merged). The retry re-runs the rebase (does not skip), surfaces the new conflict, sets `MergeStatus: conflict` again. User resolves once more, runs `tb merge` again. Verifies § 4.8 step 5's "always re-rebase" rule and that we never loop into a stale FF-push attempt.

- **AC8g**: Corrupted sidecar. Manually overwrite `<board>/.worktrees/PR-1.json` with invalid JSON. Restart daemon. Recovery logs an entry to `tb doctor` and skips PR-1; worktree+branch+task file are left untouched. Verifies § 4.13's "no auto-repair on corruption" rule.

- **AC8h**: Rebase-apply backend. Run a conflict scenario but force `git rebase` to use the apply backend (e.g., `git -c rebase.backend=apply rebase <merge_target>` or `git rebase --apply <merge_target>`; environment may need `--rebase=apply` propagated through tb). On crash mid-rebase, the in-flight state lives in `rebase-apply/`, not `rebase-merge/`. Restart daemon. Recovery's `rebasing` row probe (both `--git-path rebase-merge` AND `--git-path rebase-apply`) detects it; sets `MergeStatus: conflict`. Verifies decision 18.

- **AC8i**: Finalize crash with user intervention before recovery. Crash at `phase: "finalizing"`, FF already happened (`mergeContainsBranch` true), task in `in-progress/`. Before restart, user manually `tb mv PR-1 archive`. Restart daemon. Recovery: sees `finalizing` + `archive` + `mergeContainsBranch` true → cleans up worktree/branch/sidecar, **does not move the task file back to `done/`**. Verifies user-respect rule for the finalizing row.

Negative / safety:

- **AC9**: `tb start PR-1` when `task/PR-1` already exists → fails with clear error, no partial state.

- **AC9b**: Inject a failure mid-creation (e.g., simulate `git worktree add` failing — point `worktrees.path` at a path that becomes unwritable just before step 5). Run `tb start PR-1`. Expected: task remains in `backlog/PR-1.md` (NOT moved to `in-progress/`), sidecar is either absent or in `state: "creating"` (which the next startup recovery cleans up). No orphan branch. This verifies the "file move is the last step" atomicity invariant.

- **AC10**: `tb done PR-1` with dirty worktree → fails with `dirty_worktree`, no state change.

- **AC11**: `tb worktree clean PR-1` on a worktree with un-merged commits → fails without `--force`, refuses with explanation.

- **AC12**: Preflight — submodule detection. Repo with `.gitmodules` present, `worktrees.enabled: true` set in `.tb.yaml`. `tb start PR-1` fails with `submodules unsupported`; no worktree created, task remains in `backlog/`. Setting `worktrees.allow_submodules: true` overrides the refusal.

- **AC13**: Preflight — path conflict. `<repo root>/.tb-worktrees/foo.txt` exists as a stray file. `tb start PR-1` fails with `path conflict` instructing user to rename/remove; no partial worktree state.

- **AC14**: Push protection. Inside `.tb-worktrees/PR-1`, run `git push origin task/PR-1`. Push is rejected by the local `pre-push` hook with a clear message; nothing reaches the remote. (Verified offline against a fake remote.)

Non-functional:

- **AC15**: `tb done` of a typical task (small diff, clean merge) completes in < 2 s on a board with no contention.

- **AC16**: With `max_workers = 4`, four concurrent `tb done` calls serialize via `.merge.lock` without deadlock; total wall time ≈ sum of individual merge times, not blocked by `.board.lock`.

## 11. Decision log

| # | Decision | Rejected alternatives |
|---|---|---|
| 1 | Board lives only in main checkout; worktrees gitignore `board/`. | Per-worktree board (rejected: divergence). Separate board repo (rejected: complexity, breaks single-repo invariant). |
| 2 | Worktree creation tied to `tb start`, not to agent run. | Agent-only worktrees (rejected: humans need isolation too). |
| 3 | Rebase before merge, FF-merge after. | Plain merge commit (rejected: noisy history). `--squash` (rejected: loses agent commit granularity). Octopus merge (N/A). |
| 4 | On conflict: fail, preserve worktree, manual resolution. | Auto-resolve via agent (deferred). Drop the work (rejected). |
| 5 | Global merge-lock. | Per-target-branch lock (no benefit yet — one target per board). Optimistic concurrency (rejected: complexity for unclear gain). |
| 6 | Three distinct locks (board / per-task worktree / merge). | One mega-lock (rejected: blocks unrelated ops during slow merges). |
| 7 | `MergeStatus` as new task metadata field. | Subdir-based (rejected: status dir = primary status; conflict is orthogonal). Log-line-only (rejected: invisible to queries). |
| 8 | Existing boards opt-in via `.tb.yaml`. | Force-on (rejected: breaks running workflows). Force-off (rejected: feature doesn't ship). |
| 9 | ~~FF-merge via local ref push~~ — **superseded by decisions 15, 21, 22.** Retained for traceability: the original ref-push-only proposal failed in two ways (denyCurrentBranch when the target is checked out anywhere; blanket clean-WT prerequisite). The current algorithm is the case-A/B/C split. |
| 10 | `.tb-worktree.json` sentinel at worktree root + tb resolution order: env var > sentinel > .tb.yaml. | env-var-only (rejected: humans forget). Delete worktree-local `.tb.yaml` (rejected: makes the worktree's commits look weird; sentinel is additive). Symlink `board/` into main checkout (rejected: cross-filesystem fragility, breaks `git status`). |
| 11 | On rebase conflict: leave the rebase in flight (no `--abort`); user does `git rebase --continue` then `tb merge`. | Abort and require manual replay (rejected: user has nothing to continue from, contradicts the docs). Auto-resolve (deferred). |
| 12 | Phase-tagged sidecar (`creating → active → rebasing → finalizing → cleanup`) with explicit recovery table per phase. | Single `mergeInFlight: bool` (rejected: maps post-FF crashes to "conflict" incorrectly). No recovery (rejected: orphans accumulate). |
| 13 | `tb start` writes the sidecar *before* `git worktree add`, and moves the task file *last* under `.board.lock`. | File move first (rejected: original Codex finding — leaves task in-progress with no worktree on failure). Sidecar-last (rejected: leaves orphan worktrees uncatchable by recovery's primary scan). |
| 14 | Resolve per-worktree `.git/*` paths via `git rev-parse --git-path`. | Hard-code `worktrees/<ID>/.git/info/exclude` (rejected: linked-worktree `.git` is a pointer file, not a directory — the literal path does not exist). |
| 15 | FF-finalize is a three-way case split on `mainHead == <merge_target>` *and* on whether any **external** worktree (outside `<worktrees.path>/`) has `<merge_target>` checked out. Refined further by decision 21 (no blanket WT-clean check) and decision 22 (external-worktree refusal). | Single `git push . <branch>:<target>` (rejected: `receive.denyCurrentBranch=refuse` default). `git update-ref` directly (rejected: leaves user's WT inconsistent with HEAD). Force `receive.denyCurrentBranch=updateInstead` (rejected: silent config change, surprising side effects). |
| 16 | Recovery uses ancestry (`merge-base --is-ancestor`) to detect post-FF crashes, not ref-equality. | Equality only (rejected: a subsequent task's FF advances the ref, and ancestry-vs-equality miscounts the first task as still-pre-FF). |
| 17 | `tb merge` always re-runs the rebase after the user's manual `git rebase --continue`. | Skip rebase if user already continued (rejected: target may have advanced during manual resolution, leaving a stale rebase and a guaranteed FF-push failure). |
| 18 | Probe both `rebase-merge` and `rebase-apply` directories to detect an in-flight rebase. | Probe only `rebase-merge` (rejected: Git uses either depending on backend/config). |
| 19 | Sidecar writes use `writeFileAtomic` (temp+fsync+rename) and corrupt sidecars are surfaced to `tb doctor` rather than auto-repaired. | Non-atomic writes (rejected: a crash mid-phase-write makes the entire recovery table unreachable). Auto-repair (rejected: bad guesses can delete user work). |
| 20 | § 4.12 table is the single source of truth for recovery actions — Scan A never short-circuits on "worktree path missing". | Pre-classify orphan sidecars before consulting the table (rejected: bypassed the `creating`/`cleanup` branch-removal logic — Codex's second-pass finding). |
| 21 | No blanket `git status --porcelain` clean check on main before `merge --ff-only`. The task branch has `skip-worktree` on `board/`, so the FF diff cannot touch our canonical board state — `git merge --ff-only` itself is the gatekeeper for unrelated user dirt. | Pre-check clean main (rejected: blocked the common case where main's `board/` is dirty *by our own design* — Codex's third-pass blocker). Pre-check clean main excluding `board/` (rejected: redundant — `git merge --ff-only` already does exactly this check). |
| 22 | Enumerate ALL worktrees when picking the FF-finalize path; refuse if any external (non-`<worktrees.path>/`) worktree has `<merge_target>` checked out. | Check only the main checkout (rejected: modern git's `denyCurrentBranch` checks every linked worktree — Codex's third-pass important). Take over the external worktree and merge there (rejected: intrusive into the user's workspace). Bypass denyCurrentBranch via `-c receive.denyCurrentBranch=updateInstead` (rejected: silently mutates a user's external worktree). |
| 23 | `finalizing` recovery branches on `taskDir` and respects user intervention (don't auto-move if user moved/archived during the recovery window). | Always move to `done/` on `mergeContainsBranch` true (rejected: clobbers user actions, breaks directory=status invariant — Codex's third-pass important). |
