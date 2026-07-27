---
id: RT-compozy-home-isolation
area: RT
title: Isolate runtime state with COMPOZY_HOME
persona: Bruno
journey: J-validate-compozy-hard-cut
expected: Two temporary COMPOZY_HOME values create independent compozy.db files, daemon metadata, logs, and sockets; each compozy status -o json reports only its selected home, and an AGH_HOME-only process never redirects or merges either runtime.
entry_points: COMPOZY_HOME=<tempdir> compozy daemon start; compozy status -o json; compozy doctor -o json; AGH_HOME-only negative control
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-compozy-home-layout;RT-refuse-legacy-database
---

QA impact 2026-07-26: the environment namespace hard cut makes `COMPOZY_HOME`
the sole runtime-home override. Planning flag only; the next QA cycle owns the
two-home isolation pass and the legacy-variable negative control.
