# BUG-20260729-agent-knowledge-refresh-missed: Active worker misses a changed workspace knowledge signal

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Priya; Bruno; Mateo Rivera
- **Journey Step:** consumer-saas-growth, silent event-volume disruption recovery
- **Scenarios:** TA-agent-knowledge-refresh-on-wake; RT-073
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-ext-improvs.md
- **Origin:** Task 11 isolated consumer SaaS growth replay

## Summary

The Data Scientist received three runtime turns after the event-volume knowledge file changed from `first_save: 7812` to `first_save: 0`, but none of those turns re-read the updated file or reported the zero-volume anomaly. The existing launch hold happened to remain in force for a different reason, so the missed signal was silent rather than immediately destructive.

## Reproduction

1. Start the `consumer-saas-growth` playbook and allow the Data Scientist session to become active.
2. Replace the workspace knowledge value in `event-volume-yesterday.md` with `first_save: 0`.
3. Observe the Data Scientist through subsequent review wake turns for five minutes.

**Expected:** The active worker refreshes its workspace knowledge, reports the anomaly to `data-watch`, and blocks launch until tracking is confirmed.
**Actual:** Three post-change turns completed without reading or mentioning the changed value.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/probes/silent-event-drop.json`
- Session `sess-5cdcb57b1d058e6f` processed three turns after the trigger.
- Journey-log probe `silent_event_drop-6` records `disruption_seeded` followed by `disruption_missed`.

## Fix

- **Root cause:** Confirmed. Workspace `knowledge/` files were not part of daemon prompt composition,
  and the composite returned early for every synthetic turn. Task and Heartbeat wakes therefore
  carried neither current knowledge bytes nor a revision signal. Prompt-time harness resolution also
  failed to reconstruct the synthetic metadata already present in the ACP turn after that bypass was
  removed.
- **Correction:** The daemon now reopens a bounded, no-follow Markdown snapshot inside the session
  workspace on every eligible user, Network, and synthetic turn. It includes current bytes, relative
  paths, a revision digest, and omission metadata; a file change is visible on the next eligible wake
  without a watcher or operator follow-up prompt. Synthetic ACP metadata is projected back into the
  harness policy at dispatch.
- **Fix commit:** pending final whole-diff commit
- **Regression test:** `TestPromptInputCompositeIntegrationRefreshesWorkspaceKnowledgeOnSyntheticWake`
  mutates workspace knowledge between synthetic turns and proves current-byte delivery plus
  outside-workspace symlink exclusion.

## Verification

- **Result:** FAIL. The launch hold preserved safety by coincidence, but the named disruption signal was not detected.

## Regressed — 2026-08-02

- **Report:** `docs/qa/reports/2026-08-02-devtool-oss-launch.md`
- **Reproduction:** in the isolated `devtool-oss-launch-20260803-004228-669172` lab, the observer corrected the materialized benchmark knowledge file at 2026-08-03T01:39:21Z from its clean baseline to 410 ms versus a 500 ms candidate. The active benchmark worker had read the old value immediately before the write and received no runtime-owned knowledge revision signal.
- **Expected:** the benchmark owner reports the +21.95122% regression to `bench-watch` within five minutes, the performance reviewer classifies it, and the engineering lead holds the release.
- **Actual:** no `bench-watch` thread existed at the 2026-08-03T01:44:21Z deadline. The worker eventually reread the file and posted the correct delta at 2026-08-03T01:56:27Z, 17m06s after the visible write; the reviewer then issued the correct `REGRESSION` / `HOLD` verdict at 01:59:53Z.
- **Evidence:** `/home/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260803-004228-669172-lab/qa-artifacts/qa/disruption-bench-regression-result.json` and the same lab's `journey-log.jsonl`.
- **Result:** Reopened. Safety ultimately converged, but the knowledge-refresh SLA failed and the decision depended on a late reread rather than a runtime-owned revision signal.

## Verified — 2026-08-03

- **Report:** `docs/qa/reports/2026-08-02-knowledge-refresh-on-wake.md`
- **Lab:** `/home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab`
- **Provider:** native Codex, model `gpt-5.6-sol`, session `sess-f75e342442812f4d`
- **Replay:** the agent first reported `CURRENT_CANDIDATE_MS=410`; after the workspace Markdown changed to `500 ms`, one manual Heartbeat wake returned `wake_sent` and the first synthetic response reported `CURRENT_CANDIDATE_MS=500`.
- **Timing:** 19.594 seconds from file change to the complete agent message, inside the unchanged five-minute limit.
- **Independent reads:** `session events` and a fresh `session recap` agreed; post-wake health was idle, healthy, attachable, and wake-eligible.
- **Prompt fidelity:** exactly one user message and one synthetic reentry; no second operator prompt.
- **Evidence:** `/home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab/qa-artifacts/qa/knowledge-refresh-evidence.json`
- **Result:** Verified. The narrow behavior passes; the same lab's generic release-grade auditor remains blocked by wider actor/channel/API/Web/artifact minimums and the intentionally deferred final gate, so this report makes no release-ready claim.
