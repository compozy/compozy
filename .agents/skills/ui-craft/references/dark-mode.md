# Dark Mode

Read this when implementing or reviewing a dark theme, when adding new tokens that need a dark counterpart, or when "the dark mode looks worse" is a real review comment. Most dark modes fail the same way: they invert the palette, keep light-mode shadows, leave focus states invisible, and use the same saturated accents that vibrate against a dark canvas.

Dark mode is not light mode flipped. It is a different scene.

## The Scene Test (Again)

Before touching tokens, write the scene sentence (see `SKILL.md` HARD-GATE). Dark mode lives or dies on this answer.

- ❌ "We need dark mode because users expect it."
- ✅ "SRE glances at incident severity on a 27" monitor at 2am in a dim room — every UI element competes with a paged alert sound."
- ✅ "Code review at 11pm on a personal laptop in bed; brightness is already low, eyes are tired."

If the scene does not force the answer, the request is preference, not need. Ship light mode, plus a system-following toggle. Do not over-design.

## Surface Lightness for Depth (Not Shadows)

The single biggest mistake: porting light-mode shadows to dark mode. In dark mode, shadows have nothing to cast against — they read as smudges.

**Light mode:** elevation = darker shadow.
**Dark mode:** elevation = *lighter* surface. Up = brighter.

Build a 4-surface scale:

```css
:root {
  /* Light mode */
  --surface-1: #ffffff;   /* page */
  --surface-2: #f8fafc;   /* subtle (sidebars, list rows) */
  --surface-3: #ffffff;   /* elevated (cards, popovers) */
  --surface-4: #ffffff;   /* highest (modals, command palette) */
}

[data-theme="dark"] {
  --surface-1: #0a0a0a;   /* page (near-black, never pure #000) */
  --surface-2: #141414;   /* +6% lightness */
  --surface-3: #1d1d1d;   /* +10% */
  --surface-4: #262626;   /* +15% — highest elevation */
}
```

Combine surface-lightness with a hairline 1px top border or subtle inner glow to reinforce elevation without shadows.

```css
.card {
  background: var(--surface-3);
  border-top: 1px solid color-mix(in oklch, white 6%, transparent);
}
```

Shadows still have a role — but soft, low-opacity, used to anchor floating elements (popover, tooltip), not to lift everything.

## Why Not Pure Black

`#000` is OLED-friendly on phones but a UX trap on desktop monitors:

- Pure black + bright text causes halation (blurry edges) for some readers
- Smear / black smearing on OLED panels during motion
- High contrast tax: AAA contrast against pure black is harder to read than AA against `#0a0a0a`
- No room for "lower than surface-1" — you can never go darker

Use `#0a0a0a` to `#121212` for `--surface-1`. Material Design 3 recommends `#121212`; Apple's HIG dark is around `#1c1c1e`. Both are correct for different reasons.

## Text Adjustments

Light text on dark needs slight typographic compensation. Without it, text looks thinner and tighter than its light-mode twin.

| Adjustment        | Direction         | Why                                                |
| ----------------- | ----------------- | -------------------------------------------------- |
| `line-height`     | +0.05 to +0.1     | Tighter baseline reads cramped on dark             |
| `letter-spacing`  | +0.01 to +0.02em  | Compensates for the optical "thickening" of light text |
| `font-weight`     | +50 to +100       | Thin weights (200, 300) often need a bump on dark  |
| Pure white text   | Avoid `#fff`      | Use `oklch(98% 0 0)` (`#fafafa`) or warmer        |

```css
[data-theme="dark"] body {
  font-weight: 400;
  line-height: 1.55;     /* up from 1.5 */
  letter-spacing: 0.005em;
}
```

Most teams ship dark mode at the same body-text settings as light. It is correct, but not great. Compensate.

## Accent Desaturation

Saturated brand colors that look great on a white canvas vibrate against dark. Reduce chroma 10–20% in OKLCH:

```css
:root {
  --color-action: oklch(60% 0.20 250);          /* light mode — vivid blue */
}
[data-theme="dark"] {
  --color-action: oklch(70% 0.16 250);          /* dark mode — lifted, less chroma */
}
```

