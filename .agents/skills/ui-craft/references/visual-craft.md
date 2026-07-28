# Visual Craft

Read this when the project has no design system, when the existing system needs extending, or when a visual decision needs defending. The defaults below are humane, opinionated, and battle-tested.

## Typography

Type is the load-bearing system of any UI. Get type wrong and nothing else can save the surface.

### The Scale (default — adjust to brand)

A perfect-fourth (1.333) or major-third (1.250) modular scale at 16px base:

| Token         | Size    | Use                                                    |
| ------------- | ------- | ------------------------------------------------------ |
| `text-xs`     | 11–12px | Captions, metadata, helper text                        |
| `text-sm`     | 13–14px | Secondary text, table cells, form labels in dense UIs  |
| `text-base`   | 16px    | Body text — the default                                |
| `text-md`     | 18px    | Emphasized body, intros                                |
| `text-lg`     | 20px    | Card titles, small section headings                    |
| `text-xl`     | 24px    | Section headings (h3)                                  |
| `text-2xl`    | 30px    | Page subheading (h2)                                   |
| `text-3xl`    | 36px    | Page title (h1) — product UI                           |
| `text-display`| 48–72px | Marketing / hero only, never product chrome            |

**Rules:**
- Base font-size: 16px. Anything smaller for body is a contrast tax on readers.
- Line-height: 1.5 for body, 1.3 for headings, 1.0 for display
- Maximum line length: 60–75 characters for body text. Wider columns hurt readability.
- Use no more than two weights per surface (regular + bold, or medium + bold). Three weights signals a brand system at work.
- Never letter-space body text. Tighten display text by 1–2% only.

### Font Pairing
- One sans-serif system font stack for product UI (`-apple-system, BlinkMacSystemFont, 'Inter', 'Segoe UI', sans-serif`)
- One monospace for code/data (`'JetBrains Mono', 'SF Mono', Menlo, Consolas, monospace`)
- A second sans or serif for marketing voice — never inside the product
- Loading three+ font files is performance and brand confusion combined

### Numeric Tables
- Use tabular figures (`font-variant-numeric: tabular-nums`) in tables and dashboards so digits align column-wise
- Right-align numbers, left-align labels

## Color

Color is the second load-bearing system. It encodes meaning, hierarchy, and brand.

### Semantic Tokens Over Primitive Tokens

**Primitive token (raw):**
```
--gray-50:   #f9fafb;
--gray-900:  #111827;
--blue-500:  #3b82f6;
```

**Semantic token (purposeful):**
```
--color-bg-default:        var(--gray-50);
--color-bg-elevated:       #ffffff;
--color-text-primary:      var(--gray-900);
--color-text-secondary:    var(--gray-600);
--color-border-default:    var(--gray-200);
--color-action-primary:    var(--blue-500);
--color-action-primary-hover: var(--blue-600);
--color-status-success:    var(--green-600);
--color-status-warning:    var(--amber-500);
--color-status-error:      var(--red-600);
--color-status-info:       var(--blue-500);
```

Components reference semantic tokens, never primitives. Dark mode flips the semantic layer; primitives stay the same.

### Palette Anatomy

Every product palette needs:

- **Neutrals** — 9–11 steps from background to text (gray, slate, stone, zinc)
- **Brand** — 1–2 brand hues with 9–11 steps each
- **Status** — success (green), warning (amber), error (red), info (blue) with 5–7 steps each
- **Accent** — optional secondary brand or category hues, 5–7 steps each

Steps should be perceptually uniform — use OKLCH or LAB color space, not naive HSL.

### Dark Mode

- Never just invert. Dark mode requires its own semantic mapping.
- Background is not `#000` — use a near-black (`#0a0a0a`, `#111`). Pure black exaggerates OLED smearing and hurts contrast perception.
- Reduce saturation by 10–20% on hues that would burn the retina at full strength
- Elevation in dark mode is *lighter*, not darker (shadows are still useful but lighter overlays + subtle borders carry most of the work)
- Test every state in both modes. Borders, focus rings, disabled states all need separate verification.

### Color Don'ts

- No raw hex in component code — only tokens
- No color-only state indication (see `accessibility-floor.md` — color always paired)
- No gradient backgrounds carrying the entire visual identity (`GradientCrutch` anti-slop pattern)
- No "rainbow" status systems (10 hues to encode 10 states — chunk and reuse instead)
- No CSS color names (`crimson`, `firebrick`) — they bypass the token system

## Spacing

Spacing is the silent grammar of a layout. Inconsistent spacing reads as inconsistent thinking.

