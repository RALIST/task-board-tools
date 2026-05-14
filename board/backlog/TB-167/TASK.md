# TB-167: TB-93/CLI: minor polish - attach help-text grouping, --rm=false ambiguity, doc step ordering

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** cli
**Tags:** epic-tb93,review-tb93,polish
**Branch:** —
**Parent:** TB-93

## Goal

Bundled CLI low-priority findings from the grand review:

1. cli/main.go - the two new help-text lines for tb attach and tb attach --rm sit on either side of tb assign in the usage block. Move them adjacent so the two forms are listed together. (Finding #10)

2. cli/attach.go:46-58 - containsAttachRemoveFlag recognizes --rm= as prefix, so tb attach --rm=false ... still takes the remove path. Fix: drop the --rm= prefix match and let the FlagSet parse, or document accepted forms. (Finding #11)

3. docs/ARCHITECTURE.md File->folder promotion section step ordering says step 8 (log entry) occurs after the legacy file removal (step 7). Implementation writes the log entry into the staged TASK.md *before* publishing the directory rename (cli/attach.go:357-363). Behavior is equivalent but docs and code disagree. (Finding #13)

Source: CLI grand review findings #10, #11, #13.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
