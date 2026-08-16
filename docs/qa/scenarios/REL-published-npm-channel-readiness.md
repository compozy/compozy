---
id: REL-published-npm-channel-readiness
area: REL
title: Wait for published npm channels to become observable
persona: Dora
journey: J-publish-compozy-beta
expected: After both npm publishes succeed, production waits at most ten minutes for @compozy/cli and @compozy/extension-sdk to expose the requested dist-tag at the exact release version; stale or absent tags are re-read, while query failures, malformed policy data, or a beta that moves latest stop immediately without republishing.
entry_points: GitHub Actions release.yml Production Release; Verify published channel policy; npm registry dist-tags
qa_status: untested
bug_ids: BUG-20260807-npm-dist-tag-readiness
fix_status: fixed
retest_status: pass
fix_commits: a46b424e; c38ba0fa
evidence: https://github.com/compozy/compozy/actions/runs/31817112935/job/94858267424
last_report: docs/qa/reports/2026-08-13-release-pipeline-recovery.md
overlaps: REL-beta-install-paths
---

QA impact 2026-08-07: hosted beta.6 proved the Cosign v3 path and published the GitHub Release plus
both npm packages. The job read the extension SDK dist-tag about two seconds after publish and
observed beta.5 even though npm had accepted beta.6; the registry later converged without any
republish. The working-tree fix waits on the observable dist-tag condition for at most ten minutes,
uses fresh registry reads, and keeps malformed responses, query errors, and beta/latest violations
terminal. The next hosted release must exercise the readiness loop before this scenario can pass.

QA impact 2026-08-13: publication custody moved after the public GitHub release and a real CLI
installation from its archives. The readiness loop remains the final registry convergence check,
but can no longer observe a version whose GitHub dependency is private. Reset to `untested` for
the next hosted release.

QA result 2026-08-14: the beta.16 package job published both packages only after GitHub and both
public CLI installs passed, then the bounded readiness loop observed `beta: 0.3.0-beta.16` for
`@compozy/cli` and `@compozy/extension-sdk`. The job completed successfully without republishing.
