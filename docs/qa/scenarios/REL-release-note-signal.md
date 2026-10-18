---
id: REL-release-note-signal
area: REL
title: Publish release notes with product changes only
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: Release PR changelogs and GitHub Release bodies retain user-facing features, fixes, refactors, breaking changes, and authored release notes while omitting repository-maintenance commits and squash-generated conventional titles appended after a breaking footer.
entry_points: pr-release release-body; GitHub release PR; GitHub Releases page and API
qa_status: skipped
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: REL-release-candidate-plan
---

This scenario owns the public signal-to-noise contract shared by the release candidate body and
published GitHub Releases. Repository maintenance remains available in Git history and pull
requests; it is not promoted as release-note content. Conventional child-commit titles appended by
GitHub after a `BREAKING CHANGE` footer remain commit history rather than becoming release bullets.

The 2026-08-01 local candidate replay passed through `releasepr release-body`, the formatted Markdown
path, and `git-cliff --context`. Public verification remains blocked until the working-tree fix reaches
`main` and the release workflow regenerates PR #272.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
