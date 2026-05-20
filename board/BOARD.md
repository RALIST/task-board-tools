# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-109 | Worktree-isolated task execution | 0/12 | backlog | cli |
| TB-262 | Auto-review | 0/4 | backlog | gui |
| TB-186 | Change parent task | 0/3 | backlog | gui |
| TB-292 | Epic: mirror project settings between GUI and .tb.yaml | 0/3 | ready | gui |
| TB-303 | Epic: remove generic AgentStatus field | 0/4 | backlog | workflow |

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
| TB-267 | Auto-implement: respect epic child order | 0/0 | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 11/11 | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 10/10 | gui |
| TB-204 | Show epic progress | 0/0 | gui/frontend |
| TB-205 | Setup ESLint and dead-code check for frontend | 1/1 | tooling |

## In Progress (1/3)

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| TB-266 | Daemon: reconcile autonomous stage transitions | P1 | gui | — |

## Code Review (2/3)

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| TB-269 | Docs: define staged autonomous agent workflow | P1 | docs | — |
| TB-290 | CLI: edit Context and Constraints sections | P2 | cli | — |

## Ready (6/10)

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-292 | Epic: mirror project settings between GUI and .tb.yaml | feature | P2 | XL | gui |
| TB-293 | Config: add GUI project settings to .tb.yaml schema | feature | P2 | M | cli |
| TB-294 | GUI backend: persist project settings in .tb.yaml | feature | P2 | M | gui |
| TB-295 | GUI settings panel mirrors project .tb.yaml | feature | P2 | M | gui |
| TB-304 | Auto-groom: respect ready WIP limit | bug | P2 | M | gui |
| TB-306 | Generated conventions and skill should omit autonomous flows | bug | P2 | M | cli |

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
| TB-248 | Manual macOS verification of Task Board Tools branding | improvement | P2 | S | gui |
| TB-254 | Stale recovery should write per-mode pairs for recovered terminal runs | tech-debt | P2 | S | gui |
| TB-255 | TaskDrawer marks stale per-action attribution during same-mode run | improvement | P2 | S | gui |
| TB-256 | Test TestRunQueuedAgentSync_ResumeRehydratesParentContext should assert per-mode write on daemon replay | tech-debt | P2 | S | gui |
| TB-259 | Per-task chat panel via claude/codex CLIs | feature | P1 | L | gui |
| TB-260 | Ability to edit agent prompts | feature | P2 | L | gui |
| TB-262 | Auto-review | feature | P1 | L | gui |
| TB-263 | GUI: persist auto-review setting and controls | feature | P1 | M | gui |
| TB-264 | GUI: enqueue code-review tasks for review-mode daemon runs | feature | P1 | M | gui |
| TB-265 | GUI: surface auto-review state and decisions | feature | P1 | S | gui |
| TB-272 | CLI: add managed review pass flow | feature | P1 | M | workflow |
| TB-273 | CLI: make tb init interactive | improvement | P1 | M | cli |
| TB-285 | CLI: tb scan --apply creates folder-form tasks | bug | P0 | S | cli |
| TB-298 | Allow DnD into the Archive column | improvement | P2 | S | gui-frontend |
| TB-299 | Auto-implement: gate on per-mode ImplementStatus, clear AgentStatus on tb ready | tech-debt | P1 | S | gui |
| TB-301 | GUI: add startup grace before automation pickup | bug | P2 | M | gui |
| TB-302 | Board switch: auto runned tasks behaviour | bug | P2 | M |  |
| TB-303 | Epic: remove generic AgentStatus field | tech-debt | P2 | XL | workflow |
| TB-305 | CLI: install project task-board skills during init | improvement | P2 | M | cli |
| TB-307 | CLI: replace AgentStatus metadata with per-mode status fields | tech-debt | P2 | M | cli |
| TB-308 | GUI: run lifecycle uses per-mode statuses only | tech-debt | P2 | L | gui |
| TB-309 | Frontend: remove generic AgentStatus display dependency | improvement | P2 | M | gui-frontend |
| TB-310 | Docs and board cleanup for AgentStatus removal | tech-debt | P2 | M | docs |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-300 | Auto-implement: respect WIP and CPU worker budget | bug | gui |
| TB-297 | Remove auto-implement filter from settings UI | tech-debt | gui |
| TB-296 | Refactor GUI header: condense controls so they fit on one row | improvement | gui |
| TB-291 | Auto-resume interrupted tasks in auto-groom and auto-implement coordinators | bug | gui |
| TB-289 | Extend tb ls with multi-value filter flags + --agent + --search | feature | cli |
| TB-288 | FilterBar-driven auto-implement query (replaces text DSL) | feature | gui |
| TB-287 | Flaky race in TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart | bug | gui |
| TB-286 | Show readable GUI error toasts | bug | gui-frontend |
| TB-271 | Fix Codex post-tool hook timeout | bug | tooling |
| TB-270 | Align agent prompts with staged kanban workflow | improvement | agent |
| TB-268 | Review-failed handoff clears retry-blocking agent state | bug | workflow |
| TB-267 | Auto-implement: respect epic child order | feature | gui |
| TB-261 | Safely clean up orphaned agent processes | improvement | gui |
| TB-253 | GUI Run History shows multiple concurrent RUNNING rows for one task | bug | gui |
| TB-252 | Allow Resume when session_id is present regardless of AgentStatus | improvement | gui |
| TB-251 | Distinguish agent-failed from daemon-lost in recovery | improvement | gui |
| TB-250 | Resolve GUI golangci-lint baseline findings | tech-debt | tooling |
| TB-249 | Resolve CLI golangci-lint baseline findings | tech-debt | tooling |
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
