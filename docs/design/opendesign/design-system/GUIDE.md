# CompozyOS Design System — Author's Guide

Single source of truth for every future design/prototype in `docs/design/opendesign/`.
Read this before starting any surface. On conflict: **production (`packages/ui/src/tokens.css` + `web/src`) > this folder > any older prototype**. Shell-v2 edition (two themes, rail shell) — prototypes authored before it are off on several axes (see `index.html` § What changed in shell v2).

## Files

| File | Role |
| --- | --- |
| `ds-core.css` | Canonical tokens (`:root` = dark, `[data-theme="light"]` = light; bare names, shell-v2 values) + base + window-content components (buttons, fields, pills, tabs, rows, cards, tables, menus, key caps, srow, switch, radio cards, dialog system, empty, KPI). Link it or paste the `:root` verbatim. |
| `ds-shell.css` | OS chrome (shell v2): flat 52px topbar + tray pager, 60px rail (the dock), window frame/head/toolbar, **deck (browser tabs)**, gutterless seams, overlays, palette shell, toasts, compact mode. No glass, no radius waiver. |
| `ds-docs.css` | Chapter-page documentation chrome only — never used by product prototypes. |
| `index.html` | Overview, the twelve non-negotiables, the post-#440 change table. |
| `foundations.html` | 01 — tokens, color, type, ladders, radius, motion, icons, focus/a11y, divergence ledger. |
| `os-shell.html` | 02 — window manager (shell v2): anatomy, topbar, rail, window frame, deck, tiling/seams/arrange, desktops + wallpapers, themes, focus/z, compact, rules S1–S13. Both themes (`?theme=light`). |
| `components.html` | 03 — component catalog with live demos (every class ships in `ds-core.css`). |
| `command-palette.html` | 04 — canonical ⌘K anatomy (20px rail, row ladder, chord tiers, view states, action panel). |
| `modals.html` | 05 — dialog ladder, head/body/foot contract, forms, split/sheet, canonical selectors. |
| `settings.html` | 06 — srow vocabulary, info-shapes, save models, width ladder, G-rules. |
| `dashboards.html` | 07 — KPI grammar, chart cards, viz ink, flush-bottom panels, feeds, heatmaps. |
| `empty-states.html` | 08 — 48px well grammar, four variants, suggestion lifecycle, CTA budget. |
| `profiles.html` | 09 — identity color as user data, glyph ladder, switcher, pickers. |
| `patterns.html` | 10 — listings, drill-in, detail rhythm (32/26/320), status dictionary, truthful UI. |
| `PARITY.md` | DS class ↔ `@compozy/ui` export map + bare-token ↔ `--color-*` mapping. Replaces the shipped `UI-ALIGNMENT-SPEC.md`. |

## Starting a new prototype

1. One folder per design set at `opendesign/<slug>/`, sibling of `design-system/`; retired sets live under `_done/`. Self-contained HTML, semantic filenames.
2. Link `../design-system/ds-core.css` (+ `ds-shell.css` for shell surfaces) or paste the `:root` verbatim into the first `<style>`. Never rebind ramp tokens in a leaf board — if a value looks wrong, the root fix belongs here.
3. Fonts: Inter `wght@100..900` (425 regular and 620 display numerals need the axis) + Geist Mono, Google Fonts links as in the chapter files. Preview light with `data-theme="light"` on `<html>`.
4. Compose from the component classes — check `components.html` and `PARITY.md` before authoring anything; domain variants get domain-prefixed names, never forked geometry.
5. Surfaces render **inside an OS window**: unified 48px head (identity once, ≤2 actions, quiet controls last), optional 44px toolbar (views · filters · spacer · Rows|Cards), 40px deck at ≥2 tabs, drill-in via breadcrumb. No legacy topbar/PageHead; no views in the head (`.w2-tabs` is deprecated).
6. Data is runtime-plausible daemon truth; design empty/loading/error/degraded states (chapter 08).
7. `data-od-id` on regions, headings, controls, repeated cards. Lucide only (`data-lucide` + CDN + `createIcons()`), sized by container.
8. Iterate on existing files — never regenerate a delivered prototype from scratch.

## Hard rules (checkable)

- Every CSS color literal traces to the token set (or is a `color-mix` of it). Teal `#225555` = wallpaper depth only. Identity `--id` (profiles) is the only sanctioned inline color.
- Accent budget: orange = highlight / needs-you (badges, attention dots) — the primary action is the inverted `--primary` pill. Never card/panel borders, tab indicators, or selection markers.
- Geometry ladder: switch 32×18 · input 36 · `.ctl` 32 · search 28 · buttons 32/26/24/34 (pills) · pill 20 (18/24) · pill-group segment 30 (sm 20) · tabs list 40 (1.5px `--fg-strong` underline) · window head 48 · toolbar 44 · deck 40/tab 32 · srow ≥54 · property row 30 · icon well 34 (head well 26) · empty icon 48.
- Selection = `--selected` plate + `--fg`, no rim, no accent. Hover steps to `--surface-2` (or adds `shadow-card`); it never dims text.
- Eyebrows: sentence case 12/500 default; `.eyebrow-caps` 11/600/+.06em opt-in; uppercase never takes negative tracking. Key caps use `--font-keys`, one cap per binding.
- Reasoning = 7-bar `.im` meter (Medium = 4/7); quiet footers, accent only on fill/pressed/selected.
- Listing routes ship Rows|Cards; strip order locked (views → search/filters → spacer → display mode).
- Focus: keyboard `:focus-visible` = 2px ring at 50% white (dark) / 50% ink (light) (inset for full-bleed rows); pointer focus = border strengthen only. Reduced-motion double guard.
- Disabled controls swap tokens (`--disabled` ink), never opacity (buttons may use .5).
- Modals build on the `ds-core.css` `.dialog` system (bare names). The 16 delivered surfaces in `_done/modals/` keep their `--color-*` island — never mix both naming schemes in one file.

### Prototype directives (promoted from loops `DESIGN-LESSONS.md` §D, 2026-08-02 — evidence: `docs/_memory/lessons/L-034`)

- Transcribe production first; every delta annotated with its authority (`production` / `spec <id>` / `authorized delta`) — un-annotated divergence is a defect.
- Per domain: final pages + labs + `index.html`, one coherent cross-linked story (same entity/run ids across pages). Labs alone are an incomplete delivery.
- Color = state, never decoration. Badge budget: state pills only; enums/types as text or icon+text; zero counts render nothing.
- Every section collapsible (`details/summary`: icon · title · one-line gist · chevron); calm defaults; a closed section still informs.
- Machine truth demoted to micro mono (≤11px, faint), never removed.
- Canonical primitives (timeline, selectors, meters, modal chrome) are copied from the exported component, never approximated.
- No explainer cards in product chrome — hints/tooltips at point of use.
- Every control cites its gating payload field or route in an HTML comment; demos exercise the real state machine.
- Specs gate implementation scope, not what the user may preview — out-of-scope surfaces render tagged as proposals (`new · <spec>`).

## Legacy map

Retired design sets live under `_done/` (dashboard, modals, settings, empty-states, graph-eng, os-redesign-v1, …) — reference material only; their tokens and geometry predate #440. Living current sets: `../command-palette/`, `../profiles/`, `../loop-legibility/`. `UI-ALIGNMENT-SPEC.md` and `window-tabs-variations.html` were retired 2026-08-23: the alignment spec shipped in production, and the deck contract now lives in `os-shell.html` + `ds-shell.css`.
