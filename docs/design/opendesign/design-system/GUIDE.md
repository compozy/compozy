# CompozyOS Design System — Author's Guide

Single source of truth for every future design/prototype in `docs/design/opendesign/`.
Read this before starting any surface. On conflict: **production (`packages/ui/src/tokens.css` + `web/src`) > this folder > any older prototype**.

## Files

| File | Role |
| --- | --- |
| `ds-core.css` | Canonical tokens (`:root`) + base + window-content components. Copy the `:root` verbatim into new prototypes. |
| `ds-shell.css` | OS chrome: menu bar, dock, window frame/head/strip, overlays, mobile mode. Radius-waiver layer (12–22px). |
| `index.html` | Overview + the twelve non-negotiables + supersedes ledger. |
| `foundations.html` | Color, type, spacing ladder, radius, motion, icons, a11y, prototype-vs-production divergence ledger. |
| `os-shell.html` | Shell contract with live anatomy demos (root head, drill-in, document window, dock). |
| `components.html` | Component catalog with demos (every class ships in `ds-core.css`). |
| `patterns.html` | Surface patterns: listings, drill-in, detail, settings, modals + canonical selectors, sessions, dashboard, states. |
| `UI-ALIGNMENT-SPEC.md` | Executable spec aligning `packages/ui` production primitives to this DS (tokens, per-component targets, consolidations, P0–P2). |

## Starting a new prototype

1. Self-contained HTML in the domain folder (`tasks/`, `settings/`, `os/`, …), semantic filename.
2. Paste the `ds-core.css` `:root` verbatim into the first `<style>` (or link `../design-system/ds-core.css` for multi-file work). Shell surfaces also take `ds-shell.css`.
3. Fonts: Geist + JetBrains Mono (Google Fonts links as in the chapter files).
4. Compose from the component classes — check `components.html` before authoring anything; domain variants get domain-prefixed names, never forked geometry.
5. Surfaces render **inside an OS window**: unified 44px head (identity once, ≤2 actions), optional 38px tools strip, drill-in via breadcrumb. No legacy topbar/PageHead.
6. Data is runtime-plausible daemon truth; design empty/loading/error/degraded states.
7. `data-od-id` on regions, headings, controls, repeated cards.
8. Iterate on existing files — never regenerate a delivered prototype from scratch.

## Hard rules (checkable)

- Every CSS color literal traces to the token set (or is a `color-mix` of it). Teal `#225555` = wallpaper depth only.
- Accent budget: 1 primary action + live dots + selection markers per screen. No accent card/panel borders.
- Switch 32×18 · form input 36 · settings `.ctl` 32 · search 28 · button 26/22 · pill-group segment 24 (sm 22) · window head 44 · strip 38 · srow ≥54.
- Reasoning = 7-bar `.im` meter (Medium = 4/7); reasoning controls stay quiet footers (track ≤16px equivalents; accent only on fill/pressed/selected).
- Listing routes ship Rows|Cards; toolbar order locked (search/filters → spacer → view pills).
- Sessions is a normal OS window; minimize folds into the dock icon; Settings lives in the menu-bar cog; tray = bell · ⌘K · cog.
- Focus: keyboard `:focus-visible` = 2px white ·5 ring; pointer focus = border strengthen only. Reduced-motion double guard.
- Modals build on `modals/modal-system.css` (`--color-*` names there; bare names in prototypes — never both in one file).

### Prototype directives (promoted from loops `DESIGN-LESSONS.md` §D, 2026-08-02 — evidence: `docs/_memory/lessons/L-034`)

- Transcribe production first; every delta annotated with its authority (`production` / `spec <id>` / `authorized delta`) — un-annotated divergence is a defect. Parallel transcription-grade deep-dives (exact px, tokens, class/icon names) precede any redesign of an implemented surface.
- Per domain: final pages + labs + `index.html`, one coherent cross-linked story (same entity/run ids across pages). Labs alone are an incomplete delivery.
- Color = state, never decoration. Badge budget: state pills only; enums/types as text or icon+text; zero counts render nothing. Richness comes from structure (icon tiles, kv rows, handles, rings), never hue.
- Lucide only (`data-lucide` + CDN + `createIcons()`), one icon per concept, sized by container.
- Every section collapsible (`details/summary`: icon · title · one-line gist · chevron); calm defaults; a closed section still informs.
- Machine truth demoted to micro mono (≤10.5px, faint), never removed.
- Canonical primitives (timeline, selectors, meters, modal chrome) are copied from the exported component, never approximated.
- No explainer cards in product chrome — hints/tooltips at point of use.
- Every control cites its gating payload field or route in an HTML comment; demos exercise the real state machine.
- Specs gate implementation scope, not what the user may preview — render requested out-of-scope surfaces tagged as proposals (`new · <spec>`).

## Legacy map

Deleted (2026-07-22, superseded by this folder): `systems/design-system.html`, `systems/catalog-design-system.html`, `settings/settings-design-system.html`, `modals/modal-design-system.html`.
Living references: `os/compozy-os-v2.html` + `os/os-v2.js` (shell behavior), `os/pagehead-redesign.html` (head lab), `dashboard/dashboard.html`, `tasks/task-detail.html`, `_done/agents/agent-detail.html` (RuntimeSelector), `_done/shell/sidebar-sessions-02.html` (session lists), `modals/` (library + `MODAL-STANDARD.md` + `verify.mjs`).
