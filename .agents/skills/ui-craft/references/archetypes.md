# UI Archetypes

Read this when picking the right pattern for a surface, when "we need a [thing] like Linear / Notion / Vercel" is the brief, or when the team is reinventing a primitive that already has a known signature. Archetypes are not templates. They are contracts: each one has a fixed signature across motion, spacing, typography, and state. Break the signature and the user no longer recognizes the pattern — they just see a broken version.

7 archetypes cover ~80% of modern product UI. Use the six-slot schema to evaluate any candidate.

## The Six-Slot Schema

Every archetype defines:

| Slot              | What it locks                                                      |
| ----------------- | ------------------------------------------------------------------ |
| **Pattern**       | One sentence describing the contract                               |
| **Style Priority**| Product vs Brand register (which side of `SKILL.md` rules applies) |
| **Color Mood**    | Chroma range + role assignments (where saturation is allowed)      |
| **Typography Mood**| Scale + weight contract                                            |
| **Key Effects**   | Motion + materials that carry the feel                             |
| **Anti-patterns** | Things that break the contract — you are no longer doing this archetype |

Apply all six. Missing any slot is missing the archetype.

---

## 1. Bento Grid

**Pattern:** Asymmetric modular tile layout where related-but-distinct facets of a thing live side by side at different sizes — pioneered by Japanese lunchboxes, popularized by Apple, M1 marketing, and Vercel/Linear feature grids.

**Style Priority:** Brand. Bento is rarely the right product-chrome pattern; it is a presentation layer.

**Color Mood:** Restrained palette per tile, but the *set* can carry color variety. One tile per accent role. Background is usually `--surface-1` to let tiles breathe.

**Typography Mood:** Per-tile hierarchy. Large display-weight numerals or product names; supporting body text small. Vertical rhythm is per-tile, not page-wide.

**Key Effects:**
- Tile radius: `radius-lg` to `radius-xl` (16–24px)
- Gap: 12–16px on desktop; tighter on mobile (8–12px)
- Each tile has an individual hover state (subtle lift via `translateY(-2px)` + shadow increase, 200ms `ease-out-quart`)
- Optional: one tile carries a small motion loop (data scroll, pulse) to anchor attention
- Density is the feature — small tiles imply scannable facets

**Anti-patterns:**
- All tiles the same size → it's a card grid, not bento
- More than 8 tiles per screen → it's a list, not bento
- Same content type in every tile → it's a uniform grid, not bento
- No anchor/large tile → no focal point; the eye wanders
- Tiles centred with empty page margins → `CenteredEverything` slop hit

**Use it for:** Landing pages, feature overviews, "what's new" releases, app dashboards with mixed-shape KPIs.
**Don't use for:** Lists of records, sequential flows, settings pages.

---

## 2. Command Palette

**Pattern:** Spotlight-like fuzzy-search overlay invoked by `⌘K` / `Ctrl+K`, surfacing actions, navigation, and entities in one ranked list.

**Style Priority:** Product. Even on a brand-heavy site, the command palette is product mechanism, never brand expression.

**Color Mood:** Highest elevation surface (`--surface-4`), strongest contrast. Search input prominent. Result icons monochrome stroke. Highlighted (top) result uses the action accent at low saturation.

**Typography Mood:**
- Input: `text-base` or `text-md`, regular weight
- Result label: `text-sm`, medium weight
- Result kind / context: `text-xs`, regular, secondary color
- Keyboard shortcut chips: monospace, tabular-nums
- Section headers: `text-xs`, uppercase, letter-spacing +0.04em

**Key Effects:**
- Open via `⌘K`. Close via `Esc` or click outside. Always.
- Top result auto-highlighted, action triggered by `Enter`
- ↑/↓ navigates; preserves scroll
- Mouse hover does **not** override keyboard selection — many implementations get this wrong
- Result list: 200–500ms debounced fuzzy filter; instant for < 50 items
- Empty state: shows recent / suggested actions, never blank
- Backdrop: blurred (`backdrop-filter: blur(8px)`) — the one place glassmorphism is correct
- Open animation: 180ms `ease-out-quint` scale 0.96 → 1 + opacity 0 → 1
- Result row height fixed for keyboard predictability (36–44px)

**Anti-patterns:**
- Mouse hover replaces keyboard selection → broken keyboard contract
- Result rows variable height → keyboard nav feels jittery
- No `⌘K` discovery (no hint, no menu link) → users never find it
- Used only for navigation, not actions → underused
- Closes on every keystroke (popover-style) → it's not a palette, it's a dropdown
- Slow filter (> 100ms after first character) → feels broken

**Use it for:** Power-user navigation, action shortcuts, entity search, recent-history jump.
**Don't use for:** Form input, multi-step flows, content editing.

**Canonical references:** Linear (the modern benchmark), Raycast, Notion, Vercel.

---

## 3. Focus Mode

**Pattern:** Single primary task in foreground; everything else dimmed or hidden until exit. Pulls the user into deep work — writing, reviewing, configuring — without surrounding chrome distracting.

