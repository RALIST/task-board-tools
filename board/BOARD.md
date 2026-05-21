# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-109 | Worktree-isolated task execution | 0/12 | backlog | cli |
| TB-186 | Change parent task | 0/3 | backlog | gui |
| TB-292 | Epic: mirror project settings between GUI and .tb.yaml | 0/3 | ready | gui |
| TB-303 | Epic: remove generic AgentStatus field | 0/4 | ready | workflow |

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
| TB-93 | Move from file-based to folder-based approach | 42/42 | cli |
| TB-177 | Auto task implementation | 6/6 | gui |
| TB-1 | M1: CLI extensions for GUI integration | 8/8 | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 9/9 | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 10/10 | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | 10/10 | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 10/10 | gui |
| TB-130 | Agent session resume + interrupted-run recovery | 12/12 | gui |
| TB-172 | Auto-groom | 3/3 | gui |
| TB-182 | Add special labes\tags\status for user attention | 3/3 | agent |
| TB-194 | Code-review column | 6/6 | workflow |
| TB-239 | Canonical Kanban: add ready column + WIP/pull mechanics | 0/0 | core |
| TB-262 | Auto-review | 4/4 | gui |
| TB-267 | Auto-implement: respect epic child order | 0/0 | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 11/11 | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 10/10 | gui |
| TB-204 | Show epic progress | 0/0 | gui/frontend |
| TB-205 | Setup ESLint and dead-code check for frontend | 1/1 | tooling |

## In Progress (3/3 ⚠)

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| TB-313 | GUI: virtualize large kanban columns | P2 | gui | — |
| TB-318 | GUI: show loading state during board switch | P2 | gui | — |
| TB-320 | GUI: make startup grace header a live countdown | P2 | gui/frontend | — |

## Code Review (0/3)

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| — | — | — | — | — |

