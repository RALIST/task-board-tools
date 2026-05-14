# TB-155: TB-93/GUI: attachmentsLoading flicker on rapid task switch and concurrent refresh race

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,frontend
**Branch:** —
**Parent:** TB-93

## Goal

TaskDrawer.svelte:113-114, 131-135 - (1) On rapid task switch the previous task's attachment rows remain visible with no loading indicator: refreshAttachments early-returns on (cancelled || taskId !== id) without resetting attachmentsLoading, the gate at line 724 ('attachmentsLoading && attachments.length === 0') hides the spinner when stale rows are still present. Fix: in the , set attachmentsLoading = true *after* clearing attachments = []. (2) Concurrent races: if a board:reloaded event fires for the same id while a previous load is in flight, the second call sets attachmentsLoading=true again and the first promise resolves with stale data (taskId === id still holds) and overwrites possibly-older data. Fix: track a monotonic request token (let reqSeq = (0); const my = ++reqSeq; ... if (my !== reqSeq) return;) inside refreshAttachments. Source: GUI frontend review findings #1 + #7.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