**Style Priority:** Product. Focus mode amplifies the product's core surface.

**Color Mood:** Surrounding chrome drops to `--surface-1` or fully hides. The focused surface stays at `--surface-3`. Page background may darken (overlay at 40–60% opacity) to push attention.

**Typography Mood:** Increased line-height (+0.1), increased measure (up to ~80ch for prose), slightly larger body text if appropriate. Headings keep the system scale.

**Key Effects:**
- Entry: 300–400ms `ease-out-quart`. Chrome fades out; focused surface rises slightly.
- Exit affordance always visible (close icon top-right, `Esc` keyboard hint)
- No surrounding chrome motion — chrome that survives is static
- Scroll restored to the focused surface only; surrounding page scroll-locked
- Optional dim-on-pointer-away (subtle, 1500ms delay; off by default)

**Anti-patterns:**
- No clear exit → user feels trapped
- Surrounding chrome still demands attention (notifications, hover overlays) → not focus mode
- Multi-task surface within focus mode → it's a modal, not focus
- Permanent state (no exit) → it's a layout, not a mode
- Hidden `Esc` hint → keyboard users get stuck

**Use it for:** Long-form editing, reviewing AI-generated content, immersive viewing.
**Don't use for:** Quick edits, multi-input forms, settings.

**Canonical references:** iA Writer, Ulysses, Google Docs "Focus Mode," Notion's "Reader Mode."

---

## 4. Live Status Indicator

**Pattern:** Persistent UI element communicating that something is happening *right now* — agent thinking, build running, deployment in flight, real-time count updating. The respiration of the product.

**Style Priority:** Product.

**Color Mood:** Semantic, not decorative. Green/yellow/red dot per state. Accent color only when the system is in a "doing useful work" state, never on idle.

**Typography Mood:** `text-xs` or `text-sm`, tabular-nums for counts. Last-updated timestamp in secondary color.

**Key Effects:**
- Pulse: 1.5–2s respiration cycle on the dot when "live"
- Pulse uses `opacity` only — no `scale` — to stay compositor-cheap
- Count updates ease in (200ms) — never snap
- **Honor `prefers-reduced-motion: reduce`** — replace pulse with static dot; preserve count animation only if subtle
- On state change (live → error), color transitions over 240ms; do not flash
- Tooltip on hover: last-updated time, click-through to detail view

**Anti-patterns:**
- Pulse uses `scale` → triggers compositor cost on every cycle
- No reduced-motion fallback → vestibular failure
- Updates snap discontinuously (count jumps 5 → 47) → reads as broken
- Status color is decorative ("brand purple for live") → loses signal
- Disappears when "stable" instead of showing "all green" → silence reads as broken

**Use it for:** Agent activity, build/deploy state, real-time collaboration presence, queue depth.
**Don't use for:** One-off completions (use toast), historical data (use list).

**Canonical references:** Vercel deployment status, Linear's cycle progress, GitHub Actions live status, Google Meet's "X others are here."

---

## 5. Intelligent List

**Pattern:** Long list of records with type-ahead filter, multi-select, batch actions, keyboard-first navigation, and virtual scrolling past a threshold. The workhorse of any data-heavy product.

**Style Priority:** Product.

**Color Mood:** Restrained. Row background `--surface-1` or `--surface-2`. Selected row uses subtle accent tint (~5% of accent saturation). Hover row uses neutral elevation (~2% lighter).

**Typography Mood:** Single scale row. Primary value at `text-sm` medium weight; secondary metadata `text-xs` secondary color. Tabular-nums for any numeric column.

**Key Effects:**
- Type-ahead filter is **above** the list, sticky on scroll. Focuses on `/` keyboard shortcut.
- Arrow keys navigate row selection; `Space` toggles multi-select; `Enter` opens row; `Esc` clears selection
- Sticky batch toolbar appears when any row is selected — slides in from top, 240ms `ease-out-quint`
- Row height fixed for keyboard predictability (36–52px depending on density)
- Virtual scroll past 200 items (rendered window only; total scroll position preserved)
- Empty filter result: explain *why* ("No workflows match 'foo' — try clearing filters")
- Loading state: skeleton rows in exact final height to prevent CLS

**Anti-patterns:**
- Mouse-only interactions (no keyboard nav) → not intelligent, just a list
- Variable row height → keyboard nav unpredictable
- Batch toolbar overlaps content instead of sliding the list down → covers data
- No type-ahead → forces scroll on > 30 items
- Search replaces the list with results instead of filtering in place → loses scroll position
- No empty-filter state → silence reads as broken
- Pagination instead of virtual scroll → breaks "scan a long list" mental model

**Use it for:** Workflow lists, deployment history, user tables, audit logs, project rosters.
**Don't use for:** < 10 items (use a simple list), < 5 items (use chips/cards).

**Canonical references:** Linear's issue list, Vercel's deployments, GitHub's PR list, Notion's database table view.

---

## 6. Empty State Hero

**Pattern:** First-run / no-data state that invites a single primary action, explains the category in one sentence, and provides exactly zero alternative paths.

