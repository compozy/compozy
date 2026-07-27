---
id: REL-beta-self-update
area: REL
title: Keep self-update on the running beta line
persona: Maia
journey:
expected: A v0.3 beta build offers only the newest non-draft v0.3 beta, ignores v0.2 stable, RC, future minor, and draft releases, keeps beta cache state separate from stable, and returns beta-safe npm, Go, or hosted-installer guidance without a Homebrew command.
entry_points: compozy update; compozy update --check -o json; GitHub releases API; managed install detection
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-beta-install-paths
---

QA impact 2026-07-27: Compozy migration Task 10 made update selection channel-aware and changed
managed lifecycle guidance to the active beta distribution identities. Planning flag only; the next
QA cycle owns real release/API behavior.

