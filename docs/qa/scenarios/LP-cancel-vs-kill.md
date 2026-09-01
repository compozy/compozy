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
evidence: looprun-db9; looprun-1e5; looprun-08b; looprun-d7f; /Users/pedronauck/dev/qa-labs/compozy-loop-node-lifecycle-20260803-191237-281307-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-loop-agent-ownership-r2-20260806-040706-936266-lab/qa-artifacts/qa/evidence/loop-cancel-draining.json;/Users/pedronauck/dev/qa-labs/compozy-loop-agent-ownership-r2-20260806-040706-936266-lab/qa-artifacts/qa/evidence/loop-cancel-latest.json;docs/qa/reports/2026-08-31-loop-result-fix.md
last_report: docs/qa/reports/2026-08-31-loop-result-fix.md
overlaps: LP-016
---

acceptance-walk: Cancel one active run and kill another through Web, then repeat and race the run and node verbs through structured CLI and HTTP. Confirm the cooperative drain versus immediate session stop, exact terminal causes, one terminal effect, deterministic loser responses, fresh-read parity, and absence of every retired stop control or route.

QA impact 2026-08-31: targeted public-daemon E2E replay passed after fixing the cancellation race where a run-owned Goal binding could be created after the cancellation request timestamp. The binding failure and cleanup obligation now share a terminal timestamp that never precedes binding creation; the wider cancel-versus-kill contract remains covered by the prior acceptance walk.
