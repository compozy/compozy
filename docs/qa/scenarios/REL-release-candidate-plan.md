---
id: REL-release-candidate-plan
area: REL
title: Prove the beta candidate with one pinned read-only planner
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: Both the release workflow and vendored release skill pin github.com/compozy/releasepr@v0.0.25; one read-only plan resolves the explicit candidate ref to checked-out HEAD, rejects a leading-v version and tags present locally or on origin, emits all twelve authoritative outputs, and feeds downstream policy without re-derivation. Dry-run and production install pinned Cosign v3 before GoReleaser, stage the same workflow tools, and run the same preflight; invalid assets or a dirty worktree fail before tag publication, while dry-run creates no tag or publication.
entry_points: .github/workflows/release.yml; .github/actions/setup-release/action.yml; scripts/release-preflight.sh; .agents/skills/releasepr/**; pr-release plan --ref <candidate> --version 0.3.0-beta.1 --channel beta; local and origin git tag guards
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-release-cosign-v3-20260807-170358-478309-lab/qa-artifacts/qa/evidence/release-candidate-contract.txt; /Users/pedronauck/dev/qa-labs/compozy-release-cosign-v3-20260807-170358-478309-lab/qa-artifacts/qa/evidence/cosign-real-bundles.txt
last_report: docs/qa/reports/2026-08-07-release-cosign-v3.md
overlaps:
---

story: As the release administrator, I can prove the candidate satisfies the same local pre-publish
contract as production without creating or pushing a tag, release, package, signature, installer
artifact, or DNS change.

Evidence must name `release_ref`, `release_commit`, `release_version`, `release_tag`,
`release_previous_tag`, `release_git_range`, `release_initial`, `release_channel`,
`github_prerelease`, `github_make_latest`, `npm_tag`, and `homebrew_skip_upload`, trace those values
into the consuming workflow steps, and show that the shared preflight rejects both tracked and
untracked release-workspace contamination.

Task 12 QA plan: pre-publish only. Live publication and registry acceptance remain Task 10's
authorized human runbook.

QA impact 2026-08-07: the release trust chain now pins Cosign v3.1.3, installs it before
`goreleaser-action@v7`, and exercises the same GoReleaser signature verification in dry-run and
production. This scenario was reset for a targeted release-candidate walk.

QA verdict 2026-08-07: local planner, preflight, tool ordering, version propagation, remote-tag
guards, and both real Sigstore bundles passed. The final GitHub-hosted release-PR dry-run remains
`blocked-verify` until it records both GoReleaser download verification messages on the candidate.
