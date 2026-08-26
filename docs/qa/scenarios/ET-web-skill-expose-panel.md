---
id: ET-web-skill-expose-panel
area: ET
title: Expose a skill and read its origin from the web
persona: Dora
journey: J-share-skills-with-other-tools
expected: Skills absorbed from another tool carry a neutral origin label in the composer picker and on catalog cards while Compozy-native ones stay unlabeled; the skill detail exposes to enabled presets only, repairs links CompozyOS created, never offers an action on a foreign entry, and accounts for every target of a partial failure
entry_points: session composer `/` picker; /marketplace/skills; /marketplace/skills/$entryId installed detail Exposures card
qa_status: pass
bug_ids: BUG-20260825-expose-picker-crashes;BUG-20260825-workspace-native-skill-missing
fix_status: fixed
retest_status: pass
fix_commits: df739b0
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/exposure-summary.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-skill-exposure-lifecycle; ET-skill-origin-attribution; ET-session-composer-skill-chip; ET-web-marketplace-installed-management
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

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-share-skills-with-other-tools`. Entry points now use the real routes — the installed detail is `/marketplace/skills/$entryId`, not a singular `skill` segment. Stable selectors exist for this walk (`skill-exposures-card`, `skill-expose-panel` plus its `-row-{target}`, `-row-{target}-expose-again`, `-row-{target}-unexpose`, `-result-{target}`, `-failure` children, and `skill-expose-target-picker-{trigger|option-{slug}|confirm|none}`), so the browser pass asserts on them rather than on copy. Charter: `CH-skill-expose-web-repair`.
