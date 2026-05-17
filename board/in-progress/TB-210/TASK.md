# TB-210: Manual QA: MVP live-board pass

**Type:** spike
**Priority:** P1
**Size:** L
**Module:** gui
**Tags:** manual-qa,mvp
**Branch:** main

## Goal

Run live-board manual QA across finished MVP epics M1-M7 plus TB-93 folder tasks and attachments.

## Acceptance Criteria

- [x] Baseline build, board state, and real agent binary availability recorded
- [x] M1 CLI and board-contract QA marked Pass, Fail with ticket, or Blocked
- [x] M2 read-only GUI QA marked Pass, Fail with ticket, or Blocked
- [x] M3 GUI mutation QA marked Pass, Fail with ticket, or Blocked
- [x] M4 manual agent run QA marked Pass, Fail with ticket, or Blocked
- [x] M5 daemon/recovery QA marked Pass, Fail with ticket, or Blocked
- [x] M6 groom-flow QA marked Pass, Fail with ticket, or Blocked
- [x] M7 polish/settings/shortcut QA marked Pass, Fail with ticket, or Blocked
- [x] TB-93 folder-task and attachment QA marked Pass, Fail with ticket, or Blocked
- [x] Confirmed findings filed as standalone backlog tasks
- [x] Final QA summary includes probes, findings, blocked coverage, and verification commands

## QA Summary

Tested 2026-05-17 11:51 +04. App launch path was `cd gui && task dev`; after the M5 kill/recovery probe the dev app was relaunched with `open /Users/ralist/projects/task-board-tools/gui/bin/tb-gui.dev.app`.

Real agent binaries: `codex` available at `/opt/homebrew/bin/codex` (`codex-cli 0.130.0`) and used for M4/M5/M6 probes; `claude` available at `/Users/ralist/.local/bin/claude` (`2.1.143`) but not used.

QA probe tasks: TB-211 CLI happy path, TB-212 folder attachments, TB-213 manual agent run, TB-214 daemon pickup, TB-215 groom placeholder, TB-216 legacy file parity, TB-218 GUI create dialog.

Finding tickets filed: TB-217 for dash-leading attachment removal parsing and TB-219 for active agent runs remaining visibly queued with no cancel action.

Matrix results:
- M1 CLI + board contract: Pass. JSON read paths, metadata edits, archive/all/active filtering, and invalid ID/status/agent-status errors were exercised.
- M2 read-only GUI: Pass with partial error-path coverage. Live-board counts and TB-210 drawer fields matched CLI state; invalid board-path switching was not fully re-run in this pass.
- M3 GUI mutations: Pass with blocked DnD automation. GUI create/edit/body/archive/filter and watcher refresh paths worked; Computer Use drag did not trigger the app's HTML5 drag-and-drop.
- M4 manual agent runs: Fail, TB-219. A real Codex run produced JSONL/log files, but the drawer stayed queued and exposed no cancel action.
- M5 daemon/recovery: Pass with partial worker breadth. CLI queueing was picked up by the daemon and stale restart recovery wrote a synthetic failed finished event; multi-task worker breadth was not expanded after TB-219.
- M6 groom flow: Fail, TB-219. Needs-grooming state was visible and Groom queued a real Codex groom run, but the drawer had the same stale queued/no-cancel behavior and the run ended failed.
- M7 polish: Pass with blocked tray coverage. Settings persistence and invalid CLI path handling worked; `/`, `N`, and `Esc` shortcuts worked; File menu exposed Open board, Open Recent, Settings, and Quit; tray show/hide was not reachable through Computer Use.
- TB-93 folder tasks + attachments: Fail, TB-217. Folder-form creation, attach, duplicate/missing errors, ordinary removal, folder moves with attachments/state/logs, GUI attachment listing, and mixed folder/file read parity worked; removing a dash-leading filename failed.

Verification commands:
- `cd cli && go build -o tb .`
- `cd cli && go test ./...`
- `cd gui && go test ./...`
- `cd gui/frontend && npm run check`
- `cd gui/frontend && npm test -- --run`
- `./cli/tb regenerate`

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Started live-board MVP QA pass from manual QA plan.
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Recorded manual QA results, finding tickets, blocked coverage, and verification commands.
- 2026-05-17: Retracted TB-220 after user clarified the prompt diff was a parallel manual edit, not QA fallout.
