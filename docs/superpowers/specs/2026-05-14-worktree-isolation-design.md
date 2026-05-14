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

- When a worktree is created for a task, `tb` writes `worktrees/<ID>/.git/info/exclude` adding `board/` so any accidental edits there don't reach the branch.
- The agent run sets `TB_BOARD_PATH=<absolute path to canonical board>` in the agent process env. `tb` (invoked from within the worktree) resolves the board via, in order: (1) `TB_BOARD_PATH`, (2) `.tb.yaml` upward search starting at cwd. (1) wins.
- Folder-form task contents (`board/<status>/<ID>/attachments/`, task-local `.agent-state.jsonl`, `.agent-logs/`) live in the canonical board. The agent writing to its task folder writes to canonical board, not to the worktree copy. Same `TB_BOARD_PATH` mechanism.
- `.agent-state/` and `.agent-logs/` (the non-folder-form locations) likewise stay in canonical board.

**Invariant:** worktrees never write to `board/`. If they do, the writes are gitignored and discarded; the canonical board is the only durable home for board state.

## 4. Worktree lifecycle

### 4.1 Layout

Worktrees live at `<repo root>/.tb-worktrees/<task_id>/`. The `.tb-worktrees/` directory is added to `.gitignore` and to `git worktree`'s own administrative tracking.

Path is configurable per board via `.tb.yaml` → `worktrees.path` (relative to repo root). Default `.tb-worktrees`.

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

Algorithm:

1. Take `.board.lock`. Move file `backlog/<ID>.md` → `in-progress/<ID>.md` (or folder form). Record Log line. Regenerate. Release `.board.lock`.
2. Take `.worktree.lock` (new per-task lock file at `board/.worktree-locks/<ID>.lock` — distinct from `.board.lock` to avoid blocking board mutations during git operations). Note: per-task lock files allow concurrent `tb start` on *different* tasks.
3. Resolve merge target: `git rev-parse --verify <merge_target>` — error if missing.
4. Resolve branch name. If `task/<ID>` already exists locally or remotely, fail loudly — do not auto-overwrite. (Failed/abandoned earlier runs left a branch; user must explicitly `tb worktree clean <ID>` first.)
5. `git worktree add -b task/<ID> .tb-worktrees/<ID> <merge_target>`.
6. Append `board/` to `.tb-worktrees/<ID>/.git/info/exclude` and run `git update-index --skip-worktree board/` inside the worktree to be safe.
7. Write the worktree's path + base SHA into a sidecar `board/.worktrees/<ID>.json`:
   ```json
   { "path": ".tb-worktrees/PR-100", "branch": "task/PR-100", "base": "main", "baseSHA": "abc123…", "createdAt": "…" }
   ```
   This sidecar is what `tb` consults; the source of truth at runtime is still `git worktree list`, but the sidecar carries our extra metadata (merge target, base SHA, status).
8. Release `.worktree.lock`.

Idempotent: if a worktree already exists for `<ID>`, step 4 detects it and skips creation. Useful for the daemon, which may be asked to "ensure worktree" repeatedly.

For the daemon, the agent-run path calls the same code through a thin wrapper (`EnsureWorktree(ID)`); the daemon doesn't reimplement git plumbing.

### 4.6 Agent execution inside the worktree

When the daemon spawns an agent for a task whose worktree exists, it sets:

- `cwd = <repo root>/.tb-worktrees/<ID>`
- `TB_BOARD_PATH = <repo root>/board` (canonical)
- (Optional, M-WT.2) `TB_TASK_ID = <ID>` for prompt scaffolding.

The agent prompt gains a small footer reminding it: "You are operating inside a git worktree at `<cwd>`. Commit your changes to the current branch. Do not switch branches. Do not run `git push`. The canonical task board is at `$TB_BOARD_PATH` — use `tb` normally for task operations."

The agent commits to `task/<ID>` as it works. No push, no rebase mid-run, no branch switching.

### 4.7 Completion: `tb done <ID>`

`tb done` becomes a multi-step operation when worktree mode is on:

