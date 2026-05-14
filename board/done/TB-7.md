# TB-7: M7: Polish — settings, shortcuts, tray, menus

**Type:** feature
**Priority:** P2
**Size:** L
**Module:** gui
**Tags:** milestone-m7,gui,polish,epic
**Branch:** —

## Goal

Optional polish pass: settings UI (agent timeout, max workers, default agent, **CLI binary path** — per `docs/FEATURES.md` F7.1), native application menu with **Open Recent ›** submenu, system tray, keyboard shortcuts (N create, / search, Esc close drawer, Enter open selected card).

## Context

Optional polish pass after M1–M6 ship. Pick up only items that meaningfully improve daily use. Scope and order can shift based on user feedback. See `docs/FEATURES.md` § M7.

**Foundations already in place:**
- `Preferences` struct + `preferences.json` persistence + `MaxWorkers` round-trip + clamping ship in TB-56 (M5). M7 extends the struct with `agent_timeout_minutes`, `default_agent`, `cli_path` rather than re-laying the storage layer.
- Recent boards already persist in `recent.json` and are listed by `SettingsService.ListRecentBoards()` (TB-2 / M2). The existing surface is the empty-state list inside the window — M7's deliverable is a **native application-menu** "Open Recent ›" submenu, distinct from that.
- `cli_path` exists as a `SettingsOptions` field but is not persisted today; M7 wires it through `preferences.json` and reapplies it on board open.

**Default-agent semantics:** the persisted value is the **default selection in the Agent dropdown** when a task has no `**Agent:**` set. The GUI does not auto-assign on `tb create` — assignment remains a deliberate user gesture.

**Spec gap to close:** `docs/FEATURES.md` § M7 lists F7.1/F7.2/F7.3 only; the application menu (TB-82) has no anchor. TB-85 (docs flip) is responsible for introducing **F7.4 — Native application menu** before the milestone is sealed.

**Out-of-scope for M7 polish (deferred follow-ups):**
- Arrow-key column traversal between cards (TB-84 ships DOM-focus + Enter only). Open as a backlog task if/when needed.

**Risks:**
- Wails3 alpha menu/tray API surface is unstable (API present at `pkg/application/menu.go` + `pkg/application/system_tray_manager.go` but behaviour on macOS not yet exercised). The menu and tray children should each begin with a small probe (build a 1-item menu / tray icon, confirm it shows on macOS) before sinking time into the full design.
- Keyboard-shortcut conflicts with macOS native input (Cmd+N is reserved by some IMEs; bare `N` must be suppressed inside contenteditable / textarea / CodeMirror).
- TB-79 has a footgun: the existing Agent dropdown auto-saves on change. Pre-selecting a default must not trigger an assignment write — see TB-79's implementation note.

## Decomposition

See children under `tb epic TB-7`. Decomposition spans:
1. Backend: extend `Preferences` (adds new fields + clamps + tests).
2. Backend wiring: agent timeout, CLI path, default-agent dropdown.
3. Frontend service layer: `api.ts` settings wrappers + `preferencesStore.ts`.
4. UI: settings panel; Wails3 application menu (File/View/Help + Open Recent ›); system tray; keyboard shortcuts.
5. Docs flip: `IMPLEMENTATION.md` M7 markers + `FEATURES.md` F7.1/F7.2/F7.3 + new **F7.4** (application menu) + `ARCHITECTURE.md` if the menu/tray patterns warrant invariants.

## Subtasks

- **TB-76** (S) — Preferences struct: add agent_timeout_minutes, default_agent, cli_path with clamps + tests
- **TB-77** (S) — Wire agent_timeout_minutes into agent_run.go (replace agentTimeoutDefault const)
- **TB-78** (S) — Wire cli_path preference into cli.NewClient at board open + reload on change
- **TB-79** (S) — Wire default_agent into AssignAgent dropdown default for unassigned tasks
- **TB-80** (S) — Frontend api.ts settings wrappers + preferencesStore.ts
- **TB-81** (M) — SettingsPanel.svelte: form for timeout/max_workers/default_agent/cli_path with Save + toast
- **TB-82** (M) — Wails3 application menu: File (Open board…, Open Recent ›, Quit), View, Help
- **TB-83** (M) — System tray: idle/running glyph + click to show/hide window
- **TB-84** (S) — Keyboard shortcuts: N (new), / (search), Esc (close drawer), Enter (open card)
- **TB-85** (S) — Docs flip: IMPLEMENTATION.md M7 + FEATURES.md F7.1/F7.2/F7.3 markers + ARCHITECTURE.md if needed

## Acceptance Criteria

- [x] Settings UI for agent timeout, max workers, default agent, CLI binary path
- [x] Native application menu: File → Open board… / Open Recent › / Quit; View; Help
- [x] System tray icon with idle/running glyph + show/hide window
- [x] Keyboard shortcuts: `N` (new task), `/` (focus search), `Esc` (close drawer), `Enter` (open selected card)
- [x] All M7 markers in `docs/IMPLEMENTATION.md` and `docs/FEATURES.md` (F7.1/F7.2/F7.3) flipped to ☑

## Related Tasks

- **TB-2** — Board picker / context (prerequisite — recent.json + ListRecentBoards already in place)
- **TB-3..TB-6** — Prerequisites (M3–M6 milestones)
- **TB-56** — `max_workers` settings foundation (M5; M7 extends `Preferences` rather than re-laying storage)

## Log

- 2026-05-13: Created
- 2026-05-14: Groomed — clarified CLI-path inclusion, distinguished native menu from empty-state recents, called out Wails3 alpha probe risk, locked default-agent semantics; decomposed into TB-76..TB-85
- 2026-05-14: Codex review pass — verified Wails3 alpha menu/tray APIs exist; corrected TB-79 (Agent dropdown auto-saves on change, no Assign button), TB-84 (selection.ts only tracks open-drawer id, not card focus), TB-77 (AgentService is built before SettingsService — needs late-bound TimeoutProvider), TB-78 (live reload via BoardService.setClient, not OpenBoard), TB-80 (use writable/derived per runs.ts, not Svelte 5 runes), TB-76 (normalize wording for missing/zero), TB-82+TB-85 (introduce F7.4 spec anchor for the application menu)
- 2026-05-14: Started — moved to in-progress
- 2026-05-14: Done
- 2026-05-14: Done — shipped M7 preferences/settings UI, per-run timeout, live CLI path reload, visual default-agent fallback, keyboard shortcuts, native menu, tray state controller, docs flip, and focused Go/Vitest/Svelte verification.
