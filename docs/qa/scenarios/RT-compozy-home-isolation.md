---
id: RT-compozy-home-isolation
area: RT
title: Isolate runtime state with COMPOZY_HOME
persona: Bruno
journey: J-validate-compozy-hard-cut
expected: Two temporary COMPOZY_HOME values create independent runtime state, and neither runtime reclassifies the operator-home .compozy config as a workspace overlay or merges it into the selected home.
entry_points: HOME=<operator-home> COMPOZY_HOME=<tempdir> compozy daemon start; compozy status -o json; compozy workspace list -o json; second isolated home; retired-home-variable negative control
qa_status: pass
bug_ids: BUG-20260812-global-workspace-gateway-config
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/bootstrap-manifest.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/cli/first-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/cli/restart-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/cli/second-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/cli/project-overlay-rejection.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-13-compozy-home-isolation-regression.md
overlaps: RT-compozy-home-layout;RT-refuse-legacy-database
---

QA impact 2026-07-26: the environment namespace hard cut makes `COMPOZY_HOME`
the sole runtime-home override. Planning flag only; the next QA cycle owns the
two-home isolation pass and the legacy-variable negative control.

QA impact 2026-08-13: reset after the detached daemon again treated the operator-home
`.compozy/config.toml` as a project workspace overlay when `COMPOZY_HOME` selected a distinct
runtime home. The focused re-walk owns the isolated-home startup, restart, and workspace checks.
