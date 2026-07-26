---
id: NB-bridge-overload-recovery
area: NB
title: Recover a first-party bridge delivery from provider overload
persona: Omar
journey: J-connect-bridge-provider
expected: "A first-party outbound bridge call receiving HTTP 529 classifies the failure as `overloaded`, waits once through the distinct bounded overload profile, and succeeds on the next provider response. HTTP 500 is `server_error`, a connection reset remains `transient`, and a positive `Retry-After` is preserved exactly. A committed mutation is never replayed, delegated ACP agents are unaffected, and no provider-local retry loop exists."
entry_points: agh bridge send-test; HTTP and UDS bridge send-test; fake-provider outbound transport
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-indeterminate-bridge-delivery; NB-long-bridge-replies; NB-bridge-provider-setup
---

An operator gets bounded automatic recovery when a first-party bridge provider explicitly reports
overload, without risking replay after a remote mutation has committed.

Phase D impact flag 2026-07-19: W3 consolidates first-party outbound retry execution in the shared
runner and adds distinct overload/server-error taxonomy. Planning flag only; no QA retest ran.

Phase C planning 2026-07-19: persona normalized to Omar (fleet operator lane); companion to the W3
retry-consolidation gate (ADR-010 §6, two-touch determination recorded in
`.codex/plans/20260719T110539Z-shared-retry-consolidation.md`). Forensic contract (SD-006):
timestamped fake-provider request log showing 529 → `overloaded` single bounded retry → success,
500 → `server_error`, reset → `transient`, preserved `Retry-After`, and the delegated-ACP zero-diff
check.
