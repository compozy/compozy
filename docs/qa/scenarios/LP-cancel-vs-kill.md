---
id: LP-cancel-vs-kill
area: LP
title: Distinguish cooperative Loop cancellation from immediate kill
persona: Bruno
journey: J-recover-loop-node-failure
expected: Cancel is durably visible through requested, delivering, and draining before canceled(operator_cancel); kill immediately stops the bound session and ends canceled(operator_kill); repeated verbs are idempotent, races produce one terminal effect, and the removed stop verb is unavailable.
entry_points: `compozy loop cancel|kill`; `compozy loop node cancel|kill`; HTTP/UDS routes; native tools; Web run controls
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: looprun-db9; looprun-1e5; looprun-08b; looprun-d7f; /Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-03-loop-node-lifecycle.md
overlaps: LP-016
---

acceptance-walk: Cancel one active run and kill another through Web, then repeat and race the run and node verbs through structured CLI and HTTP. Confirm the cooperative drain versus immediate session stop, exact terminal causes, one terminal effect, deterministic loser responses, fresh-read parity, and absence of every retired stop control or route.
