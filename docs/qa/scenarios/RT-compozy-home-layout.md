---
id: RT-compozy-home-layout
area: RT
title: Create the Compozy home layout on fresh boot
persona: Bruno
journey: J-validate-compozy-hard-cut
expected: A fresh daemon boot creates the canonical .compozy home and workspace overlay, status and doctor report only those paths, and no retired home directory or fallback is created or read.
entry_points: compozy daemon start; compozy status -o json; compozy doctor -o json; fresh isolated home and workspace
qa_status: untested
bug_ids: BUG-20260727-dirty-build-release-track
fix_status: fixed
retest_status: pass
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/bootstrap-manifest.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/daemon-start.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-home.png
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: RT-refuse-legacy-database
---

QA impact 2026-07-26: the runtime home and workspace overlay moved from the retired path to
`.compozy/` as a zero-legacy hard cut. Planning flag only; the next QA cycle owns a
fresh-boot and structured-diagnostics pass.

2026-08-23 qa-impact (Profiles): **reset from `pass` to `untested`** — the canonical home layout
changed. It now creates a `profiles/<name>/` tree per profile, and the durable memory root moved
from `$COMPOZY_HOME/memory/` to `$COMPOZY_HOME/profiles/default/memory/` with the old path a delete
target rather than a fallback read. Re-walk the fresh-boot layout assertion against the new tree,
and confirm status and doctor report only the new paths. The move's own crash-safety and
fail-closed guard are owned by `MS-profile-memory-tier-scope`.
