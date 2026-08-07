---
name: better-colors
description: OKLCH color space and color usage for web projects. Convert hex/rgb/hsl to oklch, generate palettes, check contrast, handle gamut boundaries, theme with Tailwind v4, and apply color with meaning. Triggers on oklch, color conversion, palette generation, contrast ratio, gamut, display p3, design tokens, semantic color tokens, hue drift, chroma, dark mode colors, accent color, color meaning, light and dark appearance, increased contrast.
---

# OKLCH Colors

OKLCH is a perceptually uniform color space where lightness, chroma, and hue are useful design controls. Use it when the project already uses OKLCH, when creating a new color system, or when the user asks for conversion or palette work. Otherwise preserve the project's established tokens and notation: a consistent hex or RGB token system is better than introducing a second color representation for an isolated fix. To explore interactively, visit [oklch.fyi](https://oklch.fyi).

## Quick Reference

| Category | When to use | Reference |
| --- | --- | --- |
| Conversion | Hex/rgb/hsl to oklch | [color-conversion.md](color-conversion.md) |
| Palettes | Generate scales, multi-hue, dark mode | [palette-generation.md](palette-generation.md) |
| Contrast | APCA/WCAG checks, reporting failures, fixing on request | [accessibility-contrast.md](accessibility-contrast.md) |
| Gamut & Tailwind | P3 fallbacks, `@theme` scales, gamut clamping | [gamut-and-tailwind.md](gamut-and-tailwind.md) |
| Usage | Semantic tokens, one meaning per color, primary-action emphasis, appearance variants | [color-usage.md](color-usage.md) |

## Core Principles

### 1. Use a Perceptual Color Space

- **Respect the existing system.** Do not convert notation merely because this skill was loaded. Reuse the project's semantic tokens and authoring format unless the task includes a color-system migration.
- **Perceptual uniformity.** Equal L steps = equal brightness. `oklch(0.5 ...)` is visually mid. HSL's `lightness: 50%` varies wildly by hue.
- **Stable hue.** HSL blue shifts toward purple as lightness changes. OKLCH hue stays constant across the full lightness range.
- **Independent chroma.** Chroma is an absolute measure of colorfulness that doesn't depend on lightness. HSL saturation does.
- **Finite gamut.** Not every oklch value maps to a displayable sRGB color. High-chroma values at certain hues will clip; gamut awareness is required.

### 2. Write and Format OKLCH Consistently

```
oklch(L C H)
oklch(L C H / alpha)
```

| Channel | Range | Description |
| --- | --- | --- |
| L (Lightness) | 0–1 | 0 = black, 1 = white. Perceptually uniform. |
| C (Chroma) | 0–~0.4 | Colorfulness. 0 = gray. Max depends on L and H. |
| H (Hue) | 0–360 | Hue angle in degrees. |
| alpha | 0–1 | Optional transparency. Slash syntax. |

```css
oklch(0.637 0.237 25.331)
oklch(0.8 0.05 200 / 0.5)
```

Use three decimal places for L and C and up to three for H. Drop trailing zeros and format `-0` as `0`. OKLCH is Baseline 2023; when support requirements are unusually broad, check the target project's browser matrix instead of relying on a fixed global-coverage percentage.

### 3. Measure Contrast, Gamut, and Palette Behavior

| Rule | Value |
| --- | --- |
| Light/dark boundary | L > 0.73 = light background → dark text; below it, light text still scores higher |
| Lightness gap (light bg) | Foreground L < 0.35 when background L > 0.9 |
| Lightness gap (dark bg) | Foreground L > 0.9 when background L < 0.25 |
| Hue drift threshold | > 10° spread across palette steps = visible drift |
| APCA body text | \|Lc\| >= 75 minimum, >= 90 preferred |
| APCA non-body text | \|Lc\| >= 60 minimum |
| WCAG 2 normal text | 4.5:1 AA, 7:1 AAA |
| Contrast fix (only when asked) | Adjust L first; preserve C and H when possible, then remeasure the rendered pair |

## Common Mistakes

| Issue | Fix |
| --- | --- |
| Raw color bypasses the project's semantic token system | Reuse or add the correct role token in the project's existing notation |
| Isolated OKLCH value introduced into a hex/RGB codebase | Preserve the established notation unless the task includes a color-system migration |
| HSL palette ramp with hue drift | Rebuild with constant oklch hue |
| Failing contrast (check foreground vs its background using APCA) | Report the pair, its measured Lc and the threshold it misses; change colors only when asked (then adjust L, keep C and H) |
| High chroma without gamut check | Clamp to max chroma for the L/H in sRGB |
| Same absolute C across different hues | Use same C% (percentage of max) for consistent vividness |
| P3 color without sRGB fallback | Add `@media (color-gamut: p3)` pattern |
| Dark mode created by mechanically reversing the light palette | Use the light palette as a starting point, then tune chroma and lightness and recheck every foreground/background pair |
| Hex in Tailwind v4 `@theme` | Convert to oklch values |
| Alpha with comma syntax | Use slash: `oklch(L C H / alpha)` |
| Same hue means two different things (link color reused decoratively) | One color, one meaning; give the second use a neutral |
| Semantic token used outside its role (separator as text) | Add a token for the missing role; never borrow by value |
| Several colored control backgrounds in one view | Fill only the single primary action; secondaries stay neutral |
| Palette verified only in light mode | Recheck every foreground/background pair in both appearances |

## Review Output Format

Use this format only when the user asks for a standalone color review. When `better-interface` orchestrates the review, provide domain evidence and findings to that skill and let its output format, severity scale, consolidation rules, cap, and verdict take precedence.

Present the standalone review in two parts.

### Findings

Group all confirmed findings by principle. Use a markdown table with **Severity**, **Location**, **Before**, **After**, and **Why** columns. Never use separate "Before:" / "After:" lines.

- **Severity**: `HIGH` makes content unreadable or assigns a misleading semantic color; `MEDIUM` creates a noticeable theme, gamut, or consistency failure; `LOW` is isolated polish.
- **Location**: cite `path/to/file:line`. If the artifact has no source files, cite the exact screen and component instead.
- **Before / After**: show the current value or token and the exact replacement.
- **Why**: name the violated principle and include measured contrast or gamut evidence when relevant.

Consolidate a repeated systemic issue into one row and list every affected location. Omit principles with no findings.

| Severity | Location | Before | After | Why |
| --- | --- | --- | --- | --- |
| MEDIUM | `src/theme.css:18` | `color: #3b82f6` | `color: oklch(0.623 0.188 259.815)` | New project colors use OKLCH tokens |
| MEDIUM | `src/palette.ts:31` | Same absolute C across hues | Same C% of each hue's maximum chroma | Equal chroma values do not appear equally vivid across hues |
| HIGH | `src/theme.css:52` | P3 color with no fallback | Add an sRGB fallback before `@media (color-gamut: p3)` | The color fails on non-P3 displays |

### Verification and Verdict

After the findings:

1. **Verification**: list the exact checks run and their observed results, including contrast measurements, gamut checks, and both light and dark appearances when applicable. If a check was not run, state what still needs verification.
2. **Verdict**: `Block` if any `HIGH` finding remains, `Needs changes` if only `MEDIUM` or `LOW` findings remain, and `Approve` only when no actionable findings remain.

When there are no findings, omit the table, state "No actionable color findings", report verification, and end with `Approve`.
