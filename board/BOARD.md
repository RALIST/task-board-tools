# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-177 | Auto task implementation | 0/4 | backlog | gui |
| TB-109 | Worktree-isolated task execution | 0/12 | backlog | cli |
| TB-172 | Auto-groom | 0/3 | backlog | gui |
| TB-186 | Change parent task | 0/3 | backlog | gui |
| TB-205 | Setup ESLint and dead-code check for frontend | 0/1 | code-review | tooling |

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
| TB-93 | Move from file-based to folder-based approach | 42/42 | cli |
| TB-1 | M1: CLI extensions for GUI integration | 8/8 | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 9/9 | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 10/10 | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | 10/10 | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 10/10 | gui |
| TB-130 | Agent session resume + interrupted-run recovery | 12/12 | gui |
| TB-182 | Add special labes\tags\status for user attention | 3/3 | agent |
| TB-194 | Code-review column | 6/6 | workflow |
| TB-239 | Canonical Kanban: add ready column + WIP/pull mechanics | 0/0 | core |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 11/11 | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 10/10 | gui |
| TB-204 | Show epic progress | 0/0 | gui/frontend |

## In Progress (2/2 ⚠)

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| TB-176 | Track PID of launched agents | P2 | gui | — |
| TB-206 | Setup golangci-lint for project and initial run it | P2 | tooling | — |

## Code Review

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| TB-205 | Setup ESLint and dead-code check for frontend | P2 | tooling | — |
| TB-237 | Save diffrent agent actions in diffrent fields | P2 | cli | — |
| TB-238 | Update implement.md agent prompt to set ReviewRef before submit | P2 | workflow | — |

## Ready

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| — | — | — | — | — | — |

