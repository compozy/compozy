---
id: REL-release-candidate-plan
area: REL
title: Prove the beta candidate with one pinned read-only planner
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: Both the release workflow and vendored release skill pin github.com/compozy/releasepr@v0.0.24; one read-only plan resolves the explicit candidate ref to checked-out HEAD, rejects a leading-v version and tags present locally or on origin, emits all nine authoritative outputs, and feeds downstream workflow policy without re-derivation while annotated tag creation remains workflow-owned and unexecuted.
entry_points: .github/workflows/release.yml; .agents/skills/releasepr/**; pr-release plan --ref <candidate> --version 0.3.0-beta.1 --channel beta; local and origin git tag guards
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/release-plan/release-plan-contract.txt; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/release-plan/workflow-consumption.txt; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/release-plan/leading-v.stderr; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/release-plan/ref-mismatch.stderr
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps:
---

story: As the release administrator, I can prove exactly what the candidate would publish without
creating or pushing a tag, release, package, signature, installer artifact, or DNS change.

Evidence must name `release_ref`, `release_commit`, `release_version`, `release_tag`,
`release_channel`, `github_prerelease`, `github_make_latest`, `npm_tag`, and
`homebrew_skip_upload`, then trace those values into the consuming workflow steps.

Task 12 QA plan: pre-publish only. Live publication and registry acceptance remain Task 10's
authorized human runbook.
