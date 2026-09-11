# Profiles — visual contract

Eight boards from `.compozy/tasks/profiles/_uiux.md` (S1–S14). Profiles are who-is-working contexts: work is separate per profile, project folders and machine tools are shared. The boards cover the menubar switcher, aggregate listings, the Settings page, the identity picker, lifecycle dialogs, extension placements, project hints, and the command-palette Profiles view.

This file is the locked semantic contract — every glyph, chip, boundary sentence, and dialog button traces back here. Companions: `_spec.md` Part I (behavior authority), `_user_stories.md` (ACs/ECs), `_dx.md` (non-UI half). Implementation tasks cite these boards as visual contracts.

## Locked decisions

### Post-#440 parity lane (the defining decision of this set)

`design-system/ds-core.css` predates PR #440 (`636bc823`, the normie-friendly pass) and is **stale drift** against `packages/ui/src/tokens.css`. Following the command-palette precedent, `profiles.css` opens with a set-scoped parity lane that rebinds the ds-core token names to current production values. Siblings don't link this file, so nothing leaks. The root fix belongs in `ds-core.css`; until it lands, this lane is the only sanctioned rebind.

What moved in #440 (all bound in the lane):

| Axis | Pre-#440 (ds-core) | Post-#440 (tokens.css) |
| --- | --- | --- |
| Surface ramp | canvas `#131211` · soft `#1a1918` · tint `#1c1b1a` · elevated `#232220` | canvas `#171615` · soft `#1f1e1c` · tint `#232220` · elevated `#2a2927` |
| Text ladder | cool-violet (`#ececef`…`#545458`) | warm-neutral: fg `#eeedeb` · strong `#f7f6f4` · muted `#a4a29e` · subtle `oklch(0.663 0.004 75)` · faint `oklch(0.638 0.003 75)` |
| Glaze ladder | hover `.022` · selected `.03` · input `.025` · btn `.04/.07` | hover `.045` · selected `.06` · input `.05` · btn `.07/.10` — anything under `.045` reads as no affordance |
| Radius | 4/5/6/8/10/14 | xs 6 · sm 7 · base 8 · md 10 · lg 14 · xl 18 (xxs 3 and pill unchanged) |
| Motion | 100/140/200ms, `cubic-bezier(.2,0,0,1)` | 120/180/260ms, `--ease: cubic-bezier(.22,1,.36,1)`; fixes the malformed 5-arg `--ease-in-out` |
| Body | 13.5px / 1.5 | 15px / 1.55 |
| Eyebrow | 10px 600 uppercase | **12px 510 sentence case**; uppercase survives only as the opt-in `.eyebrow-caps` kicker (11px 600 +0.06em) |
| Key caps | mono 10px | `--font-keys` (system sans) 10.5px 510 +0.03em — key caps are never mono |
| Weights | 500/600 | medium is **510**; display numerals 620 (needs the Geist variable axis — fonts load `wght@100..900`) |

Component patches ride in the same chapter: `.btn` 30px (sm 26), `.pill` 20px + `--radius-pill`, `.pop-eyebrow`/`.pal-group` demoted to sentence case, `kbd` on `--font-keys`. Focus ring (2px @ 50% white) and the signal palette are unchanged — do not touch them.

### Identity color is user data, never a signal (US-001, US-002)

- The profile's color/symbol render exactly as chosen — Linear-style free hex plus a suggested palette. Identity flows as a per-element `--id` custom property (`style="--id:#C26AD6"`), the one sanctioned inline style besides meter widths. It never becomes a CSS token.
- Glyph anatomy: rounded square, fill `color-mix(in oklab, var(--id) 20%, transparent)`, ink `color-mix(in oklab, var(--id) 72%, #fff)` — computed toward white for AA on the dark ramp.
- Color never carries meaning alone: name + symbol are always present. The active marker is a ring in the identity color on the glyph (data, not signal).
- The suggested palette avoids the exact signal hexes (`#5FBF85 #D6A647 #E0635A #8E8EB5 #E8572A`); a user may still type them — that's their data.