## Backlog

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-109 | Worktree-isolated task execution | feature | P1 | L | cli |
| TB-111 | Worktree config + preflight + gitignore | feature | P1 | S | cli |
| TB-112 | Board path resolution from inside a worktree | feature | P1 | M | cli |
| TB-113 | tb start: worktree creation flow | feature | P1 | M | cli |
| TB-114 | Agent daemon: run agent inside worktree | feature | P1 | S | gui |
| TB-115 | tb done: three-case FF + merge-lock + phase transitions | feature | P1 | L | cli |
| TB-116 | tb merge: conflict retry path | feature | P1 | M | cli |
| TB-117 | Worktree recovery on startup | feature | P1 | M | cli |
| TB-118 | tb worktree adopt/list/status + cancel/fail preservation | feature | P2 | M | cli |
| TB-119 | Worktree flow: acceptance criteria as integration tests | feature | P1 | L | cli |
| TB-120 | Per-task MergeTarget override | feature | P1 | S | cli |
| TB-121 | tb doctor: surface unresolvable worktree state | feature | P1 | S | cli |
| TB-122 | tb worktree clean: removal command | feature | P1 | S | cli |
| TB-128 | Keep ”Done” column sorted by timestamp, not priority | improvement | P2 | M | gui |
| TB-143 | Add semver to cli tool | feature | P2 | M | cli |
| TB-172 | Auto-groom | feature | P1 | L | gui |
| TB-173 | GUI: persist auto-groom setting and toggle | feature | P1 | M | gui |
| TB-174 | GUI: auto-groom triage tasks via groom-mode daemon runs | feature | P1 | M | gui |
| TB-175 | GUI: surface auto-groom feedback and manual fallback | feature | P1 | S | gui |
| TB-177 | Auto task implementation | feature | P0 | L | gui |
| TB-178 | GUI: persist auto-implement settings and query | feature | P0 | M | gui |
| TB-179 | GUI: enqueue auto-implement candidates from daemon | feature | P0 | M | gui |
| TB-180 | GUI: show auto-implement controls and feedback | feature | P0 | S | gui |
| TB-186 | Change parent task | feature | P2 | L | gui |
| TB-187 | Quick add task to epic | improvement | P2 | M | gui/frontend |
| TB-188 | Quick jump to child ticket | improvement | P2 | S | gui |
| TB-189 | Quick jump to parent task | improvement | P2 | S | gui/frontend |
| TB-191 | CLI: safely reassign a task parent | feature | P2 | M | cli |
| TB-192 | GUI backend: expose parent reassignment | improvement | P2 | S | gui |
| TB-193 | TaskDrawer: edit parent epic from task page | improvement | P2 | M | gui/frontend |
| TB-233 | Auto-implement priority: rank review-failed backlog tasks first | improvement | P2 | S | gui |
| TB-234 | Daemon should not auto-pick up tasks in code-review | bug | P2 | S | gui |
| TB-241 | GUI: Resume button enabled for interrupted tasks with no captured session | bug | P2 | S | gui |
| TB-242 | Agent runner blocks on stdout EOF when child processes inherit pipes | bug | P1 | M | gui |
| TB-244 | Periodic re-recovery for stale agent runs | improvement | P2 | M | gui |
| TB-246 | Regenerate darwin/Assets.car on working Xcode env | tech-debt | P2 | S | gui |
| TB-247 | Triage TB-205 knip first-pass findings | tech-debt | P2 | S | gui-frontend |
| TB-248 | Manual macOS verification of Task Board Tools branding | improvement | P2 | S | gui |
| TB-249 | Resolve CLI golangci-lint baseline findings | tech-debt | P2 | M | tooling |
| TB-250 | Resolve GUI golangci-lint baseline findings | tech-debt | P2 | M | tooling |
| TB-251 | Distinguish agent-failed from daemon-lost in recovery | improvement | P1 | M | gui |
| TB-252 | Allow Resume when session_id is present regardless of AgentStatus | improvement | P1 | S | gui |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-239 | Canonical Kanban: add ready column + WIP/pull mechanics | feature | core |
| TB-236 | macOS titlebar double-click should zoom/restore window | bug | gui |
| TB-235 | Require ReviewRef metadata before code-review moves | improvement | workflow |
| TB-232 | tb-gui usage tap: chain to user's original statusline instead of replacing it | bug | gui |
| TB-231 | Address TB-130 adversarial review findings | bug | gui |
| TB-230 | CLI: avoid backups when init content is unchanged | bug | cli |
| TB-229 | CLI: reconcile .tb.yaml with annotated config template | improvement | cli |
| TB-228 | CLI: make init refresh existing boards by default | improvement | cli |
| TB-227 | CLI: refresh generated board docs for existing boards | improvement | cli |
| TB-226 | CLI: preserve literal command examples in task creation | bug | cli |
| TB-225 | Ask user to init board from GUI when he tries to open folder without initialized tb status | feature | gui |
| TB-224 | Support task-root attachments | improvement | cli/gui |
| TB-223 | Audit secondary docs for stale layout/status | improvement | docs |
| TB-222 | Refresh README for current repo layout | improvement | docs |
| TB-221 | Prepare repository for GitHub push | tech-debt | tooling |
| TB-219 | Manual QA: running agent remains queued and cannot be cancelled | bug | gui |
| TB-218 | QA probe GUI create dialog | bug | gui/frontend |
| TB-217 | Manual QA: attachment removal mis-parses dash-leading filename | bug | cli |
| TB-216 | QA probe legacy file parity | bug | cli |
| TB-215 | QA probe groom placeholder | bug | gui |
| TB-214 | QA probe daemon pickup | bug | gui |
| TB-213 | QA probe agent run | bug | gui |
| TB-212 | QA probe folder attachments | bug | cli |
| TB-211 | QA probe CLI happy edited | improvement | cli |
| TB-210 | Manual QA: MVP live-board pass | spike | gui |
| TB-208 | Switch projects bug | bug | gui |
| TB-207 | Allow tasl title edit from GUI | improvement | gui |
| TB-204 | Show epic progress | improvement | gui/frontend |
| TB-203 | obfuscation agents logs and tasks | bug | gui |
| TB-202 | Create proper name and icon for app | improvement | gui |
| TB-201 | MacOS: window buttons hides header | bug | gui |
| TB-200 | Docs: document code-review workflow | improvement | docs |
| TB-199 | Workflow: review-failed marker and retry priority | feature | agent |
| TB-198 | Agent: add review mode and findings section | feature | agent |
| TB-197 | GUI: show code-review column and review fields | feature | gui |
| TB-196 | CLI: add review target and notes commands | feature | cli |
| TB-195 | CLI: add code-review status and submit flow | feature | cli |
| TB-194 | Code-review column | feature | workflow |
| TB-190 | Implement autosave instead of save with buttons | improvement | gui |
| TB-185 | GUI: surface user-attention state and automation guard | feature | gui |
| TB-184 | Docs: define user-attention handoff protocol | improvement | docs |
| TB-183 | CLI: add user-attention agent status and note section | feature | cli |
| TB-182 | Add special labes\tags\status for user attention | feature | agent |
| TB-181 | Persist draft task/prevent close unsaved form | bug | gui |
| TB-171 | TB-93/REVIEW: re-run Codex cross-cutting architectural review (previous run stalled) | spike | gui |
| TB-170 | TB-93/GUI: resolveArtifactPaths hot path - 8 stats per agent log line, cache layout | improvement | gui |
| TB-169 | TB-93/GUI: attachment size display polish - IEC unit labels and exact-byte tooltip | tech-debt | gui |
| TB-168 | TB-93/GUI: test infra cleanup - hardcoded sleeps, /tmp/tb fallback, idDirRe negative case | tech-debt | gui |
| TB-167 | TB-93/CLI: minor polish - attach help-text grouping, --rm=false ambiguity, doc step ordering | tech-debt | cli |
| TB-166 | TB-93/GUI: folder_tasks_test.go uses temp/staging names that don't match the CLI's real pattern | tech-debt | gui |
