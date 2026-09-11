# skill-sources — design notes

Six boards over the three surfaces of `.compozy/tasks/skill-sources/_uiux.md`
(S1 Settings › Skills — Sources section · S2 session composer `/` picker ·
S3 skills catalog & detail). Skill sources make discovery configurable: a curated
preset table (`compozy` always-on, `agents` default-on, `claude` available-off) plus
custom directories, workspace-level per-key overrides, origin attribution wherever a
skill renders, and per-skill sharing (symlinks into other ecosystems' enabled preset
roots — API verb `expose`) with four filesystem-reconciled health states. Companions:
`_spec.md` (behavior authority), `_user_stories.md` (states), `_tests.md` (UT-068..073,
E2E-007..011), `task_06.md` (web execution).

Note: `_uiux.md` declares "Artboards: none" — this set was requested explicitly as a
composition reference. It introduces **no new visual language**; every board composes
the existing settings/menu grammar, so implementation binds to production primitives,
not to these files. Visual-contract capture remains out of spec scope.

## Revision log

- **2026-08-23 — collapsible-rows pass** (review feedback: always-open folder lists
  were too dense; 10–20 sources would be a wall). One source = ONE 44px collapsible
  line (`details.sks-src` > `summary.sks-src__sum`), closed by default: chevron ·
  name · `custom` pill · tone pill when degraded · total count · control. Folder
  paths render only on expand. Production mapping: `Collapsible` from `@compozy/ui`;
  summary controls stop propagation. Board 01 gained a scale section (14 sources —
  scale-fixture names below). Boards 02/03 share the grammar (02 renders degraded
  rows open because states are the subject). Deleted: `.sks-src__head`,
  `.sks-src--flat`, `.sks-src--padb` (see the CSS ledger).
- **2026-08-23 — plain-language + parity pass** (review feedback: content too
  dev/operator-first, components off-standard, composer picker misaligned with the
  real session menu, density too high). Everything below reflects this pass; the
  original CLI-literal vocabulary is retired. Deleted with reason (see also the
  ledger in `skill-sources.css`): `.sks-src__slug` mono slugs on row heads ·
  `.sks-key` interleaved per-key table headers · `.sks-scope-line` CLI-mirror footer ·
  `.sks-cmd-input` fake command input · diagnostics mono key/value dump.
- First commit: full set delivered.

## Locked decisions

### Token lane — no rebind

`ds-core.css` on disk carries the post-#440 production values; this set binds it and
`ds-shell.css` unmodified. No parity-lane rebind. The graph-eng lab scaffold retired to
`_done/`, so chapter 31 of `skill-sources.css` carries a verbatim copy.

### Plain language first, machine truth demoted (review directive)

Every state, error, and posture leads with a short plain sentence a non-developer can
read. Machine truth is never removed — codes, paths, and slugs demote to micro mono
(≤11px, faint) after the sentence. Config-key ids (`skills.sources`) and CLI-mirror
sentences do not render in product chrome; provenance chips stay in the Advanced fold.

### Origin is taxonomy, never signal (US-013, S2/S3 shared vocabulary)

Origin labels are neutral mono `Pill`s (`.pill.pill--xs[data-mono]`) in slots the row
already owns: composer trailing slot, card foot. Never colored, never replacing the
tier string (`bundled`/`user`/`workspace` stay lowercase verbatim). `origin: ""`
(compozy-native) renders nothing. Custom rows carry a plain `custom` pill instead of a
mono slug.

### Preset display labels (authorized delta, pending daemon envelope truth)

`Compozy` · `Universal` · `Claude` — the parenthetical dot-folder was dropped because
each row lists its folder paths directly beneath the name. Share-target picker shows
the display label first with the dot-folder (`.agents`) as faint mono trailing.

### "Share" is the UI verb (flagged proposal — needs COPY.md sign-off)

User-facing copy on S3 reads **Sharing / Share to… / Share again / Stop sharing**.
The API/CLI verb stays `expose` (routes, payloads, status enum, error codes
untouched). If COPY.md rejects the split, boards 05–06 revert to Expose vocabulary
with the same anatomy.

### Signal mapping (all boards)

