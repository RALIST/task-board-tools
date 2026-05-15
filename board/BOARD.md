# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-93 | Move from file-based to folder-based approach | 24/42 | backlog | cli |
| TB-177 | Auto task implementation | 0/3 | backlog | gui |
| TB-109 | Worktree-isolated task execution | 0/12 | backlog | cli |
| TB-130 | Agent session resume + interrupted-run recovery | 0/12 | backlog | gui |
| TB-172 | Auto-groom | 0/3 | backlog | gui |
| TB-182 | Add special labes\tags\status for user attention | 0/3 | backlog | agent |
| TB-194 | Code-review column | 0/6 | backlog | workflow |
| TB-186 | Change parent task | 0/3 | backlog | gui |
| TB-204 | Show epic progress | 0/0 | backlog | gui/frontend |

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
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
| TB-93 | Move from file-based to folder-based approach | feature | P0 | XL | cli |
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
| TB-131 | Closed-set schema sweep: interrupted status + resume mode | feature | P1 | S | gui |
| TB-132 | JSONL schema: SessionID/ResumedFrom/Cwd/RunEnv + ResumeCandidate helper | feature | P1 | M | gui |
| TB-133 | Shared post-started session-write hook in runGoroutine | feature | P1 | S | gui |
| TB-134 | Codex --json switch + codexJsonTranslator + parity tests | feature | P1 | M | gui |
| TB-135 | Claude session capture via --session-id pre-allocation | feature | P1 | S | gui |
| TB-136 | Codex session capture via --json OnSessionID callback | feature | P1 | M | gui |
| TB-137 | Recovery: dead-PID + SessionID -> interrupted (markInterrupted) | feature | P1 | M | gui |
| TB-138 | Resume backend Claude: ResumeDecorator + -r flag + cwd/env replay | feature | P1 | M | gui |
| TB-139 | Resume backend Codex: codex exec --json resume <uuid> wiring | feature | P1 | M | gui |
| TB-140 | Frontend: Resume button, interrupted pill, resumed_from chip | feature | P1 | M | gui |
| TB-141 | Fake-runner integration test: kill->interrupted->resume cycle | feature | P1 | M | gui |
| TB-142 | Docs sweep: ARCHITECTURE.md + CLAUDE.md + FEATURES.md for resume | improvement | P1 | S | docs |
| TB-143 | Add semver to cli tool | feature | P2 | M | cli |
| TB-144 | Append logs realtime when task is opened | bug | P1 | S | gui |
| TB-152 | TB-93/GUI: TaskDrawer attachments UI has no component-level tests | tech-debt | P1 | M | gui |
| TB-153 | TB-93/GUI: attachment remove is destructive single-click without confirmation | improvement | P1 | S | gui |
| TB-154 | TB-93/GUI: attachment list accessibility improvements (aria-label, keyboard nav) | improvement | P1 | S | gui |
| TB-156 | TB-93/CLI: tb attach add path should validate destination filename (parity with --rm) | improvement | P2 | S | cli |
| TB-157 | TB-93/CLI: log warnings to stderr from best-effort rollback removal failures | improvement | P2 | S | cli |
| TB-159 | TB-93/GUI: resolveArtifactPaths should normalize taskID to uppercase | bug | P2 | S | gui |
| TB-160 | TB-93/GUI: TestRecoverStale_DurableCancelledTaskIgnored tests the wrong code path | bug | P2 | S | gui |
| TB-161 | TB-93/GUI: OpenAttachment surfaces opaque error when attachments/ dir is missing | improvement | P2 | S | gui |
| TB-162 | TB-93/GUI: api.ts listAttachments re-mapping strips the Attachment binding type | improvement | P2 | S | gui |
| TB-163 | TB-93/GUI: add error-path tests for removeAttachments and openAttachment in api.test.ts | tech-debt | P2 | S | gui |
| TB-164 | TB-93/GUI: surface drag-and-drop in-flight state via attach:dropping/attach:dropped events | improvement | P2 | S | gui |
| TB-165 | TB-93/GUI: empty-state hint should say 'drag onto this drawer' not 'onto the task' | improvement | P2 | S | gui |
| TB-166 | TB-93/GUI: folder_tasks_test.go uses temp/staging names that don't match the CLI's real pattern | tech-debt | P2 | S | gui |
| TB-167 | TB-93/CLI: minor polish - attach help-text grouping, --rm=false ambiguity, doc step ordering | tech-debt | P2 | S | cli |
| TB-168 | TB-93/GUI: test infra cleanup - hardcoded sleeps, /tmp/tb fallback, idDirRe negative case | tech-debt | P2 | S | gui |
| TB-169 | TB-93/GUI: attachment size display polish - IEC unit labels and exact-byte tooltip | tech-debt | P2 | S | gui |
| TB-170 | TB-93/GUI: resolveArtifactPaths hot path - 8 stats per agent log line, cache layout | improvement | P2 | S | gui |
| TB-171 | TB-93/REVIEW: re-run Codex cross-cutting architectural review (previous run stalled) | spike | P2 | S | gui |
| TB-172 | Auto-groom | feature | P1 | L | gui |
| TB-173 | GUI: persist auto-groom setting and toggle | feature | P1 | M | gui |
| TB-174 | GUI: auto-groom triage tasks via groom-mode daemon runs | feature | P1 | M | gui |
| TB-175 | GUI: surface auto-groom feedback and manual fallback | feature | P1 | S | gui |
| TB-176 | Track PID of launched agents | bug | P2 | M | gui |
| TB-177 | Auto task implementation | feature | P0 | L | gui |
| TB-178 | GUI: persist auto-implement settings and query | feature | P0 | M | gui |
| TB-179 | GUI: enqueue auto-implement candidates from daemon | feature | P0 | M | gui |
| TB-180 | GUI: show auto-implement controls and feedback | feature | P0 | S | gui |
| TB-181 | Persist draft task/prevent close unsaved form | bug | P1 | S | gui |
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
| TB-201 | MacOS: window buttons hides header | bug | P2 | M | gui |
| TB-202 | Create proper name and icon for app | improvement | P2 | S | gui |
| TB-203 | obfuscation agents logs and tasks | bug | P1 | M | gui |
| TB-204 | Show epic progress | improvement | P2 | M | gui/frontend |
| TB-205 | Setup esling and deadcode check for frontend | tech-debt | P2 | M | tooling |
| TB-206 | Setup golangci-lint for project and initial run it | tech-debt | P2 | M | tooling |
| TB-207 | Allow tasl title edit from GUI | improvement | P2 | M | gui |
| TB-208 | Switch projects bug | bug | P1 | M | gui |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-158 | TB-93/GUI: insert '--' before user paths in tb attach mutations to prevent flag confusion | bug | gui |
| TB-155 | TB-93/GUI: attachmentsLoading flicker on rapid task switch and concurrent refresh race | bug | gui |
| TB-151 | TB-93/GUI: watcher attach() lacks mutex for concurrent Switch invocations | bug | gui |
| TB-150 | TB-93/GUI: watcher race during file->folder promotion misses first TASK.md edit | bug | gui |
| TB-149 | TB-93/GUI: Windows cmd.exe metacharacter injection in OpenAttachment | bug | gui |
| TB-148 | TB-93/CLI: confirm TB-96 hard-error reverted to warn+self-heal is intentional | tech-debt | cli |
| TB-147 | TB-93/CLI: implement startup recovery sweep for stale .promote/.attach staging dirs OR amend doc | tech-debt | cli |
| TB-146 | TB-93/CLI: attach promotion orphans file-form agent state + logs | bug | cli |
| TB-145 | Board switch error | bug | gui |
| TB-129 | Remove ”non-editable” section when edit task’s body | bug | gui |
| TB-127 | Open ticket full screen | improvement | gui |
| TB-126 | GUI: dropping a file on a task card attaches it | improvement | gui |
| TB-125 | GUI: whole TaskDrawer accepts attachment file drops | improvement | gui |
| TB-124 | Test attachments | bug |  |
| TB-123 | Test claude logs | bug |  |
| TB-108 | GUI attachment smoke blocked by missing attach surfaces | bug | gui |
| TB-107 | Show global agent quota in app header | feature | gui |
| TB-106 | Mixed-board final smoke test for TB-93 | spike | cli |
| TB-105 | GUI: watcher emits one logical refresh per attachment op and folder move | feature | gui |
| TB-104 | GUI: drag-and-drop attachments onto task card and drawer | feature | gui |
| TB-103 | GUI: TaskDrawer attachments list, add via picker, remove via tb | feature | gui |
| TB-102 | Agent: task-local logs/state for folder tasks + stale recovery | feature | gui |
| TB-101 | CLI: BOARD.md byte-identical regardless of storage form | feature | cli |
| TB-100 | CLI: remove attachments via `tb attach --rm` safely | feature | cli |
| TB-99 | CLI: tb attach <ID> <path>... with auto-promotion from file to folder | feature | cli |
| TB-98 | CLI: move folder tasks as whole directories across statuses and archive | feature | cli |
| TB-97 | CLI: tb create defaults to folder form | feature | cli |
| TB-96 | CLI: read folder-form tasks identically to file-form | feature | cli |
| TB-95 | Publish TB-93 folder-task milestone in docs | improvement | docs |
| TB-94 | Spec folder-task contract in docs/ARCHITECTURE.md | feature | docs |
| TB-92 | Limit showed tags in header to 10 | bug |  |
| TB-91 | Task card is bigger then column | improvement |  |
| TB-90 | Board switching is not working | bug |  |
| TB-89 | Click outside should close dropdown | bug | gui |
| TB-88 | GUI triage fails when active tb binary lacks JSON support | bug | gui |
| TB-87 | Parent epic filter hides the epic card itself | bug | gui |
| TB-86 | Adapt post-edit hook for Codex PostToolUse | improvement | tooling |
| TB-85 | Docs flip: IMPLEMENTATION.md M7 + FEATURES.md F7.1/F7.2/F7.3 markers + ARCHITECTURE.md if needed | tech-debt | gui |
| TB-84 | Keyboard shortcuts: N (new), / (search), Esc (close drawer), Enter (open card) | feature | gui |
| TB-83 | System tray: idle/running glyph + click to show/hide window | feature | gui |
| TB-82 | Wails3 application menu: File (Open board…, Open Recent ›, Quit), View, Help | feature | gui |
| TB-81 | SettingsPanel.svelte: form for timeout/max_workers/default_agent/cli_path with Save + toast | feature | gui |
| TB-80 | Frontend api.ts settings wrappers + preferencesStore.ts | feature | gui |
| TB-79 | Wire default_agent into AssignAgent dropdown default for unassigned tasks | feature | gui |
| TB-78 | Wire cli_path preference into cli.NewClient at board open + reload on change | feature | gui |
| TB-77 | Wire agent_timeout_minutes into agent_run.go (replace agentTimeoutDefault const) | feature | gui |
| TB-76 | Preferences struct: add agent_timeout_minutes, default_agent, cli_path with clamps + tests | feature | gui |
| TB-75 | CLI: tb edit --goal / --acceptance — body-section writes via writeFileAtomic | feature | cli |
| TB-74 | Docs: flip M6 markers — IMPLEMENTATION.md / FEATURES.md F6.1+F6.2 / ARCHITECTURE.md GroomingDecorator | improvement | docs |
| TB-73 | Card: 'needs grooming' indicator + click-to-suggest-Groom in drawer | feature | gui |
