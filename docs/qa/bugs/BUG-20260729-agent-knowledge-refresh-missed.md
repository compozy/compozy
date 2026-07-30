# BUG-20260729-agent-knowledge-refresh-missed: Active worker misses a changed workspace knowledge signal

- **Status:** verified
- **Impact (user-side):** Silent decision risk
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Priya
- **Journey Step:** consumer-saas-growth, silent event-volume disruption recovery
- **Scenarios:** TA-agent-knowledge-refresh-on-wake
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

- **Root cause:** UNCONFIRMED. The session did not refresh the changed workspace knowledge on later wake turns; the defect may be in knowledge invalidation, wake context assembly, or worker instructions.
- **Correction:** Pending diagnosis. The fix must make changed knowledge observable without requiring an operator follow-up prompt.
- **Fix commit:** pending
- **Regression test:** A canonical runtime scenario must mutate workspace knowledge between turns and prove the next eligible wake observes the new bytes.

## Verification

- **Result:** FAIL. The launch hold preserved safety by coincidence, but the named disruption signal was not detected.
