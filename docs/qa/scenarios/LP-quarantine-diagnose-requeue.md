---
id: LP-quarantine-diagnose-requeue
area: LP
title: Diagnose, repair, and requeue a quarantined Loop node
persona: Bruno
journey: J-recover-loop-node-failure
expected: A repeated failing node parks with sanitized bounded repair context while independent work continues, required consumers name the quarantined dependency, and requeue records provenance before normal bounded succession completes the run.
entry_points: `compozy loop nodes --state quarantined -o json`; `compozy loop node requeue --node <node-id> --run-id <run-id> -o json`; Loop run events over HTTP/SSE
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: looprun-9552c0109f7a382c; /Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa/evidence/task13/16-quarantine-requeue-done.png
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: LP-sick-target-degrades-one-lane; TA-loop-failure-breaker
---

acceptance-walk: Drive one node into quarantine while an independent lane continues, inspect its bounded sanitized repair context and the blocked consumer, repair the target, and requeue as an identified actor. Confirm provenance precedes one successor generation and refreshed Web, CLI, HTTP, and SSE views agree through terminal completion.