### Signal mapping (locked, from `_uiux.md`)

- Needs setup (unfilled credential asks) = **warning** `#D6A647` badge on glyph/row + the words "needs setup".
- Archived owner in aggregate = **info** `#8E8EB5`, muted row tag + the word "archived".
- Destructive delete = **danger** `#E0635A` on the confirm action **only** — never on the dialog frame, never on archive.
- Created/renamed success toast = **success** `#5FBF85` accent.
- Dormant content hint = **info** inline hint with a create action. Absence stays calm: dormant placements and ignored hints use the neutral ramp.
- Blocked archive (running sessions) = warning naming the sessions, not danger — nothing is broken, something is in use.

### Quiet until plural (US-007.EC-2, US-010.EC-1)

With only `default`, the switcher is a neutral icon button — no name, no glyph color, no ceremony anywhere. Its menu holds the default row, "Create profile…", and the boundary sentence. Every other surface renders exactly as today until a second profile exists.

### Truthful UI

- No per-profile controls for machine facts. Sandboxes, scheduler budgets, native provider logins say "machine-level" plainly (US-021.EC-3).
- Remote surfaces render management **absent, not disabled** (US-032.AC-2): the list is readable, create/edit/archive simply aren't in the DOM.
- Delete appears only on an archived profile with no work (US-006.AC-2); otherwise the flow routes to archive. Worktree rows always carry the owner tag, even scoped (US-009.EC-1).
- Owner tags appear **only in aggregate mode** — scoped views stay tag-free (calm default, SD-012).

### Locked copy (COPY.md register — sentence case, no helper prose under headings)

- Boundary answer (switcher menu footer, verbatim): **"Work is separate per profile. Project folders and machine tools are shared."**
- Separation line (Settings + create dialog, verbatim): **"Profiles keep work separate. They are not a security boundary."**
- Scoped empty state: **"No sessions in Marketing yet."** — names the active profile, then the next action.
- Destination chip is the fixed text **"→ default"** — a label, never a picker (ADR-005).
- Owner toast after creating under All: **"Created in default."**
- Archived fallback toast: **"Old agency was archived. You're back on default."**
- Uninstall copy: **"The growth profile and its work stay."**
- Vocabulary: the UI says **project**, never "workspace"; "CompozyOS", never "daemon". Runtime nouns get one plain clause on first use.

### Where an icon belongs

Lucide, stroke 1.75, sized by the canon ladder (14px default, 12px inline/rail, nothing over 16px inside rows). Profile symbols are the user's choice and render inside `.pf-glyph` only. No icons beside headings.

### Lab layout

Boards are states sheets: §01 is the final composed surface inside a `.stage` (with menubar or window chrome when the piece needs its host), §02+ walk states as `.spec` note/demo pairs. Staged widths live in the LAB FIT chapter of `profiles.css` — never inline. Viewport target 1280–1440; `.spec` collapses under 760px via graph-eng.

- `--pf-menu-w: 280px` switcher menu · `--pf-modal-w: 560px` lifecycle dialogs · `--pf-win-w: 960px` listing window · `--pf-set-w: 760px` settings window content.

### CSS

`profiles.css`, linked after `ds-core.css → ds-shell.css → graph-eng.css` (own file last so the parity lane wins). It carries: the post-#440 parity lane (chapter 20), the `pf-` domain families (21–29), LAB FIT (30). No `<style>` blocks in boards, no inline styles except `--id` and meter widths. Chapters start at 20 — graph-eng owns 1–12 and loop-legibility claimed 13–19.

### Primitives — reuse before create

Production composes from `@compozy/ui`: the switcher is `CommandSelect*` + `Avatar` (never a `GlobalScopeToggle` fork); the picker ships as the new `SymbolPicker` primitive; everything else is composition (see `_uiux.md` component plan).

