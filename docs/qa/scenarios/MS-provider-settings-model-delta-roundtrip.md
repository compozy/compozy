---
id: MS-provider-settings-model-delta-roundtrip
area: MS
title: Provider Settings preserves model deltas and validation semantics
persona: Ada
journey: J-20
expected: A Provider Settings PUT with unchanged curated membership persists only explicit five-rate and reasoning deltas, survives restart, leaves unchanged catalog enrichment out of config, and rejects negative or non-finite rates as caller validation without mutation.
entry_points: Provider Settings HTTP+UDS; `compozy config show`; model catalog all-view readback
qa_status: blocked-verify
bug_ids: BUG-20260729-provider-model-pricing-roundtrip;BUG-20260729-provider-model-validation-status
fix_status: pending
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-056
---

Added by the MS-056 QA repair impact flag. The rebuilt candidate passed the originating replay;
execute this tracker row in the next QA cycle after the governed fix commit exists.