### The Scale

A 4-based scale with 8 as the anchor:

| Token   | Value | Use                                             |
| ------- | ----- | ----------------------------------------------- |
| `space-0` | 0     | None                                            |
| `space-1` | 4px   | Tight clusters (icon + label)                   |
| `space-2` | 8px   | Inter-element default                           |
| `space-3` | 12px  | Slightly more breathing room                    |
| `space-4` | 16px  | Section internal padding                        |
| `space-5` | 20px  | Form field gaps                                 |
| `space-6` | 24px  | Card padding, section gap                       |
| `space-8` | 32px  | Page section gap                                |
| `space-10`| 40px  | Major section gap                               |
| `space-12`| 48px  | Hero / page-level gap                           |
| `space-16`| 64px  | Outer layout breathing                          |

Every gap, margin, padding snaps to this scale. Anything else is a `MagicNumber` anti-slop hit.

### Rhythm

- **Vertical rhythm:** consistent baseline grid. Lines of text + paragraph gaps + heading gaps should be multiples of a base unit (8px works well at 16px body / 24px line).
- **Horizontal rhythm:** indentation, list bullets, and aligned UI elements pull the eye through the page. Break rhythm only intentionally.
- **Whitespace density:**
  - Marketing / brand surfaces: generous (40–80px section gaps)
  - Product chrome: moderate (16–32px)
  - Data-dense surfaces (tables, dashboards): tight (8–16px) — but with rules and zebra striping to compensate

### Density Modes

Some products ship a `compact` and `comfortable` density toggle. If so:
- Each density uses its own spacing tokens (`--space-row-comfortable: 12px; --space-row-compact: 6px;`)
- All affected components reference these tokens, never hard-coded values
- Default to comfortable; compact is for power users with explicit opt-in

## Radius

A radius scale of 4 steps is enough:

| Token         | Value     | Use                                  |
| ------------- | --------- | ------------------------------------ |
| `radius-sm`   | 4px       | Chips, badges, small controls        |
| `radius-md`   | 6–8px     | Buttons, inputs                       |
| `radius-lg`   | 10–12px   | Cards, panels, surfaces              |
| `radius-xl`   | 16–20px   | Hero blocks, large overlays          |
| `radius-full` | 9999px    | Pills, avatars                       |

A single surface should not mix radii arbitrarily (`RandomRadii` anti-slop hit). Pick one per role and apply consistently.

## Elevation

Shadows convey depth and focus. Five steps cover most needs:

| Token       | Use                                                       |
| ----------- | --------------------------------------------------------- |
| `shadow-0`  | Flat — no shadow                                          |
| `shadow-1`  | Hover state on resting cards                              |
| `shadow-2`  | Resting cards on patterned backgrounds, dropdowns         |
| `shadow-3`  | Popovers, comboboxes                                      |
| `shadow-4`  | Modals, side sheets                                       |
| `shadow-5`  | Full-screen dialogs, command palette                      |

**Rules:**
- Shadows are softer (larger blur, lower opacity) at higher elevations, not harder
- Two-layer shadows (one sharp short shadow + one soft long shadow) read more realistically than single-layer
- Dark mode shadows are subtle or replaced by border + lighter overlay

## Motion

Motion communicates causality. Done well, it speeds comprehension. Done badly, it slows the user down.

### Duration

| Use                                           | Duration   |
| --------------------------------------------- | ---------- |
| State change on a small element (hover, focus)| 80–160ms   |
| Subtle reveal (tooltip, popover)              | 120–200ms  |
| Modal / dialog entry                          | 200–280ms  |
| Modal / dialog exit                           | 160–220ms  |
| Page transition                               | 240–360ms  |
| Hero animation                                | 400–600ms  |

Anything longer than 600ms in functional UI feels sluggish.

### Easing

- **`ease-out`** for entries — fast start, slow stop, mimics objects settling
- **`ease-in`** for exits — slow start, fast leave, mimics objects departing
- **`ease-in-out`** for state changes that stay (toggles, smooth transitions)
- **Linear** only for indeterminate loaders and continuous motion (parallax)

### Motion Don'ts

- No bouncy spring physics in product chrome (reserve for delight moments, never for navigation)
- No motion on every page transition — fade is enough in most cases
- No simultaneous motion on multiple unrelated elements
- No motion that violates `prefers-reduced-motion: reduce` (see `accessibility-floor.md`)

### State Transitions

