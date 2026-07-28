---
id: RT-compozy-claim-token-redaction
area: RT
title: Keep Compozy claim tokens inside the lease boundary
persona: Dora
journey: J-validate-compozy-hard-cut
expected: Claim generation produces a case-insensitively protected compozy_claim_ raw secret while claim_token and claim_token_hash field names remain stable; no raw token crosses logs, diagnostics, HTTP, UDS, CLI, native tools, SSE, events, transcripts, Web responses, or persisted diagnostic payloads, and hashes plus correlation ids survive redaction.
entry_points: task claim/heartbeat/complete routes; compozy task next -o json; compozy__task_run_claim_next; daemon logs; SSE; diagnostics; Web task detail
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-runtime.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: RT-secret-redaction-boundary; NB-020; TA-exact-claim-single-owner
---

QA impact 2026-07-26: the raw claim-token identity moved to `compozy_claim_` across
generation and every leak fence. Planning flag only; the next QA cycle must plant both
lowercase and uppercase-shaped Compozy tokens and prove zero raw hits across all listed
surfaces without renaming the stable public fields.
