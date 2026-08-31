---
id: TA-task-run-result-paging
area: TA
title: Page one task-run result exactly across public surfaces
persona: Ada
journey: J-complete-partial-loop
expected: CLI, HTTP, UDS, Host API, and the task native tool return the same ordered base64 bytes and descriptor for inline and external results, preserve multibyte boundaries after restart, mask foreign workspaces as not found, reject invalid ranges, and report corrupt content without exposing it.
entry_points: compozy task run result; GET /api/task-runs/:id/result; tasks/runs/result; compozy__task_run_result
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/result-page-00000.json; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/http-result-page.json; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/native-task-run-result-page.json; /Users/pedronauck/dev/qa-labs/compozy-consumer-saas-growth-20260831-181713-956331-lab/qa-artifacts/qa/result-post-restart.sha256
last_report: docs/qa/reports/2026-08-31-loop-result-fix.md
overlaps: LP-oversized-action-result-fails; ET-tool-result-artifact-recovery
---

Create one Loop action result that crosses both the 16 KiB inline boundary and 64 KiB. Read it in ordered pages through every structured surface, concatenate decoded bytes once, and compare the exact result. Restart the daemon and repeat the read. Confirm a different workspace receives the same not-found shape as a missing result, an invalid range is rejected, and a corrupt reference is reported as unreadable rather than returned.

QA impact 2026-08-31: new public task-run result resource and agent-manageable readers.

QA 2026-08-31: CLI/UDS, HTTP, and `compozy__task_run_result` returned the same descriptor and ordered 16 KiB byte pages. Five decoded pages reconstructed 71,694 bytes with SHA-256 `97eca41229b21547388b09aa0f26e9a46f7fee61bc2a85daa8cc4169a670da3f` before and after restart. An inline 1,944-byte run returned one JSON page with `eof=true`; invalid range returned 400 and a missing run returned the masked 404 shape. Canonical store/service tests own corruption, multibyte-boundary, and foreign-workspace probes.
