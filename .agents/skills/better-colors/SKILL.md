---
name: better-colors
description: OKLCH color space for web projects. Convert hex/rgb/hsl to oklch, generate palettes, check contrast, handle gamut boundaries, and theme with Tailwind v4. Triggers on oklch, color conversion, palette generation, contrast ratio, gamut, display p3, design tokens, hue drift, chroma, dark mode colors.
---

# OKLCH Colors

OKLCH is a perceptually uniform color space where the numbers actually mean what you think they mean. Most color problems in CSS (broken palettes, failing contrast, hue drift) come from using color spaces that don't match how we see. OKLCH fixes the model so the tools work. To explore interactively, visit [oklch.fyi](https://oklch.fyi).

## Quick Reference

| Category | When to use | Reference |
| --- | --- | --- |
| Conversion | Hex/rgb/hsl to oklch | [color-conversion.md](color-conversion.md) |
| Palettes | Generate scales, multi-hue, dark mode | [palette-generation.md](palette-generation.md) |
| Contrast | APCA/WCAG checks, fixing failing contrast | [accessibility-contrast.md](accessibility-contrast.md) |
| Gamut & Tailwind | P3 fallbacks, `@theme` scales, gamut clamping | [gamut-and-tailwind.md](gamut-and-tailwind.md) |

## Why OKLCH

- **Perceptual uniformity.** Equal L steps = equal brightness. `oklch(0.5 ...)` is visually mid. HSL's `lightness: 50%` varies wildly by hue.
- **Stable hue.** HSL blue shifts toward purple as lightness changes. OKLCH hue stays constant across the full lightness range.
- **Independent chroma.** Chroma is an absolute measure of colorfulness that doesn't depend on lightness. HSL saturation does.
- **Finite gamut.** Not every oklch value maps to a displayable sRGB color. High-chroma values at certain hues will clip; gamut awareness is required.

## OKLCH Syntax

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

**Formatting:** L and C use 3 decimal places, H uses up to 3. Drop trailing zeros. Format `-0` as `0`. Browser support: Baseline 2023, 96%+ global coverage.

## Key Thresholds

| Rule | Value |
| --- | --- |
| Light/dark boundary | L > 0.6 = light background → use dark text |
| Lightness gap (light bg) | Foreground L < 0.35 when background L > 0.9 |
| Lightness gap (dark bg) | Foreground L > 0.9 when background L < 0.25 |
| Hue drift threshold | > 10° spread across palette steps = visible drift |
| APCA body text | \|Lc\| >= 75 minimum, >= 90 preferred |
| APCA non-body text | \|Lc\| >= 60 minimum |
| WCAG 2 normal text | 4.5:1 AA, 7:1 AAA |
| Contrast fix | Adjust L only; chroma has negligible effect |

## Review Output Format

Always present color changes as a markdown table with **Before** and **After** columns. Include **every color that was changed**, not just a subset. Never list findings as separate "Before:" / "After:" lines outside of a table.

| Before | After |
| --- | --- |
| `color: #3b82f6` | `color: oklch(0.623 0.188 259.815)` |
| Same absolute C across hues | Same C% of each hue's max chroma |
| No sRGB fallback for P3 color | `@media (color-gamut: p3)` wrapper |

This keeps feedback scannable and diff-friendly. Each row is a self-contained change the developer can act on independently.

## Common Mistakes

| Issue | Fix |
| --- | --- |
| Hex/rgb/hsl color in new code | Convert to `oklch()` |
| HSL palette ramp with hue drift | Rebuild with constant oklch hue |
| Failing contrast (check foreground vs its background using APCA) | Adjust oklch L channel, keep C and H |
| High chroma without gamut check | Clamp to max chroma for the L/H in sRGB |
| Same absolute C across different hues | Use same C% (percentage of max) for consistent vividness |
| P3 color without sRGB fallback | Add `@media (color-gamut: p3)` pattern |
| Dark mode with hand-picked colors | Derive from light palette by reversing L mapping |
| Hex in Tailwind v4 `@theme` | Convert to oklch values |
| Alpha with comma syntax | Use slash: `oklch(L C H / alpha)` |

## Reference Files

- [color-conversion.md](color-conversion.md): Supported formats, conversion examples, bulk conversion rules, what to leave alone
- [palette-generation.md](palette-generation.md): Scale convention, generation algorithm, multi-hue palettes, dark mode, why not HSL
- [accessibility-contrast.md](accessibility-contrast.md): APCA and WCAG 2 thresholds, fixing contrast with L, lightness gap guide, hue drift detection
- [gamut-and-tailwind.md](gamut-and-tailwind.md): sRGB vs P3, gamut clamping, CSS fallback patterns, Tailwind v4 @theme and migration