1. Take `.worktree.lock` for `<ID>`.
2. Read sidecar; resolve worktree path & branch.
3. If worktree has uncommitted changes → fail with `dirty_worktree`. Don't move task. User must commit or stash.
4. Take the global **merge-lock** (`board/.merge.lock`, flock). This serializes all merges into the merge target across the daemon and direct CLI invocations.
5. Inside the worktree:
   - `git fetch` if `worktrees.fetch_before_merge: true` (default `false`; opt-in for boards using a remote merge target).
   - `git rebase <merge_target>`. If conflict → `git rebase --abort`, release merge-lock, set `MergeStatus: conflict` on the task (new metadata field), append Log entry, release `.worktree.lock`, return error to caller. Worktree + branch preserved.
6. If rebase clean: `cd` to main checkout, `git merge --ff-only task/<ID>`. With rebase done, this is a fast-forward.
   - If FF fails (shouldn't, but: someone advanced main between rebase and FF) → release merge-lock, error `merge_target_advanced`, leave the rebased branch (next `tb merge <ID>` will redo it).
7. Move task file `in-progress/<ID>.md` → `done/<ID>.md` (or folder form), append Log "Done — merged at <SHA>".
8. Cleanup: `git worktree remove .tb-worktrees/<ID>` and `git branch -d task/<ID>`. Delete sidecar `board/.worktrees/<ID>.json`.
9. `regenerateBoard`.
10. Release merge-lock; release `.worktree.lock`.

The merge-lock is held across `git rebase` and `git merge --ff-only`. Other `tb done` calls block until done. Lock scope is small (typically seconds), so contention is acceptable. Other board mutations (`tb edit`, `tb create`) are not blocked — they take `.board.lock` instead.

### 4.8 Retry after conflict: `tb merge <ID>` (new command)

When a task has `MergeStatus: conflict`, the user resolves the conflict manually inside the worktree (which is preserved), commits the resolution, and runs:

```
tb merge <ID>
```

This is `tb done`'s step 4 onward — `.merge.lock`, rebase (now expected clean), FF, move to `done/`, cleanup. If the user's rebase resolution introduced new code, that's their responsibility.

If `tb merge <ID>` is run with a clean worktree but `MergeStatus` not set to conflict — same operation, idempotent. (Lets a user finalize a task they manually marked merge-ready.)

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

Existing M5 stale-recovery is extended: for each task with a sidecar in `board/.worktrees/`, on startup:

- If the recorded branch no longer exists in git → sidecar is orphan, clean it up.
- If the worktree directory is missing → orphan sidecar, clean it up.
- If the worktree exists but is dirty and the task is not `in-progress` → log a warning visible in `tb doctor` (new diagnostic command, M-WT.2).
- If a merge was in progress (sidecar has `mergeInFlight: true`) — set `MergeStatus: conflict` and let the user inspect.

The `mergeInFlight` flag is written into the sidecar at step 4 of § 4.7 and cleared at step 10. Crash between those steps is detectable.

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
| `board/.worktree-locks/<ID>.lock` | per-task | worktree create / destroy / merge of *this* task | other worktree ops on the same task |
| `board/.merge.lock` | board-wide | rebase + FF-merge | other merges (across all tasks) |

Why three:

- `.board.lock` is short-lived (milliseconds — a single file move + regenerate). It must not be held during long git operations.
- Per-task `.worktree-locks/<ID>.lock` lets two different tasks have their worktrees created/destroyed in parallel.
- `.merge.lock` is global and held during the (potentially slow) rebase+merge. It serializes merges into `merge_target` so we never have two FF-merges racing for the same branch tip.

Ordering rule: **never acquire `.merge.lock` while holding `.board.lock`**. The merge code (§ 4.7) takes locks in order `.worktree.lock(ID)` → `.merge.lock` → (briefly, at step 7) `.board.lock` for the file move, releasing each in reverse. Every other path that touches `.board.lock` (tb edit/create/mv/etc.) never reaches for `.merge.lock`. This rules out the only possible cycle.

A consequence: while `.merge.lock` is held (typically seconds during rebase + FF), other board mutations from other processes are *not* blocked — they queue only briefly on `.board.lock` during step 7 of the merge.

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

- **Submodules.** `git worktree` interaction with submodules is fragile. The spec assumes no submodules. If the board's repo has submodules, mode should be disabled and a warning logged.

- **LFS.** `git worktree` + Git LFS works but doubles LFS bandwidth on creation. Same advice: document, don't auto-handle.

- **Agent that runs `git push` anyway.** The prompt footer asks the agent not to. We don't enforce — a misbehaving agent could push `task/<ID>` to the remote. Acceptable today; an agent-side `pre-push` hook can be considered later.

- **Worktree path conflicts** with files the user already has at `.tb-worktrees/`. Loudly fail at `tb init`/first-use if the path is non-empty.

- **Agent in worktree creates a sibling task via `tb create`**. The new task's file lands in canonical board (good). The agent's branch doesn't see it (good — sibling tasks are independent). If the new task is a child of an in-progress epic, the epic's child list now includes it — daemon picks it up normally.

- **What if two agents need the same dependency, both edit `package.json`?** Worktree isolates code edits — both agents commit non-conflicting edits to their own branches. At merge time the second one to merge rebases on top of the first; trivial JSON conflicts can still arise. This is by design — the system surfaces those rather than silently merging.

## 10. Acceptance criteria

Functional:

- **AC1**: Fresh board with `worktrees.enabled: true`. `tb create "x"` then `tb start PR-1`. Expected: `.tb-worktrees/PR-1/` exists, branch `task/PR-1` exists at `merge_target`'s HEAD, sidecar `board/.worktrees/PR-1.json` present, task file in `in-progress/`.

- **AC2**: Inside the worktree, `tb edit PR-1 -d "new desc"` modifies the canonical board (visible from main checkout) and does **not** modify the worktree's `board/` (verified by `git status` in worktree being clean except for any agent edits).

- **AC3**: Commit a change in the worktree, run `tb done PR-1`. Expected: task moves to `done/`, branch FF-merged into `merge_target`, worktree removed, branch deleted, sidecar removed.

- **AC4**: Set up a conflict: in main, advance `merge_target` with a conflicting change after `tb start`. Commit work in worktree. Run `tb done PR-1`. Expected: exit code non-zero, `MergeStatus: conflict` on task, task stays in `in-progress/`, worktree preserved, branch preserved with rebase aborted. Log line appended.

- **AC5**: Resolve AC4's conflict manually inside the worktree (`git rebase --continue` after fixing files), then `tb merge PR-1`. Expected: task moves to `done/`, `MergeStatus` cleared, worktree/branch cleaned up.

- **AC6**: Two tasks, `max_workers = 2`, both run agents in parallel. Verify: two worktrees created, agents run concurrently without touching each other's code; both merge serially via `.merge.lock`; both end in `done/`.

- **AC7**: Cancel an agent mid-run (F4.4 flow). Expected: process killed, `AgentStatus: cancelled`, worktree preserved with whatever commits the agent made.

- **AC8**: Kill the daemon during the rebase step (between worktree creation and merge completion). Restart. Expected: sidecar shows `mergeInFlight: true`; recovery sets `MergeStatus: conflict` so user can inspect.

Negative / safety:

- **AC9**: `tb start PR-1` when `task/PR-1` already exists → fails with clear error, no partial state.

- **AC10**: `tb done PR-1` with dirty worktree → fails with `dirty_worktree`, no state change.

- **AC11**: `tb worktree clean PR-1` on a worktree with un-merged commits → fails without `--force`, refuses with explanation.

Non-functional:

- **AC12**: `tb done` of a typical task (small diff, clean merge) completes in < 2 s on a board with no contention.

- **AC13**: With `max_workers = 4`, four concurrent `tb done` calls serialize via `.merge.lock` without deadlock; total wall time ≈ sum of individual merge times, not blocked by `.board.lock`.

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
