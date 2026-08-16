---
id: REL-channel-repair-known-good
area: REL
title: Repair the desktop channel to a known-good generation
persona: Dora
journey: J-publish-compozy-beta
expected: Repair verifies every immutable release asset, records one audit commit, and atomically restores both platform manifests without exposing a partial generation.
entry_points: compozy-desktop-release repair -o json; channel-beta desktop manifests
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-beta-channel-contract; REL-beta-self-update
---

Added for the Electron channel authority. The walk must also remove one rollback asset and confirm
the repair refuses without moving the channel ref, then retry the same operation id and confirm the
structured result converges without a second flip.
