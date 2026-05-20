# TB-311: Manual smoke board-switch cancellation in desktop GUI

**Type:** spike
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** manual-qa,gui,board-switch,agent
**Branch:** —

## Goal

Run the desktop GUI against two real boards and verify the TB-302 board-switch cancellation path end to end: start a slow auto-groom or auto-implement run on Board A, switch to Board B, confirm Board B stays active and unaffected, then switch back to Board A and confirm the old run is coherently cancelled with reason board switch.

## Acceptance Criteria

- [ ] Start the Wails GUI with two valid boards available.
- [ ] Start a deliberately slow auto-groom or auto-implement run on Board A.
- [ ] Switch to Board B while Board A's run is active; confirm Board B remains active and no old-board terminal write, promotion, or review follow-up lands on Board B.
- [ ] Switch back to Board A; confirm the old task shows exactly one coherent cancelled run with reason `board switch` and no duplicate running row.
- [ ] Record the GUI version/command, board paths, and observed result in this task log or notes.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited acceptance