**Style Priority:** Product. Brand register may invade if the surface is marketing-adjacent (onboarding, signup).

**Color Mood:** Mostly neutral. Optional brand-mark or subtle illustration anchors. Primary action button at full accent. Surrounding chrome quiet.

**Typography Mood:**
- Headline: `text-xl` or `text-2xl`, semibold — one short sentence
- Body: `text-base`, regular — one short paragraph explaining *what lives here*
- CTA: same as primary-button typography
- No subheadings, no bullet lists, no multi-paragraph essays

**Key Effects:**
- Vertical center within available space, with a healthy 40–96px top padding
- No motion on idle — the eye should land on the CTA, not the illustration
- Optional micro-motion on the brand mark or icon (subtle, < 400ms loop, ≤ 1 cycle)
- No card chrome — empty state is its own surface, not a card inside a card
- Mobile: stack vertically, same hierarchy, no horizontal scroll

**Anti-patterns:**
- Multiple competing CTAs → user freezes (Hick's Law)
- "Get started" or "Create your first" with no specifics → empty performance
- Generic stock illustration (`GenericIllustration` slop hit) → reads as filler
- Long-form marketing copy in a product empty state → wrong register
- No primary action → user has to figure out what to do
- "Welcome!" greeting → `MicrocopyDecay` slop hit

**Microcopy template:**
```
No [thing] yet.
[One sentence about what lives here, in plain language.]
[ Create [your first thing] ]
```

Example:
```
No workflows yet.
Workflows are reusable automations triggered by events.
[ Create workflow ]
```

**Use it for:** First-run dashboards, empty filter results (with filter context), zero-state lists.
**Don't use for:** Loading states, error states, anonymous landing pages.

---

## 7. Inline Editor

**Pattern:** Click (or focus) directly on content to edit it in place — no modal, no separate form. The text *is* the input. Used for titles, descriptions, table cells, settings.

**Style Priority:** Product. Inline editing is one of the strongest signals of a high-craft product.

**Color Mood:** Idle: indistinguishable from surrounding text. Hover: ghost border (1px, 6% opacity). Focus: full token border + subtle background lift.

**Typography Mood:** Identical to the displayed value. The user must not perceive a font change between display and edit. Same family, size, weight, line-height.

**Key Effects:**
- Hover: 100ms reveal of subtle 1px ghost border + edit cursor on text-only fields
- Click: surface becomes editable, full focus state appears, ~150ms transition
- Save: `Enter` (single-line) or `⌘+Enter` (multi-line). Save on blur is acceptable for low-stakes fields, never for destructive.
- Cancel: `Esc` reverts to last saved value. Always.
- Save state: brief checkmark or "Saved" pill (1.5s timeout) — only for non-trivial saves
- Optimistic update: value updates immediately; rollback with red ring if save fails
- No "Save" button next to the field unless save-on-blur is destructive

**Anti-patterns:**
- Font / size changes on focus → reads as a glitch
- No `Esc`-to-revert → keyboard users lose work
- Visible save button next to every field → modal-thinking, not inline-thinking
- No optimistic update → typing feels laggy
- No save indicator → user wonders if it worked
- Pencil icon required to enter edit mode → not inline, just delayed-modal

**Use it for:** Titles, descriptions, table cells, settings values, list-item labels.
**Don't use for:** Multi-step forms, destructive actions, anything with validation requiring before-blur feedback.

**Canonical references:** Notion (the benchmark), Linear titles, Airtable cells, Google Docs.

---

## How to Pick

Run the surface through these questions:

1. **One primary task or many?** Many = list/grid. One = focus/hero.
2. **Keyboard-driven or pointer-driven?** Keyboard-heavy = command palette / intelligent list / inline editor.
3. **Display or edit?** Display = bento / live status / hero. Edit = inline editor / form.
4. **Static or live?** Live = status indicator + intelligent list.
5. **Brand or product register?** Brand = bento / hero. Product = everything else.

If two archetypes overlap (e.g., "intelligent list inside focus mode"), pick the dominant one and treat the other as a contained surface inside it.

## When to Invent

You should not. The archetypes above are the load-bearing patterns of modern product UI. If the brief truly does not fit any of them, run the scene-sentence test (see `SKILL.md`) and ask whether the requirement is real or whether the team is reaching for novelty.

The strong answer is almost always to use the right archetype, not to invent a new one. Novelty is a brand register choice, not a product chrome choice.

## Strong Opinion

> Archetypes are not templates — they are signature contracts. Break the contract, and you are no longer doing the archetype. You are just shipping a worse version of it.

## Sources

- Refactoring UI — chapter on patterns and consistency
- Vercel Geist Design System — bento, command palette, deployment status
- Linear Method (linear.app/method) — keyboard-first design philosophy
- Notion's product surfaces — inline editing as benchmark
- IBM Carbon, Material Design 3 — list and toolbar patterns
- `ui-ux-pro-max` (open-design ecosystem) — six-slot pattern schema inspiration
