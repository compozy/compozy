# Authoring validation

Validation dates: 2026-09-10 (initial package) and 2026-09-10 (redesign against the reference board). This records how the package was checked; it is not a mandatory report template for later uses of the skill.

## Package and instruction review

- The `writing-skills` metadata validator accepts the name and description after the redesign.
- Every reference is self-contained; `SKILL.md` routes to one branch per need and to the new [reference board](reference-board.md). No approval step, mandatory report or subagent round is introduced.
- Negative triggers (ordinary form alignment, photorealistic drinking-glass renders) stay excluded by the description.

## Redesign inputs

- 22 reference renders selected by the author from Savee (weather widgets by Andrei Rybin, Apple Liquid Glass lock screen, WWDC-style controls, capsule buttons, lens buttons, frosted light bars, material studies); see the board.
- Twelve external technical sources retrieved on the redesign date (WebTricks, kube.io, LogRocket, dev.to, Yvaine, Setproduct, Aave, and the `dpawlikowski`, `samasante`, `rizroze`, `glassfx`, `iart-ai` repositories) for the rim/sheen stack, displacement-map construction, chromatic passes, filter-region traps and cross-browser strategies. Their claims were checked against the built lab, not taken as specifications.
- The workspace corpus (`research/glassmorphism/`, 237 sources) remains the base for decision rules, contrast and performance guidance.

## Real browser checks of the redesigned lab

Executed against the local HTML/CSS with Playwright 1.63 on Chromium headless shell 153, Google Chrome headless (macOS 26.6, software rendering) and Playwright WebKit 26.6. Screenshots were reviewed for every backdrop at 1440px and at 390px through a same-size iframe. The harness is bundled as [scripts/validate-lab.mjs](../scripts/validate-lab.mjs).

| Case | Observed result |
|---|---|
| Console | No errors or warnings on load or during control changes |
| Three thickness presets | Computed `backdrop-filter`: clear `blur(6px) saturate(1.7) brightness(1.08)` fill `.07`; regular `blur(14px) saturate(1.6) brightness(1.05)` fill `.16`; frosted `blur(26px) saturate(1.4) brightness(1.05)` fill `.34`; white ink in all three |
| Refraction | Lensed controls carry `url("#lens-n")` ahead of the blur chain; over the poppy and stress backdrops the color edge bends inside the rim and stays straight in the center; cards carry no lens after the selector fix |
| Ink modes | `data-ink` light/dim/dark switch tint, alpha, rim colors, ink and focus color per scene; the desk switches to a smoked tint on light-ink scenes |
| Manual solid mode | Fill `rgb(59 63 75)`, ink white, `backdrop-filter: none`, rim/sheen pseudo-elements removed |
| `prefers-reduced-transparency` (CDP emulation) | Same solid material; status acknowledges the preference; lensing suspended |
| `forced-colors: active` | Fill `Canvas` (white), ink `CanvasText`, 1px system border, filters none, decorative backdrop hidden |
| `prefers-reduced-motion` | Transitions 0s; glass stays translucent (motion and transparency preferences are distinct) |
| Print | White fill, black ink, no filters; desk, media and dock hidden |
| Keyboard | 36 focusable controls; 36 of 36 Tab stops show a ≥2px focus outline; Enter toggles, Space flips the switch, arrow keys move the slider |
| Targets | No interactive target under 24×24 CSS px |
| 390px and 320px viewports | `scrollWidth` equals the viewport; no glass, stage or desk element past the viewport edge |
| WebKit 26.6 (sunset, pastel) | Frosted fallback renders: `-webkit-backdrop-filter: blur(14px) saturate(1.6) brightness(1.05)`, trig-based inset highlight (`0.707px` offsets), masked rim (`-webkit-mask-composite: xor`), no lens on controls, status reports refraction unavailable; no console errors |
| Backdrop luminance | Median / 90th-percentile relative luminance behind the widget stack: sunset .455/.534, meadow .726/.789, poppy .329/.398, deep .033/.089, studio .66/.903, pastel .899/.952, stress .167/.683 (used by the lab's estimate) |

## Measured contrast behind the widget title

Text hidden, region captured, luminance percentiles compared with the ink (Chromium, 1440px). Numbers are for this lab's composition and are sampled evidence, not certification. Final run, after the ink modes were assigned by measured backdrop luminance:

| Backdrop | Ink mode | Title, clear | Title, regular | Title, frosted | Secondary (regular) |
|---|---|---:|---:|---:|---:|
| Sunset | dim (white on smoked tint) | 5.70 / 2.64 | 5.70 / 2.64 | 5.70 / 2.64 | 5.23 |
| Meadow | dark (dark ink on white frost) | 14.51 / 14.09 | 15.12 / 14.82 | 15.60 / 15.45 | 4.85 |
| Poppy | dim | 7.29 / 2.79 | 7.29 / 2.79 | 7.29 / 2.79 | 6.66 |
| Deep | light (white on light tint) | 10.83 / 3.72 | 7.58 / 3.17 | 4.21 / 2.36 | 6.92 |
| Studio | dark | 15.67 / 15.24 | 15.80 / 15.53 | 16.04 / 15.80 | 4.91 |
| Pastel | dark | 14.13 / 13.84 | 14.85 / 14.65 | 15.50 / 15.39 | 4.83 |
| Stress | light | 6.84 / 2.79 | 5.29 / 2.56 | 3.30 / 2.03 | 4.86 |

Title cells are median / worst 5th percentile; the dim presets ignore the thickness switch by design (their alpha is fixed by the scene rule). The earlier run, before the modes were reassigned, had white ink on the light tint over the sunset at 1.70 / 1.40 and over the poppy at 2.57 / 1.56, and secondary text at 62–74% alpha below 4:1 everywhere; those pairings were removed from the lab rather than documented as a tier. Worst-percentile figures on the photographic scenes come from bright blooms behind part of the card and are the reason dense text sits on a plate. The lab shows the analytic estimate live so a user changing tokens sees the tier change.

## Unmeasured boundaries

No physical low-power device profiling, calibrated FPS benchmark, Firefox execution, real Safari (Playwright WebKit stands in for it), screen-reader audit, physical print run, full localization/RTL audit, or native SwiftUI/visionOS/WinUI/Expo compilation was performed. The refraction path was executed only in Chromium software rendering. The 400% zoom reflow was approximated by real 320px and 390px viewports. No with-skill/without-skill agent comparison was run; no measured improvement in agent performance is claimed.