| State | Rendering |
|---|---|
| truncated root (US-014.AC-1) | warning `Pill` `partial scan` on the closed line + warning sentence `large folder — first 300 scanned` inside the disclosure; count stays neutral |
| directory absent (US-001.EC-1) | muted `no folder yet`, **no signal color** |
| unreadable root (US-003.EC-6) | danger `Pill` `can't read` on the closed line + danger sentence `can't read this folder` inside; **all counts omitted** |
| share healthy | `.d--ok` + `active` |
| share missing/broken (US-011.EC-3/4) | `.d--fail` hollow danger dot + sentence (`the link was deleted` / `the skill's folder moved`) + struck path + ghost repair actions |
| foreign_conflict (US-011.EC-7) | `.d--idle` faint dot + `another app's file is there`, **zero affordances** |
| always on (compozy row) | hollow neutral pill `always on`, no Switch |
| inherited / overridden (US-005) | neutral pills `same as global` (absent form) / `custom for this workspace` (hollow form) — never signal palette |
| save states | savebar dots: `.d--idle` pending · `.d--run.d--pulse` applying · `.d--ok` saved |

Dot and sentence always travel together; color is never the sole carrier.

### Truthful counts — the three never-lie states

Counts render only daemon-reported `roots[].skill_count` / `scanned_count`. Absent →
word instead of count. Unreadable → counts omitted entirely (never zeros). Runtime
unavailable → counts and existence suppressed, toggles stay editable (sources are
policy). One `count-chip` per source row (the total); per-root counts are faint mono
text (`.sks-root__count`), not chips. Diagnostics disclosure (`.sks-diag`) renders
plain sentences — one line per field of the per-root schema (`scanned`,
`skipped_links` + reason, `collisions` + qualified form, `verification`); names and
paths stay mono. The UI renders, never derives.

### Ineligible affordances are absent, not disabled

No Switch on the compozy row. No Sharing block on bundled skills. No repair actions on
`foreign_conflict`. Disabled presets are absent from the share target picker. Disabled
state on rows is a token swap (muted/subtle), never opacity.

### Locked copy (verbatim strings)

- `Saved · applied immediately` · `1 change pending` · `Applying…` (production savebar)
- `always on` · `on` · `off` · `no folder yet` · `can't read this folder`
- `N found · M usable` · `large folder — first 300 scanned` · `partial scan`
- `same as global` · `custom for this workspace` · `Use global` · `Customize`
- Group titles at workspace scope: `Sources` · `Your folders`
- `Defaults only` + "Only Compozy's built-in folders are on. Turn on a source above,
  or add your own folder."
- "**Compozy isn't reachable right now.** Skill counts are hidden until it's back.
  You can still change these settings."
- `Sharing` · `Share to…` · `Share again` · `Stop sharing` · `sharing…` · `active`
- `the link was deleted` · `the skill's folder moved` · `another app's file is there`
- Errors lead with a sentence; codes verbatim in micro mono, never paraphrased:
  `unknown_skill_source`, `duplicate_skill_source`, `invalid_source_path`,
  `expose_name_conflict`, `rolled_back`, plus the remaining expose codes on board 06.

### Icons

Lucide only, container-sized: `folder` on root lines (12px), lane icons in the picker
14px inside a 16px well (production `size-3.5` in `size-4`), `link` for sharing,
`settings-2`/`store`/`messages-square` as window glyphs. No icons beside headings.

### Composer picker — production parity (S2)

Mirrors `session-composer-command-menu.tsx` exactly: popover spans the composer width,
anchored above it, **no input of its own** (typing happens in the composer); group
header eyebrow + hairline between groups; one line per row — skill **name** in sans
(never the `/` token), description inline on the same baseline, trailing = scope in
sans for skills / token in mono for built-ins; selection = elevated plate +
`--fg-strong`. Transcript uses the canonical `.msg` grammar (roles `You` /
`Claude Code`) and the real `.composer` input.

### Lab layout

Staged widths (`:root` in the lane): `--sks-set-w:960px` (settings window),
`--sks-detail-w:1080px` (marketplace), `--sks-session-w:820px` (session),
`--sks-panel-w:640px` (`.plain` fragments). Window variants `.win--sks`, `.win--sksd`,
`.win--sksn`.

### CSS

Link order per board: fonts → `../design-system/ds-core.css` → `ds-shell.css` →
`skill-sources.css` (own lane LAST). Chapters: **31** lab scaffold (verbatim copy) ·
**32** lab fit · **33** source rows + key headings (S1) · **34** root diagnostics ·
**35** sharing rows + results + target picker (S3) · **36** composer picker popover
(S2). Append point at end of file. No `<style>` blocks in boards (hub exempt); no
inline styles.

