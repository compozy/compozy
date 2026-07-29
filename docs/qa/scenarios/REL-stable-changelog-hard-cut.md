---
id: REL-stable-changelog-hard-cut
area: REL
title: Keep the stable CompozyOS changelog inside the migration hard cut
persona: Dora
journey: J-approve-compozy-beta-candidate
expected: The generated v0.3.0 changelog and release PR include the CompozyOS migration commit plus later beta-line changes, exclude the three pre-migration commits owned by legacy/v0.2, and continue treating prerelease tags as snapshots whose changes belong to the stable release.
entry_points: cliff.toml; pr-release pr-release; CHANGELOG.md; RELEASE_BODY.md; release/v0.3.0 pull request
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Behavior changed after the first v0.3.0 release PR attributed post-v0.2.15 legacy fixes to the
CompozyOS release. Planning flag only; verify during the next release QA cycle.

## Steps

1. Generate or refresh the v0.3.0 release PR from a full checkout with tags.
2. Confirm the v0.3.0 section contains `Introducing CompozyOS beta` and changes committed after the migration cut.
3. Confirm it excludes `Reap leaked test daemons and artifacts`, `Archive without tasks`, and `Resolve inherited cross-runtime models against runtime defaults`.
4. Confirm the historical v0.2 changelog remains unchanged.
