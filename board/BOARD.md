# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-177 | Auto task implementation | 0/3 | backlog | gui |
| TB-109 | Worktree-isolated task execution | 0/12 | backlog | cli |
| TB-130 | Agent session resume + interrupted-run recovery | 5/12 | backlog | gui |
| TB-172 | Auto-groom | 0/3 | backlog | gui |
| TB-182 | Add special labes\tags\status for user attention | 0/3 | backlog | agent |
| TB-194 | Code-review column | 0/6 | backlog | workflow |
| TB-186 | Change parent task | 0/3 | backlog | gui |
| TB-204 | Show epic progress | 0/0 | backlog | gui/frontend |

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
| TB-93 | Move from file-based to folder-based approach | 42/42 | cli |
| TB-1 | M1: CLI extensions for GUI integration | 8/8 | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 9/9 | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 10/10 | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | 10/10 | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 10/10 | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 11/11 | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 10/10 | gui |

## In Progress

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| — | — | — | — | — |

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
| TB-130 | Agent session resume + interrupted-run recovery | feature | P1 | XL | gui |
| TB-136 | Codex session capture via --json OnSessionID callback | feature | P1 | M | gui |
| TB-137 | Recovery: dead-PID + SessionID -> interrupted (markInterrupted) | feature | P1 | M | gui |
| TB-138 | Resume backend Claude: ResumeDecorator + -r flag + cwd/env replay | feature | P1 | M | gui |
| TB-139 | Resume backend Codex: codex exec --json resume <uuid> wiring | feature | P1 | M | gui |
| TB-140 | Frontend: Resume button, interrupted pill, resumed_from chip | feature | P1 | M | gui |
| TB-141 | Fake-runner integration test: kill->interrupted->resume cycle | feature | P1 | M | gui |
| TB-142 | Docs sweep: ARCHITECTURE.md + CLAUDE.md + FEATURES.md for resume | improvement | P1 | S | docs |
| TB-143 | Add semver to cli tool | feature | P2 | M | cli |
| TB-172 | Auto-groom | feature | P1 | L | gui |
| TB-173 | GUI: persist auto-groom setting and toggle | feature | P1 | M | gui |
| TB-174 | GUI: auto-groom triage tasks via groom-mode daemon runs | feature | P1 | M | gui |
| TB-175 | GUI: surface auto-groom feedback and manual fallback | feature | P1 | S | gui |
| TB-176 | Track PID of launched agents | bug | P2 | M | gui |
| TB-177 | Auto task implementation | feature | P0 | L | gui |
| TB-178 | GUI: persist auto-implement settings and query | feature | P0 | M | gui |
| TB-179 | GUI: enqueue auto-implement candidates from daemon | feature | P0 | M | gui |
| TB-180 | GUI: show auto-implement controls and feedback | feature | P0 | S | gui |
| TB-182 | Add special labes\tags\status for user attention | feature | P1 | L | agent |
| TB-183 | CLI: add user-attention agent status and note section | feature | P1 | M | cli |
| TB-184 | Docs: define user-attention handoff protocol | improvement | P1 | S | docs |
| TB-185 | GUI: surface user-attention state and automation guard | feature | P1 | M | gui |
| TB-186 | Change parent task | feature | P2 | L | gui |
| TB-187 | Quick add task to epic | improvement | P2 | M | gui/frontend |
| TB-188 | Quick jump to child ticket | improvement | P2 | S | gui |
| TB-189 | Quick jump to parent task | improvement | P2 | S | gui/frontend |
| TB-190 | Implement autosave instead of save with buttons | improvement | P2 | M | gui |
| TB-191 | CLI: safely reassign a task parent | feature | P2 | M | cli |
| TB-192 | GUI backend: expose parent reassignment | improvement | P2 | S | gui |
| TB-193 | TaskDrawer: edit parent epic from task page | improvement | P2 | M | gui/frontend |
| TB-194 | Code-review column | feature | P1 | XL | workflow |
| TB-195 | CLI: add code-review status and submit flow | feature | P1 | M | cli |
| TB-196 | CLI: add review target and notes commands | feature | P1 | M | cli |
| TB-197 | GUI: show code-review column and review fields | feature | P1 | M | gui |
| TB-198 | Agent: add review mode and findings section | feature | P1 | M | agent |
| TB-199 | Workflow: review-failed marker and retry priority | feature | P1 | M | agent |
| TB-200 | Docs: document code-review workflow | improvement | P1 | S | docs |
| TB-202 | Create proper name and icon for app | improvement | P2 | S | gui |
| TB-204 | Show epic progress | improvement | P2 | M | gui/frontend |
| TB-205 | Setup esling and deadcode check for frontend | tech-debt | P2 | M | tooling |
| TB-206 | Setup golangci-lint for project and initial run it | tech-debt | P2 | M | tooling |
| TB-226 | CLI: preserve literal command examples in task creation | bug | P1 | S | cli |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-230 | CLI: avoid backups when init content is unchanged | bug | cli |
| TB-229 | CLI: reconcile .tb.yaml with annotated config template | improvement | cli |
| TB-228 | CLI: make init refresh existing boards by default | improvement | cli |
| TB-227 | CLI: refresh generated board docs for existing boards | improvement | cli |
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
| TB-203 | obfuscation agents logs and tasks | bug | gui |
| TB-201 | MacOS: window buttons hides header | bug | gui |
| TB-181 | Persist draft task/prevent close unsaved form | bug | gui |
| TB-171 | TB-93/REVIEW: re-run Codex cross-cutting architectural review (previous run stalled) | spike | gui |
| TB-170 | TB-93/GUI: resolveArtifactPaths hot path - 8 stats per agent log line, cache layout | improvement | gui |
| TB-169 | TB-93/GUI: attachment size display polish - IEC unit labels and exact-byte tooltip | tech-debt | gui |
| TB-168 | TB-93/GUI: test infra cleanup - hardcoded sleeps, /tmp/tb fallback, idDirRe negative case | tech-debt | gui |
| TB-167 | TB-93/CLI: minor polish - attach help-text grouping, --rm=false ambiguity, doc step ordering | tech-debt | cli |
| TB-166 | TB-93/GUI: folder_tasks_test.go uses temp/staging names that don't match the CLI's real pattern | tech-debt | gui |
| TB-165 | TB-93/GUI: empty-state hint should say 'drag onto this drawer' not 'onto the task' | improvement | gui |
| TB-164 | TB-93/GUI: surface drag-and-drop in-flight state via attach:dropping/attach:dropped events | improvement | gui |
| TB-163 | TB-93/GUI: add error-path tests for removeAttachments and openAttachment in api.test.ts | tech-debt | gui |
| TB-162 | TB-93/GUI: api.ts listAttachments re-mapping strips the Attachment binding type | improvement | gui |
| TB-161 | TB-93/GUI: OpenAttachment surfaces opaque error when attachments/ dir is missing | improvement | gui |
| TB-160 | TB-93/GUI: TestRecoverStale_DurableCancelledTaskIgnored tests the wrong code path | bug | gui |
| TB-159 | TB-93/GUI: resolveArtifactPaths should normalize taskID to uppercase | bug | gui |
| TB-158 | TB-93/GUI: insert '--' before user paths in tb attach mutations to prevent flag confusion | bug | gui |
| TB-157 | TB-93/CLI: log warnings to stderr from best-effort rollback removal failures | improvement | cli |
| TB-156 | TB-93/CLI: tb attach add path should validate destination filename (parity with --rm) | improvement | cli |
| TB-155 | TB-93/GUI: attachmentsLoading flicker on rapid task switch and concurrent refresh race | bug | gui |
| TB-154 | TB-93/GUI: attachment list accessibility improvements (aria-label, keyboard nav) | improvement | gui |
| TB-153 | TB-93/GUI: attachment remove is destructive single-click without confirmation | improvement | gui |
| TB-152 | TB-93/GUI: TaskDrawer attachments UI has no component-level tests | tech-debt | gui |
| TB-151 | TB-93/GUI: watcher attach() lacks mutex for concurrent Switch invocations | bug | gui |
| TB-150 | TB-93/GUI: watcher race during file->folder promotion misses first TASK.md edit | bug | gui |
| TB-149 | TB-93/GUI: Windows cmd.exe metacharacter injection in OpenAttachment | bug | gui |
| TB-148 | TB-93/CLI: confirm TB-96 hard-error reverted to warn+self-heal is intentional | tech-debt | cli |
| TB-147 | TB-93/CLI: implement startup recovery sweep for stale .promote/.attach staging dirs OR amend doc | tech-debt | cli |
| TB-146 | TB-93/CLI: attach promotion orphans file-form agent state + logs | bug | cli |
