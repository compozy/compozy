# Accessibility and validation

Use for text/control-bearing glass, uncontrolled media, application preferences and release checks. Keep the project's existing accessibility target. The references below distinguish normative WCAG requirements from practical design recommendations.

## Contrast belongs to the final composite

No blur radius or alpha value guarantees readability. Blur removes detail; a uniform white backdrop remains white after blur. A surface that passes over one image may fail after scrolling or a tenant uploads a different cover.

- Ordinary text needs **4.5:1** under WCAG 2.2 SC 1.4.3. Large text needs **3:1** under that same criterion: at least 18pt normal or 14pt bold, approximately 24px/18.67px in CSS.
- SC 1.4.11 requires **3:1** for visual information needed to identify controls and states, and meaningful graphical objects, against adjacent colors. It does not require every decorative card rim to reach 3:1. If the faint boundary is the only way to identify an input, it becomes functional.
- Test placeholders, selected states, hover/focus content, important icons and charts. Inactive controls and purely decorative content have specific exceptions; do not treat needed secondary text as decoration.
- Focus must be visible and not completely hidden by author content at AA. SC 2.4.13's quantified Focus Appearance requirement is AAA. A 2–3px opaque outline with offset is a useful starting treatment, not automatic compliance over mixed backdrops.

See [Contrast Minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html), [Non-text Contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html), [Focus Appearance](https://www.w3.org/WAI/WCAG22/Understanding/focus-appearance.html), and [Focus Not Obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html).

### Calculation and rendered inspection have different jobs

For a simple fill composited over an opaque background, channel compositing is `C = alpha × fill + (1 − alpha) × backdrop` in the compositing color space. Convert to WCAG relative luminance, then use `(Llighter + .05) / (Ldarker + .05)`. This is not a simulator for blend modes, nested layers, wide gamut, refraction or native vibrancy. The lab's "contrast estimate" uses exactly this formula with the median and the bright-tail luminance of each backdrop measured from renders; it is a guide for tuning, not a measurement.

Calculated counterexample: white text over a 35% black fill on a white backdrop gives about **2.442:1**; 55% black gives about **4.759:1**. A generic 20–35% scrim cannot certify ordinary white text. See [W3C compositing](https://www.w3.org/TR/compositing-1/#simplealphacompositing).

**Measured counterexample and fix (2026-09-10 lab runs).** With the text made transparent and the region behind the widget title screenshotted in Chromium, then compared with the ink: white ink on a `255 255 255 / .16` tint over the pastel sunset backdrop measured **1.70:1** at the median and **1.40:1** at the worst 5th percentile; a `.34` frost made it worse (1.49:1) because the white fill lifts the composite toward the ink. Switching that scene to a smoked neutral tint (`56 50 58 / .66`, white ink) raised the title to **5.70:1** and the 90%-alpha secondary text to **5.23:1**; the pale field switched to white frost with dark ink measures **14.5:1** (secondary 4.8:1); the deep gradient keeps the light tint at **7.58:1**. Secondary text at 62–74% alpha failed on every backdrop; 85–90% clears 4.5:1 on the adapted modes. The ink must follow the backdrop; a favorite look does not override the measurement.

Apple's own course correction is the corpus's strongest evidence here (`wiki/concepts/Text Contrast on Translucent Surfaces.md`): after the first iOS 26 beta it raised the opacity of navigation bars and system overlays, added a tinted mode in 26.1 and reduced default transparency at WWDC 2026 with a user-facing slider. Adaptive tint narrows the distribution of effective contrast; it never bounds it. A deterministic scrim, plate or solid control plane does.

The corpus's contrast article (`wiki/concepts/Text Contrast on Translucent Surfaces.md`) reaches the same numbers from published recipes: a generator preset at 12% white fill, 12px blur and 160% saturation reports **FAIL 1.4:1**, the exact worst-percentile figure the lab measured; "a 12% white fill over a user photo can swing from 8:1 to under 2:1 as content scrolls behind it"; body-text surfaces need `.55–.80` fill while chrome and decoration live at `.10–.30`; white hairlines at `.25` (light) or `.12` (dark) are "a plausible 3:1 failure, measure it per surface"; focus rings should be solid, 2px inner plus 4px outer. Apple's 35% dimming layer for clear glass is the only vendor number and is native-context advice.

For actual glass:

1. Identify text/icon/focus pairs and distinguish functional edges from optical decoration.
2. Exercise the permitted backdrop range with the stress board: calm, textured, hotspot, shape conflict, dark with accents, plus actual underlying UI and scroll positions, in both themes and interactive states.
3. Render, hide the text (`color: transparent` on the subtree, nothing else moved), capture the region behind the letters, and take luminance percentiles rather than the panel average. Include grain, sheen and other overlays in the capture.
4. Compare against the intended ink before antialiasing; inspect the real glyphs for thin strokes and halos.
5. Record the weakest pair and scenario. A screenshot set is sampled evidence, not a guarantee for arbitrary future media; for unrestricted backdrops, bound the composite with a plate or a solid control plane.

When a check fails, change the ink pairing (white → dark ink on white frost, or white tint → smoked neutral tint), add a `.glass-plate` under the text, or bound the backdrop. A deliberate contrast halo can help under [W3C G18](https://www.w3.org/WAI/WCAG22/Techniques/general/G18); a diffuse decorative text shadow is not sufficient. Do not raise blur and re-shoot the same favorable frame.

## Preferences and fallback behavior

Use the solid-first cascade in [Web engineering](web-engineering.md#implementation-contract). Explicitly handle every filtered/decorative layer owned by the material, not just its outer wrapper.

| State | Required material behavior for this recipe | Verification |
|---|---|---|
| Filter unsupported/disabled | Opaque paired fill and ink; useful border | Remove the enhancement and inspect real content |
| Manual reduced transparency | Solid fill; filters/refraction and obstructive decoration off | Toggle in each theme and material variant |
| `prefers-reduced-transparency: reduce` | Respect preference with the same solid material | Media emulation plus actual OS/browser when part of target matrix |
| `forced-colors: active` | Opaque system-color surface; filters/shadows/noise off; visible boundaries/focus/states | Inspect palette and keyboard states; preserve `forced-color-adjust: auto` |
| Reduced motion | Stop ornamental parallax, tilt, lens movement and moving backgrounds; keep functional final states | Use keyboard/pointer and observe transitions |
| Increased contrast | Strengthen meaningful pairs/boundaries; optionally select solid material | Treat solid mapping as a product choice, not a universal media-query rule |
| Print, when relevant | Readable opaque page with no optical effects or overlapping fixed chrome | Print preview, including backgrounds disabled |

**Forced colors preserves background alpha when substituting colors.** It does not universally remove translucency or backdrop filters. Non-URL background images and shadows are suppressed by the browser, but URL noise images can survive. Explicitly use `Canvas`/`CanvasText` or the relevant system colors, and disable decoration. Do not opt the whole interface out to preserve branding. See [CSS Color Adjustment](https://www.w3.org/TR/css-color-adjust-1/#forced-colors-mode) and [MDN forced-colors](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@media/forced-colors).

Reduced-transparency support remains uneven; refresh the intended target versions using [MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@media/prefers-reduced-transparency) and its compatibility data. A manual solid option is especially useful when glass is pervasive. Normal mode still needs to be legible; the setting is not a substitute for that contract. Reduced motion and reduced transparency are separate preferences. `prefers-contrast: custom` does not necessarily mean increased contrast.

## Enterprise and interaction cases

These are engineering/design deductions from the surface and accessibility principles, not empirical glass usability studies. Reuse existing accessible primitives and their canonical tests.

| Case | Material allocation | Behavior to preserve |
|---|---|---|
| Command palette | Glass shell; solid search field/results and strong selection | Existing dialog/combobox behavior, focus transfer/return, keyboard selection and Escape |
| Sticky table header | Solid labels/header; optional glass outer toolbar | Stable column/sort relationships while rows pass beneath; focus not hidden by the bar |
| Filter chips above a grid | Solid chips inside one material plane | Toggle/removal semantics, readable selection beyond color, no nested buttons |
| Inline row toolbar | Stable solid controls | Reveal on keyboard focus as well as hover; usable touch path; active value remains visible |
| Split-pane resizer | Functional grip/state with stable contrast | Existing keyboard resizing, accessible name/value/orientation, adequate target area |
| Form or critical confirmation | Solid inputs, messages and primary action; decorative glass outside | Labels, errors, validation and disabled/loading state remain understandable |

Prefer native HTML tables for tabular content; a CSS grid is not automatically an ARIA grid. [APG dialogs](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/) and [comboboxes](https://www.w3.org/WAI/ARIA/apg/patterns/combobox/) explain interaction contracts, not a reason to rebuild an installed primitive. Material alone supplies none of those semantics.

## Reflow, localization and practical validation

Test text resize to 200% and reflow at an equivalent 320 CSS px width. A 1280px layout at 400% zoom is a useful reflow scenario. Check sticky bars, nested scrolling, dialog height, long translations, focus-ring clipping, and tabbing in both directions. Keep necessary two-dimensional table scrolling local; it does not excuse overflow in the surrounding app. The AA target-size criterion is 24×24 CSS px with exceptions such as sufficient spacing, not an unconditional 44px rule; a larger target is often preferable for touch. See [WCAG 2.2](https://www.w3.org/TR/WCAG22/).

For RTL, use logical layout properties and actual `dir` semantics; isolate unpredictable bidirectional user content when needed. Do not automatically mirror physical highlights just because text direction changed. Check directional icons and focus order in the real localized view.

Scope validation to what changed. An automated contrast checker may not understand the final filtered backdrop. Combine relevant component tests, rendered inspection, keyboard interaction and measured pairs; add assistive-technology and real-device runs when required by the product target. Browser emulation verifies the selected CSS branch, not the user's physical OS preference integration. Report untested engines, devices and native APIs explicitly rather than calling the design “fully accessible.”
