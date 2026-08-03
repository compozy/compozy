---
id: LP-cancel-vs-kill
area: LP
title: Distinguish cooperative Loop cancellation from immediate kill
persona: Bruno
journey: J-recover-loop-node-failure
expected: Cancel is durably visible through requested, delivering, and draining before canceled(operator_cancel); kill immediately stops the bound session and ends canceled(operator_kill); repeated verbs are idempotent, races produce one terminal effect, and the removed stop verb is unavailable.
entry_points: `compozy loop cancel|kill`; `compozy loop node cancel|kill`; HTTP/UDS routes; native tools; Web run controls
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-016
---

acceptance-walk: Cancel one active run and kill another through Web, then repeat and race the run and node verbs through structured CLI and HTTP. Confirm the cooperative drain versus immediate session stop, exact terminal causes, one terminal effect, deterministic loser responses, fresh-read parity, and absence of every retired stop control or route.
