---
id: REL-electron-cutover-announcement
area: REL
title: Announce the Electron desktop cutover and manual replacement
persona: Ada
journey: J-publish-compozy-beta
expected: Release notes direct each supported architecture to the current GitHub release, preserve the existing home and package identity, and tell portable Linux users when to delete the old AppImage.
entry_points: RELEASE_BODY.md; RELEASE_NOTES.md; getting-started installation and desktop-app docs; operations desktop-app and desktop-release runbook docs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-release-note-signal; APP-install-first-run-provision
---

Added for the one-time desktop cutover. The walk must compare the announcement with the published
asset names and confirm that no automatic cross-channel migration or fallback feed is promised.
