# TB-341: Check full card transictions protocol

**Type:** bug
**Priority:** P2
**Size:** M
**Branch:** —

## Goal

We need to check and enforce some transactions and flows between columns made deterministic by deamon instead of relying on agents, which are not deterministic. Especially read -> in progress -> code review flows. Sometimes tasks stuck in in-progress/code-review with success agents run, but is should not be the valid state: each state should have own rules and allowed transitions. At least there should not be implemented tasks in in progress and reviewd tasks in code review, they should be transitioned eaither to ready or done (or code review in case of in progress, even if column is full). 


## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-21: Created
