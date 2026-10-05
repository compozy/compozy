---
id: ET-profile-identity-appearance-catalogs
area: ET
title: Pick profile identity from the full icon, emoji, and free-color catalogs
persona: Dora
journey: J-operate-profiles
expected: The identity picker offers the entire Lucide catalog in a searchable virtualized grid tinted by the chosen color, a full emoji catalog with search and skin-tone control served from local data (no CDN), and free color choice through a popover behind the spectrum toggle that never grows the dialog; every chosen symbol renders identically on the dock-foot switcher, Settings, and the command palette, a profile can be edited directly from a switcher row, and the daemon refuses icon slugs outside the catalog with a plain-language error.
entry_points: dock-foot switcher → Create profile / row edit button; Settings → Profiles → edit identity; POST /api/profiles; PATCH /api/profiles/{name}
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-10-02-untested/profile-identity-created-uds.json; docs/qa/evidence/2026-10-02-untested/profile-identity-tone-action-semantics.json; docs/qa/evidence/2026-10-02-untested/profile-identity-full-emoji-grid.png; docs/qa/evidence/2026-10-02-untested/profile-identity-custom-color-foreground.json; docs/qa/evidence/2026-10-02-untested/profile-identity-emoji-assets-tone.json; docs/qa/evidence/2026-10-02-untested/profile-identity-invalid-icon.json; docs/qa/evidence/2026-10-02-untested/profile-identity-final-icon-refresh.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-switcher-restore; ET-profile-web-settings-lifecycle-dialogs
---

Flagged by the profiles appearance overhaul (fix-adjustments). The final QA walk owns the real-user
evidence and verdict.

Walk:

1. Open Create profile, search the icon grid for a slug far outside the old bundled set (for
   example "binoculars" or "banana"), confirm it appears, scroll the grid and confirm it stays
   smooth past hundreds of icons, pick one, and create. Confirm the topbar glyph shows exactly that
   icon in the chosen color.
2. Edit the same profile from its switcher row using the row's edit button (without opening
   Settings), switch to the Emojis tab, search, change skin tone, pick an emoji, save, and confirm
   the glyph updates everywhere (topbar trigger, switcher menu, Settings list, command palette).
3. With devtools network open, confirm emoji data loads from the local `/assets/emojibase` path and
   nothing is fetched from a third-party CDN.
4. Open the spectrum toggle next to the hex field, confirm the saturation/hue picker opens in a
   popover without changing the dialog height, drag to a custom color, confirm the hex field and
   grid tint follow, and save.
5. From a terminal, run a profile update with an icon slug that is not a Lucide name and confirm the
   daemon refuses it with a plain-language message naming the slug; repeat with a valid slug and
   confirm it persists and renders.

Expected evidence: screenshots of the searched grid, the emoji tab with a non-default skin tone, the
color popover open over an unchanged dialog, the updated glyph on all four surfaces, the local-only
network log, and the terminal transcript for the rejected and accepted slugs.

Dependency upgrade replay (2026-10-05): read a pre-upgrade profile whose icon is `trash-2`,
confirm the API and UI resolve it to `trash`, then edit only its color and confirm the same
profile ID and icon survive. Repeat the boundary check for `album`, `book-marked`, `building-2`,
`flip-horizontal-2` and `flip-vertical-2`. Current evidence and pending checks are tracked in
`docs/qa/reports/2026-10-05-dependency-upgrades.md`.

QA 2026-08-26: Passed in an isolated lab. The full Lucide and local Emojibase catalogs, skin tone,
free color popover, switcher-row edit, cross-surface rendering, and daemon slug validation all matched
the contract.

qa-impact: 2026-09-30 shell rail v2. The switcher moved to the dock foot and identity ink is now measured against the active theme's surface. Reset to re-walk the picker and the rendered glyph in both themes.

qa-impact: 2026-09-30 shell rail Q2. The emoji tab's grid now fills the dialog width (its column count follows the pane width over the shared picker cell, like the icon grid), and the Edit/Create profile dialogs carry the quiet header close (hidden while saving). Walk: open Emojis in a 560px dialog and see a full-width grid; close with the X.

QA 2026-10-02: Dora created `research` with `binoculars`, edited it directly from the switcher,
saved medium-tone writing hand and `#3757a6`, then changed to `banana` through the CLI. Independent
HTTP/UDS reads and a fresh production-page load confirm the identity. Both saved symbols render on
the dock trigger, switcher row, Settings list, and Profiles palette in light and dark themes. The
560px dialog holds a 488px emoji pane with 17 columns; its height stays 567.5px with the spectrum
popover open. Icon scrolling reaches 1600px into the catalog, and the header X closes cleanly.
Local asset requests use the production `/assets/emojibase` prefix; no third-party emoji request
was observed. An invalid CLI icon is refused without changing the saved identity. The skin-tone
button announces its next action while `aria-valuetext` reports its current tone; their difference
is not an off-by-one selection defect. Recording closed with 67 frames.
