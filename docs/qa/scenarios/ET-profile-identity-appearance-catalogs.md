---
id: ET-profile-identity-appearance-catalogs
area: ET
title: Pick profile identity from the full icon, emoji, and free-color catalogs
persona: Ada
journey: J-operate-profiles
expected: The identity picker offers the entire Lucide catalog in a searchable virtualized grid tinted by the chosen color, a full emoji catalog with search and skin-tone control served from local data (no CDN), and free color choice through a popover behind the spectrum toggle that never grows the dialog; every chosen symbol renders identically on the topbar, Settings, and the command palette, a profile can be edited directly from a switcher row, and the daemon refuses icon slugs outside the catalog with a plain-language error.
entry_points: menubar switcher → Create profile / row edit button; Settings → Profiles → edit identity; POST /api/profiles; PATCH /api/profiles/{name}
qa_status: untested
bug_ids:
evidence:
last_report:
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
3. With devtools network open, confirm emoji data loads from the local `/vendor/emojibase` path and
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
