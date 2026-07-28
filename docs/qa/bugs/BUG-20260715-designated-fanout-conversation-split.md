# BUG-20260715-designated-fanout-conversation-split: Sibling runs resolve separate conversations

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-enable-coordinated-conversations, compare and watch the future coordinated run
- **Scenarios:** NB-run-conversation-bounds-usage
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Workspace-coordination participation derived a run-strategy channel from each sibling run ID. The coordinator and workers in one designated fan-out therefore received different conversations even though the contract defines one shared conversation per designation group.

## Reproduction

- **Charter:** CH-coordination-future-runs · **Tour:** Back-Button Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Enable future coordination for one selected scope.
2. Fan out a task into one coordinator and multiple workers under one designation group.
3. Read each run's immutable participation snapshot.

**Expected:** Every sibling uses one group-derived channel; a separate group uses a different channel.
**Actual:** Each sibling channel was derived from its individual run ID and the group could not share one run conversation.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md`
- Live retest: `run-ffcfef7a87b45e16`, `run-7f66c16046c25861`, and `run-b727b6afbc9c179c` all project `coord-tdg-701c2ad27c0c4afc-0a305f94c62e`.

## Fix

- **Root cause:** `resolveQueuedRunParticipation` always passed the individual run ID to the run-channel resolver and ignored the existing `designation_group_id`.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the canonical Task participation integration suite requires three siblings to share one non-empty channel and an independent designation group to resolve a distinct channel.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** the conversation key is designation-group scoped while ownership, wake budgets, Task state, and usage remain individual-run scoped.
