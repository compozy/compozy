---
id: REL-release-candidate-plan
area: REL
title: Prove the beta candidate with one pinned read-only planner
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: Both the release workflow and vendored release skill pin github.com/compozy/releasepr@v0.0.25; one read-only plan resolves the explicit candidate ref to checked-out HEAD, rejects a leading-v version and tags present locally or on origin, emits all twelve authoritative outputs, and feeds downstream policy without re-derivation. Dry-run and production install pinned Cosign v3 before GoReleaser, stage the same workflow tools, and run the same preflight; invalid assets or a dirty worktree fail before tag publication, while dry-run creates no tag or publication.
entry_points: .github/workflows/release.yml; .github/actions/setup-release/action.yml; scripts/release-preflight.sh; .agents/skills/releasepr/**; pr-release plan --ref <candidate> --version 0.3.0-beta.1 --channel beta; local and origin git tag guards
qa_status: pass
bug_ids: BUG-20260817-explicit-release-web-assets-sync
fix_status: fixed
retest_status: pass
fix_commits: 800fbce; 7846f4b
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
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

QA impact 2026-10-20: a manual recovery can reuse completed desktop artifacts from
`desktop_artifacts_run_id` only for the exact planned source commit and version.
Missing targets build normally; all reused targets run the normal packaged smoke
and inventory checks. Intel Apple ticket delivery uses the TCP mitigation for
actions/runner-images#14924.

QA verdict 2026-10-20: [recovery run 39448472762](https://github.com/compozy/compozy/actions/runs/39448472762)
passed for `0.3.0-beta.36` at source commit `7246386f7182015d76a0dc448626e2a57d35d9f6`.
Linux and arm64 artifacts were reused from run `39431274097`; only Intel was rebuilt.
The Intel DMG passed notarization, stapling, validation, and Gatekeeper over TCP.
All three packaged-desktop smokes and public CLI installation checks passed.
The public GitHub prerelease has 30 uploaded assets, `channel-beta` names `beta.36`,
and all three npm packages expose `beta=0.3.0-beta.36` with provenance. The Go proxy
serves `sdk/go/v0.3.0-beta.36` from the same source commit. Local actionlint,
workflow-contract tests, and eight artifact-selection probes passed; correction
commit `93c2ed588` also passed [CI run 39448422894](https://github.com/compozy/compozy/actions/runs/39448422894).

QA impact 2026-08-07: the release trust chain now pins Cosign v3.1.3, installs it before
`goreleaser-action@v7`, and exercises the same GoReleaser signature verification in dry-run and
production. This scenario was reset for a targeted release-candidate walk.

QA verdict 2026-08-07: local planner, preflight, tool ordering, version propagation, remote-tag
guards, and both real Sigstore bundles passed. The final GitHub-hosted release-PR dry-run remains
`blocked-verify` until it records both GoReleaser download verification messages on the candidate.