### Primitives — reuse before create

| Board element | Class | Owner |
|---|---|---|
| settings frame | `.app/.sgroup/.panelbox/.srow` | ds-core |
| group description | `.sgroup__d` (one consequence sentence) | ds-core |
| source row | `details.sks-src` > `summary.sks-src__sum` → `Collapsible` | this lane / @compozy/ui |
| enable toggle | `.switch` 32×18 | ds-core |
| source total | `.count-chip` (one per row) | ds-core |
| per-root count | `.sks-root__count` faint mono text | this lane |
| origin chip | `.pill.pill--xs[data-mono]` | ds-core |
| state pills | `.pill` forms hollow/absent, tone warning | ds-core |
| save controls | `.savebar` | ds-core |
| scope selector | `.pill-group` | ds-core |
| notices | `.notice` info/danger | ds-core |
| empty state | `.empty` | ds-core |
| status dots | `.d` ok/fail/idle/run | ds-core |
| target picker | `.menu/.menu-item` + `.cbx` | ds-core |
| catalog cards | `.listing-card(-grid)` | ds-core |
| detail rail | `.railbox/.rail-sec/.prow` | ds-core |
| window chrome | `.win/.win-head/.win-toolbar` | ds-shell |
| transcript/composer | `.transcript/.msg*/.composer` | ds-core |
| set-only | `.sks-src/.sks-root/.sks-diag/.sks-keyhead/.sks-expo/.sks-result/.sks-cmd*/.sks-menu-anchor/.sks-target-menu` | this lane |

Production mapping (from `_uiux.md` component plan): `SettingsSkillSourcesSection`,
`SettingsSkillCustomSources`, `SkillExposePanel`; origin label is trailing-slot `Pill`
usage, no new `@compozy/ui` primitive anywhere.

## Canonical data story

Exactly these. Do not mint more.

| Fixture | Values |
|---|---|
| operator / workspace | Ana · `acme-api` (alt: `marketing-site`) |
| compozy | always on · `.compozy/skills` 8 · `~/.compozy/skills` 4 → 12 |
| agents | `Universal` · on · `.agents/skills` 5 (board 02: 5 found · 3 usable) · `~/.agents/skills` 2 |
| claude | `Claude` · off by default · when on: ws root absent, global 1 |
| team-skills | custom · `~/team-skills` · 3 skills (board 02: unreadable) |
| ml-corpus | custom, board 02 only · `~/ml-corpus` · partial scan: first 300 scanned · 214 |
| skills | `review-checklist` (native, sharing fixture) · `commit-hygiene` (agents) · `release-notes` (claude) · `deploy-runbook` (team-skills / broken+foreign fixture) · `frontend-qa` (homonym: winner native, shadowed `agents:frontend-qa`) |
| diag entries | skipped: `review-old` (dangling shortcut) · `vendor-link` (points outside the source); name clash: `frontend-qa`, winner Compozy |
| slug-hash example | `team-skills-3f2a` (picker truncation row) |
| scale list (board 01 §02 only) | + `client-acme` 6 (`~/clients/acme/skills`) · `client-nova` 4 · `data-pipelines` 5 · `design-reviews` 2 · `growth-playbooks` 3 · `marketing-ops` 4 · `onboarding-kits` 2 · `release-kits` 5 · `security-audits` 3 — 14 sources total with the presets + ml-corpus + team-skills |

## Staging that is not a VC

Settings nav rail omitted (boards scope one section). Detail rail rendered as isolated
fragment at rail width. Composer menu rendered in-flow above the composer (popover
anchoring is production's). Card grid cropped to one row. Transcript condensed to two
turns. `w2-meta` discovery counts in window heads are fixtures.

## Chapter map

31 scaffold · 32 lab fit · 33 source rows/keys · 34 diagnostics · 35 sharing ·
36 picker. Later runs append after the append point; never renumber.

## Board budget

| Board | Surfaces | Sections |
|---|---|---|
| 01 settings-sources | S1 default + scale | 5 |
| 02 settings-states | S1 states | 6 |
| 03 settings-scopes | S1 workspace/agent | 4 |
| 04 composer-picker | S2 | 4 |
| 05 catalog-origin | S3 default/healthy | 4 |
| 06 expose-states | S3 states | 4 |

Iterate on these files — never regenerate a delivered board from scratch.