Rules:
- **Lift lightness 5–10 points** so the accent stays visible against the dark surface
- **Drop chroma 15–25%** so the color does not vibrate
- **Keep the same hue.** Brand identity must survive the swap. Never re-hue the accent for dark mode.

The same applies to status colors:
- Red: `oklch(55% 0.20 25)` → `oklch(65% 0.15 25)` — preserves "danger" but tames vibration
- Green: `oklch(55% 0.17 150)` → `oklch(70% 0.13 150)` — readable success on dark
- Amber: `oklch(75% 0.15 80)` → `oklch(80% 0.12 80)` — warning legibility

Cite: this is core color-theory hygiene popularized by [Andrey Sitnik](https://evilmartians.com/chronicles/oklch-in-css-why-quit-rgb-hsl) and [Steve Schoger](https://refactoringui.com) — light mode and dark mode are different perceptual environments, not the same image with inverted brightness.

## Semantic Token Strategy

Primitives are frozen. Only semantic tokens are redefined per theme. This is the only architecture that does not rot.

```css
/* Layer 1: primitives — same in both themes */
:root {
  --blue-100: oklch(95% 0.04 250);
  --blue-300: oklch(80% 0.12 250);
  --blue-500: oklch(60% 0.20 250);
  --blue-700: oklch(45% 0.18 250);

  --gray-50:  oklch(98% 0 0);
  --gray-100: oklch(95% 0 0);
  --gray-900: oklch(20% 0 0);
}

/* Layer 2: semantic — light theme */
:root {
  --color-bg-page:       var(--gray-50);
  --color-bg-elevated:   #ffffff;
  --color-text-primary:  var(--gray-900);
  --color-action:        var(--blue-500);
}

/* Layer 2: semantic — dark theme */
[data-theme="dark"] {
  --color-bg-page:       oklch(15% 0 0);
  --color-bg-elevated:   oklch(20% 0 0);
  --color-text-primary:  oklch(95% 0 0);
  --color-action:        var(--blue-300);
}

/* Layer 3: components — reference semantic only */
.btn-primary {
  background: var(--color-action);
  color: var(--color-bg-elevated);
}
```

Components never reference primitives directly. If a component needs `var(--blue-500)`, the design system is leaking primitives — file a token. This is the same architecture covered in `design-system-integration.md`; dark mode is the test that proves it works.

## Focus Visibility — The Killer Detail

The single most common dark-mode regression: focus rings that disappear against the dark canvas.

```css
:root {
  --color-focus-ring: oklch(65% 0.18 250);
}
[data-theme="dark"] {
  --color-focus-ring: oklch(75% 0.18 250);  /* brighter, same hue */
}

:focus-visible {
  outline: 2px solid var(--color-focus-ring);
  outline-offset: 2px;
}
```

Test on every interactive element in both themes. Tab through. If you cannot find focus, it is a `GhostFocus` anti-slop hit (Critical) — see `ai-slop-patterns.md`.

## Dangerous Combinations

These read fine in light mode and fail in dark.

| Pair                                  | Failure mode                                              | Fix                                        |
| ------------------------------------- | --------------------------------------------------------- | ------------------------------------------ |
| Pure red text on dark                 | Red/black vibration; pixel shimmer                        | Lift red lightness; pair with icon         |
| Pure blue text on dark                | Edges glow; reads fuzzy                                   | Use cyan-shifted blue or lighter shade     |
| Gradient text on dark                 | Contrast unpredictable across the gradient                | Solid color only                           |
| Shadow as sole elevation cue          | Looks like a smudge, not depth                            | Surface lightness + hairline border        |
| Light-mode primary at full saturation | Vibrates; halation on text                                | Reduce chroma 15–25%                       |
| White borders (`#fff`)                | Too loud; reads as separator + accent simultaneously       | Use 6–10% white-mixed neutrals              |
| Disabled = `opacity: 0.5`             | Loses too much contrast; unreadable                       | Explicit disabled tokens with ≥ 3:1 contrast |

## Images, Icons, Illustrations

Assets need theme awareness — they cannot just "be."

- **Photos:** light-mode hero photos look harsh in dark mode. If imagery is core, provide a dark-mode crop or apply a subtle overlay (`bg-blend-mode: multiply` with `--surface-1`).
- **Icons:** stroke-based icons port cleanly. Filled icons may need a slight lightness lift. Currentcolor-based SVG (`fill="currentColor"`) inherits text color automatically.
- **Illustrations:** custom illustrations need dark variants. Generic stock looks worse in dark mode than light, because the colors clash with the surface.
- **Code blocks:** syntax-highlight themes must pair. Ship a `prism-dark` / `shiki-dark` alongside the light theme; never use one theme for both.
- **Logos:** brand mark may need a light-mode and dark-mode variant. Test on `--surface-3` (elevated card), not just on `--surface-1`.

## System vs Manual Theme

- Respect `prefers-color-scheme: dark` as the **default**.
- Provide an explicit toggle: System / Light / Dark.
- Persist the user's manual choice in `localStorage`; do not re-prompt.
- Avoid flash-of-wrong-theme: read the persisted theme synchronously before paint, ideally in a blocking inline script in `<head>`.

```html
<script>
  // In <head>, before any stylesheet
  const stored = localStorage.getItem("theme");
  const system = matchMedia("(prefers-color-scheme: dark)").matches;
  const theme = stored === "system" || !stored ? (system ? "dark" : "light") : stored;
  document.documentElement.dataset.theme = theme;
</script>
```

## Testing Checklist

Run on every state, in every theme.

- [ ] Body text contrast ≥ 4.5:1 against `--surface-1`, `--surface-2`, `--surface-3`
- [ ] Large text and UI components ≥ 3:1
- [ ] Focus ring visible and ≥ 3:1 against every surface
- [ ] All state colors (success, warning, error, info) pass contrast at body and icon sizes
- [ ] No shadow doing depth work alone — surface lightness + border carries it
- [ ] No `#fff` text — use `oklch(95-98% 0 0)`
- [ ] No `#000` page background — use `oklch(15-20% 0 0)`
- [ ] No pure-saturation accents — chroma reduced
- [ ] Disabled state has explicit token, ≥ 3:1 contrast for affordance
- [ ] Skeleton placeholders visible on dark surfaces (often invisible if ported as-is)
- [ ] No flash-of-light-theme on initial load
- [ ] All images / illustrations evaluated; dark variants where needed

## Figma → Code

If the design lives in Figma, use variant modes — not duplicated files — for light/dark. Then:

- Use the Figma MCP (`figma:implement-design`) to pull both modes in one pass — it understands variant tokens and exports them as themed CSS variables
- Use Code Connect (`figma:code-connect-components`) to map themed components so dark/light is one component with mode props, not two implementations

This is mentioned in `design-system-integration.md`; dark mode is the canonical test of whether the Figma-to-code pipeline is correctly themed.

## Common AI Failures

- `filter: invert(1)` to "make it dark" — destroys imagery, photos, and gradients alike
- Keeping all light-mode shadows in dark mode
- Forgetting to add a dark variant for the focus ring
- Reducing text to `opacity: 0.6` instead of using a token
- Same saturated accent in both themes, causing halation
- Letting raw `#000` slip into `--bg`
- Dark mode that only changes background and text, ignoring borders, badges, status pills, skeletons

## Strong Opinion

> Light mode flipped is not dark mode. It is a glitch with a switch.

## Sources

- Material Design 3 — *Dark theme*, surface elevation overlays
- Apple HIG — *Dark Mode* (system palette, dynamic colors)
- Refactoring UI — *Designing With Color* (saturation reduction principle)
- Evil Martians — *OKLCH in CSS: why we quit RGB/HSL* (Andrey Sitnik)
- IBM Carbon Design System — *Dark themes* documentation
- WCAG 2.2 — color contrast requirements unchanged across themes
