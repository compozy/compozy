---
id: ET-web-skill-expose-panel
area: ET
title: Expose a skill and read its origin from the web
persona: Dora
journey: J-layer-profile-resources
expected: Skills absorbed from another tool carry a neutral origin label in the composer picker and on catalog cards while Compozy-native ones stay unlabeled; the skill detail exposes to enabled presets only, repairs links CompozyOS created, never offers an action on a foreign entry, and accounts for every target of a partial failure
entry_points: session composer `/` picker; Marketplace > Skills installed detail Exposures card
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-skill-origin-attribution, ET-skill-exposure-lifecycle, ET-session-composer-skill-chip
---

With one Compozy-native skill and skills absorbed from `agents`, `claude`, and a custom folder,
open a session and type `/`. The absorbed rows carry a small neutral origin label beside the tier
word; the native row carries none and is otherwise unchanged. Add a homonym across two roots and
confirm both stay reachable and distinguishable.

On the installed-skill detail, expose to one enabled preset and confirm the row reads active with
the link path. Delete the link on disk and reload: the row reads `the link was deleted` with
repair actions; `Expose again` restores it. Replace the link with a foreign symlink and reload:
the row reads `another app's file is there` and offers no action at all. Expose to two targets
where one path is occupied and confirm the failure names both targets, marks the compensated one
rolled back, and keeps the daemon's codes verbatim. Confirm a bundled skill shows no Exposures
card at all — absent, not disabled.

Covers UT-072, UT-073, E2E-010, E2E-011.
