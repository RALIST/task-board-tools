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

- [x] `cli/main.go` usage block now lists `tb attach` and `tb attach --rm` adjacent; `tb assign` moved below them.
- [x] `containsAttachRemoveFlag` no longer matches the `--rm=` prefix; only `-rm`/`--rm` enable the remove path, leaving any future value-bearing form to the FlagSet.
- [x] `docs/ARCHITECTURE.md` "File → folder promotion" step ordering rewritten to reflect actual code: the promotion-log line is added to the staged `TASK.md` buffer BEFORE the publish-rename, so there is no post-publish second TASK.md write.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

