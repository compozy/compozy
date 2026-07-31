---
id: REL-release-note-signal
area: REL
title: Publish release notes with product changes only
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: Release PR changelogs and GitHub Release bodies retain user-facing features, fixes, refactors, breaking changes, and authored release notes while omitting every docs, build, and CI conventional commit, including scoped and breaking forms.
entry_points: pr-release release-body; GitHub release PR; GitHub Releases page and API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-release-candidate-plan
---

This scenario owns the public signal-to-noise contract shared by the release candidate body and
published GitHub Releases. Repository maintenance remains available in Git history and pull
requests; it is not promoted as release-note content.
