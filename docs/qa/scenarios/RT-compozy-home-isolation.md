---
id: RT-compozy-home-isolation
area: RT
title: Isolate runtime state with COMPOZY_HOME
persona: Bruno
journey: J-validate-compozy-hard-cut
expected: Two temporary COMPOZY_HOME values create independent compozy.db files, daemon metadata, logs, and sockets; each compozy status -o json reports only its selected home, and a process with only a retired home variable never redirects or merges either runtime.
entry_points: COMPOZY_HOME=<tempdir> compozy daemon start; compozy status -o json; compozy doctor -o json; retired-home-variable negative control
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/bootstrap-manifest.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/cross-surface/onboarding-complete.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-runtime.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: RT-compozy-home-layout;RT-refuse-legacy-database
---

QA impact 2026-07-26: the environment namespace hard cut makes `COMPOZY_HOME`
the sole runtime-home override. Planning flag only; the next QA cycle owns the
two-home isolation pass and the legacy-variable negative control.
