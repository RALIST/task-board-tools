# TB-169: TB-93/GUI: attachment size display polish - IEC unit labels and exact-byte tooltip

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,frontend,polish
**Branch:** —
**Parent:** TB-93

## Goal

Bundled GUI frontend display polish findings:

1. TaskDrawer.svelte:275-280 - formatSize uses 1024-based divisors but labels them KB/MB/GB. Use KiB/MiB/GiB per IEC or use 1000-based denominators for accuracy. (Finding #15)

2. TaskDrawer.svelte:735 - no tooltip showing the precise byte count. Users sometimes need exact bytes. Add title={`${a.size} bytes`} on .att-size. (Finding #16)

3. api.test.ts:153-172 - multi-select picker test asserts the entire options object including English strings, brittle to UX copy changes. Use expect.objectContaining({CanChooseFiles: true, AllowsMultipleSelection: true}). (Finding #17)

Source: GUI frontend review findings #15, #16, #17.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