## Ready (7/10)

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-292 | Epic: mirror project settings between GUI and .tb.yaml | feature | P2 | XL | gui |
| TB-293 | Config: add GUI project settings to .tb.yaml schema | feature | P2 | M | cli |
| TB-294 | GUI backend: persist project settings in .tb.yaml | feature | P2 | M | gui |
| TB-295 | GUI settings panel mirrors project .tb.yaml | feature | P2 | M | gui |
| TB-303 | Epic: remove generic AgentStatus field | tech-debt | P2 | XL | workflow |
| TB-310 | Docs and board cleanup for AgentStatus removal | tech-debt | P2 | M | docs |
| TB-317 | CLI: add --agent-status filter for running agent tasks | feature | P2 | S | cli |

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
| TB-143 | Add semver to cli tool | feature | P2 | M | cli |
| TB-186 | Change parent task | feature | P2 | L | gui |
| TB-187 | Quick add task to epic | improvement | P2 | M | gui/frontend |
| TB-188 | Quick jump to child ticket | improvement | P2 | S | gui |
| TB-189 | Quick jump to parent task | improvement | P2 | S | gui/frontend |
| TB-191 | CLI: safely reassign a task parent | feature | P2 | M | cli |
| TB-192 | GUI backend: expose parent reassignment | improvement | P2 | S | gui |
| TB-193 | TaskDrawer: edit parent epic from task page | improvement | P2 | M | gui/frontend |
| TB-246 | Regenerate darwin/Assets.car on working Xcode env | tech-debt | P2 | S | gui |
| TB-254 | Stale recovery should write per-mode pairs for recovered terminal runs | tech-debt | P2 | S | gui |
| TB-255 | TaskDrawer marks stale per-action attribution during same-mode run | improvement | P2 | S | gui |
| TB-256 | Test TestRunQueuedAgentSync_ResumeRehydratesParentContext should assert per-mode write on daemon replay | tech-debt | P2 | S | gui |
| TB-259 | Per-task chat panel via claude/codex CLIs | feature | P1 | L | gui |
| TB-260 | Ability to edit agent prompts | feature | P2 | L | gui |
| TB-273 | CLI: make tb init interactive | improvement | P1 | M | cli |
| TB-285 | CLI: tb scan --apply creates folder-form tasks | bug | P0 | S | cli |
| TB-298 | Allow DnD into the Archive column | improvement | P2 | S | gui-frontend |
| TB-299 | Auto-implement: gate on per-mode ImplementStatus, clear AgentStatus on tb ready | tech-debt | P1 | S | gui |
| TB-307 | CLI: replace AgentStatus metadata with per-mode status fields | tech-debt | P2 | M | cli |
| TB-308 | GUI: run lifecycle uses per-mode statuses only | tech-debt | P2 | L | gui |
| TB-309 | Frontend: remove generic AgentStatus display dependency | improvement | P2 | M | gui-frontend |
| TB-311 | Manual smoke board-switch cancellation in desktop GUI | spike | P2 | S | gui |
| TB-314 | GUI: opening a board can start queued agent runs during smoke/load tests | bug | P1 | M | gui |
| TB-316 | Profile GUI idle CPU usage | bug | P1 | S | gui |
| TB-319 | CLI: clear review-failed when submitting retried in-progress work | bug | P1 | S | workflow |
| TB-321 | GUI: board view misses live running automation tasks | bug | P1 | M | gui/frontend |
| TB-322 | check if app closing kills agnets | bug | P2 | M |  |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-315 | Auto-groom and auto-resume must respect worker budget | bug | gui |
| TB-312 | GUI: Open project selection does not switch board | bug | gui |
| TB-306 | Generated conventions and skill should omit autonomous flows | bug | cli |
| TB-305 | CLI: install project task-board skills during init | improvement | cli |
| TB-304 | Auto-groom: respect ready WIP limit | bug | gui |
| TB-302 | GUI board switch cancels old-board auto-runs cleanly | bug | gui |
| TB-301 | GUI: add startup grace before automation pickup | bug | gui |
| TB-300 | Auto-implement: respect WIP and CPU worker budget | bug | gui |
| TB-297 | Remove auto-implement filter from settings UI | tech-debt | gui |
| TB-296 | Refactor GUI header: condense controls so they fit on one row | improvement | gui |
| TB-291 | Auto-resume interrupted tasks in auto-groom and auto-implement coordinators | bug | gui |
| TB-290 | CLI: edit Context and Constraints sections | improvement | cli |
| TB-289 | Extend tb ls with multi-value filter flags + --agent + --search | feature | cli |
| TB-288 | FilterBar-driven auto-implement query (replaces text DSL) | feature | gui |
| TB-287 | Flaky race in TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart | bug | gui |
| TB-286 | Show readable GUI error toasts | bug | gui-frontend |
| TB-272 | CLI: add managed review pass flow | feature | workflow |
| TB-271 | Fix Codex post-tool hook timeout | bug | tooling |
| TB-270 | Align agent prompts with staged kanban workflow | improvement | agent |
| TB-269 | Docs: define staged autonomous agent workflow | improvement | docs |
| TB-268 | Review-failed handoff clears retry-blocking agent state | bug | workflow |
| TB-267 | Auto-implement: respect epic child order | feature | gui |
| TB-266 | Daemon: reconcile autonomous stage transitions | improvement | gui |
| TB-265 | GUI: surface auto-review state and decisions | feature | gui |
| TB-264 | GUI: enqueue code-review tasks for review-mode daemon runs | feature | gui |
| TB-263 | GUI: persist auto-review setting and controls | feature | gui |
| TB-262 | Auto-review | feature | gui |
| TB-261 | Safely clean up orphaned agent processes | improvement | gui |
| TB-253 | GUI Run History shows multiple concurrent RUNNING rows for one task | bug | gui |
| TB-252 | Allow Resume when session_id is present regardless of AgentStatus | improvement | gui |
| TB-251 | Distinguish agent-failed from daemon-lost in recovery | improvement | gui |
| TB-250 | Resolve GUI golangci-lint baseline findings | tech-debt | tooling |
| TB-249 | Resolve CLI golangci-lint baseline findings | tech-debt | tooling |
| TB-248 | Manual macOS verification of Task Board Tools branding | improvement | gui |
| TB-247 | Triage TB-205 knip first-pass findings | tech-debt | gui-frontend |
| TB-244 | Periodic re-recovery for stale agent runs | improvement | gui |
| TB-242 | Agent runner blocks on stdout EOF when child processes inherit pipes | bug | gui |
| TB-241 | GUI: Resume button enabled for interrupted tasks with no captured session | bug | gui |
| TB-239 | Canonical Kanban: add ready column + WIP/pull mechanics | feature | core |
| TB-238 | Update implement.md agent prompt to set ReviewRef before submit | improvement | workflow |
| TB-237 | Save diffrent agent actions in diffrent fields | improvement | cli |
| TB-236 | macOS titlebar double-click should zoom/restore window | bug | gui |
| TB-235 | Require ReviewRef metadata before code-review moves | improvement | workflow |
| TB-233 | Auto-implement priority: rank review-failed ready tasks first | improvement | gui |
| TB-232 | tb-gui usage tap: chain to user's original statusline instead of replacing it | bug | gui |
| TB-231 | Address TB-130 adversarial review findings | bug | gui |
| TB-230 | CLI: avoid backups when init content is unchanged | bug | cli |
| TB-229 | CLI: reconcile .tb.yaml with annotated config template | improvement | cli |
| TB-228 | CLI: make init refresh existing boards by default | improvement | cli |
| TB-227 | CLI: refresh generated board docs for existing boards | improvement | cli |