Always animate state changes that change what the user sees:
- Hover/focus: 120ms color + ring transitions
- Open/close: 200ms opacity + transform
- Load complete: 240ms opacity for skeleton-to-content swap

Never animate state changes the user did not initiate (background polling completing should not announce itself with motion).

## Iconography

- One icon family per product. Mixing Lucide + Heroicons + Tabler in one UI is visual incoherence.
- Two sizes max in product chrome (16px and 20px is typical; or 20px and 24px for touch-leaning UIs)
- Stroke icons (consistent weight 1.5–2px) read better in product UI than filled
- Filled variants reserved for "selected" / "active" state
- Icons paired with labels in dense UIs (never icon-only navigation that the user has to learn)
- No emoji as icons (`EmojiSpam` anti-slop hit)

## Imagery

- **Product UI:** photography rarely. When used, branded photography from a real shoot, not generic stock.
- **Marketing:** photography or custom illustration; avoid generic 3D isometric / "people pointing at laptops" stock art (`GenericIllustration` anti-slop hit)
- **Empty states:** prefer text-only or branded symbol over decorative illustration; if illustrative, brand-specific only
- **Avatars:** initials fallback when image is missing; never silhouettes (placeholder ghosting reads as broken)

## Layout

### Grid
- 12-column grid is the default web pattern; works because it divides cleanly (1/2/3/4/6 column splits)
- Outer page gutter scales with breakpoint (16/24/32/48/64)
- Inner content max-width caps long-form reading at ~72ch

### Breakpoints
Anchor on content, not devices:

| Token   | Min-width | Use                                |
| ------- | --------- | ---------------------------------- |
| `xs`    | 0         | Default — mobile-first             |
| `sm`    | 640px     | Tighter than tablet portrait       |
| `md`    | 768px     | Tablet portrait                    |
| `lg`    | 1024px    | Tablet landscape / small desktop   |
| `xl`    | 1280px    | Desktop                            |
| `2xl`   | 1536px    | Wide desktop                       |

Design mobile-first. Always test at 320px width — it is the realistic narrow end.

### Alignment
- Left-align body text by default (in LTR locales)
- Center only for: short headlines, hero blocks, dialog actions, marketing
- Reserve right-alignment for numeric columns

`CenteredEverything` is one of the most common AI-generated UI anti-patterns. Default to left-aligned.

## Defaults When No Token System Exists

If forced to start from scratch, use this conservative default kit. Mark every value as "to be tokenized" in the deliverable.

```css
:root {
  /* Type */
  --font-sans: ui-sans-serif, system-ui, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, Menlo, monospace;
  --text-base: 16px;
  --line-height-body: 1.5;
  --line-height-heading: 1.3;

  /* Neutrals */
  --color-bg: #ffffff;
  --color-bg-elevated: #ffffff;
  --color-bg-subtle: #f8fafc;
  --color-border: #e2e8f0;
  --color-text: #0f172a;
  --color-text-secondary: #475569;
  --color-text-tertiary: #94a3b8;

  /* Brand (placeholder) */
  --color-action: #2563eb;
  --color-action-hover: #1d4ed8;
  --color-action-text: #ffffff;

  /* Status */
  --color-success: #15803d;
  --color-warning: #b45309;
  --color-danger:  #b91c1c;

  /* Spacing */
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-6: 24px;
  --space-8: 32px;

  /* Radius */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;

  /* Motion */
  --duration-fast:   120ms;
  --duration-medium: 200ms;
  --duration-slow:   320ms;
  --ease-out:    cubic-bezier(0.16, 1, 0.3, 1);
  --ease-in:     cubic-bezier(0.4, 0, 1, 1);
  --ease-in-out: cubic-bezier(0.4, 0, 0.2, 1);
}

@media (prefers-color-scheme: dark) {
  :root {
    --color-bg: #0a0a0a;
    --color-bg-elevated: #161616;
    --color-bg-subtle: #1f1f1f;
    --color-border: #2e2e2e;
    --color-text: #f4f4f5;
    --color-text-secondary: #a1a1aa;
    --color-text-tertiary: #71717a;
  }
}
```

This is a scaffold, not a brand. Propose a DESIGN.md as soon as the surface is non-trivial.

## Sources

- Apple Human Interface Guidelines — Typography, Color, Materials
- Material Design 3 — Color (HCT/dynamic color), Motion (physics), Shape
- IBM Carbon Design System — Spacing, Type, Color tokens
- Microsoft Fluent 2 — Motion, Depth, Color
- Lawsofux.com — Aesthetic-Usability Effect, Cognitive Load