| Board element | Class used | Owner |
| --- | --- | --- |
| menubar, switcher trigger, icons | `.menubar .mb-side .mb-icon .ws-trigger` | ds-shell |
| switcher menu | `.popover .pop-item .pop-sep .pop-eyebrow` | ds-shell (patched case) |
| profile glyph / owner tag / destination chip | `.pf-glyph .pf-owner .pf-dest` | profiles ch21 |
| listing window + rows | `.win .win-toolbar .list-shell .listing-row` | ds-shell / ds-core |
| state chips | `.pill[data-tone]` + glyph + word | ds-core |
| notices / owner banner | `.notice` (+ `.pf-banner` grid) | ds-core / profiles ch23 |
| settings rows / disclosure | `.panelbox .srow .adv-toggle .switch` | ds-core |
| dialogs | `.pf-modal` (overlay sheet on `--shadow-overlay`) | profiles ch26 |
| identity picker | `.pf-picker` family | profiles ch25 |
| rename plan tiers | `.pf-plan` family | profiles ch26 |
| project hint | `.pf-hint` | profiles ch28 |
| palette | `.palette .palette-head .pal-item` | ds-shell (patched case) |
| toasts | `.toasts .toast` | ds-shell |
| key caps | `.keys .key` | profiles ch20 (post-#440 `--font-keys`) |
| empty state | `.empty` | ds-core |

### Retired before first commit

- `.pf-badge` (bespoke needs-setup badge) — `.pill[data-tone="warning"]` already owns it.
- `.pf-check` (bespoke active checkmark) — `.pop-item .pi-check` already owns it.
- Per-key chord caps — one `.key` carries the whole chord (command-palette lock).

## Canonical data story

Exactly these profiles. Do not mint more.

| Profile | Identity | State | Work |
| --- | --- | --- | --- |
| `default` | `#8A8F98` · `user-round` (auto-assigned starter) | permanent, active on `docs-hub` | 8 sessions · 2 loops · pre-profiles history |
| `Marketing` | `#C26AD6` · `megaphone` | **active profile in most boards**, active on `acme-site` | 3 sessions · 1 automation ("Weekly digest") |
| `Consulting` | `#4EA7FC` · `briefcase` | active on `client-alpha` | 5 sessions · 2 worktrees |
| `growth` | `#4CB782` · `trending-up` | created by extension `growth-kit`, **needs setup** (1 credential: Ads API key) | 0 |
| `Old agency` | `#B58E5F` · `folder` | **archived** | 4 sessions held |
| `scratch` | `#8A8F98` · `pencil` | archived, **empty** (the only delete-eligible profile) | 0 |

Projects (UI word for workspace): `acme-site`, `client-alpha`, `docs-hub`. Repo-declared profile folders in `acme-site`: `dev`, `marketing` (S9 hint). Rename fixture: `Marketing` → `Brand studio`.

## Staging that is not a VC

Window/desktop restoration (S12) is behavior, not visual language — covered by a note on the switcher board, verified in runtime E2E. Scoped observe surfaces (S10 first half) render nothing new; only the machine-dashboard breakdown is staged.

## Chapter map (`profiles.css`)

20 parity lane + base patches · 21 identity (glyph, owner tag, destination chip) · 22 switcher menu · 23 owner banner · 24 settings page · 25 identity picker · 26 dialogs + rename plan · 27 extension placements · 28 project hint · 29 palette view · 30 LAB FIT.

## Board budget

| Board | Surfaces | Sections |
| --- | --- | --- |
| `profiles-switcher.html` | S1 · S11 (· S12 note) | 6 |
| `profiles-aggregate.html` | S2 · S3 · S10 | 6 |
| `profiles-settings.html` | S4 · S13 | 6 |
| `profiles-picker.html` | S5 | 5 |
| `profiles-lifecycle.html` | S6 · S7 | 5 |
| `profiles-extension.html` | S8 | 4 |
| `profiles-hints.html` | S9 | 3 |
| `profiles-command-palette.html` | S14 | 5 |
