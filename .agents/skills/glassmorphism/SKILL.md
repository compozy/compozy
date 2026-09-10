---
name: glassmorphism
description: "Create, refine, or debug glass interfaces at the level of Apple Liquid Glass and premium frosted widgets: translucent cards and capsules over photos or wallpapers, glass navigation and toolbars, smoked media controls, lens-refracting buttons, and their solid/accessible fallbacks. Use for glass material design, CSS backdrop-filter stacks (fill, rim, sheen, shadow, grain), SVG displacement refraction, glass tokens, contrast on translucent surfaces, rendering cost, and conditional SwiftUI/visionOS/Fluent guidance. Also matches interfaces de vidro, vidro fosco, and liquid glass. Excludes unrelated frontend edits, generic redesigns without glass, and photorealistic 3D glass rendering."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# Glassmorphism

Build a material whose translucency reads as a lit pane of glass while text stays legible and controls stay usable. Premium glass comes from a backdrop that carries light, a thin light fill, one locked light direction expressed through rim, highlight and shadow, and a stabilized plate under dense content. It never comes from raising blur or opacity.

## Establish the material's job

Read the brief and the existing interface for platform, surface roles, reading density, who owns the backdrop, themes, and target devices. Preserve the project's components, typography, tokens and interaction conventions. Infer routine choices; this skill adds no approval step or planning artifact.

Separate three layers before styling: **environment** behind the interface, **content** people read or manipulate, **chrome** that navigates or controls it. Glass needs a reason to reveal the environment. Over flat color the blur has nothing to sample and the surface becomes a gray box; use a tinted solid there, or compose a backdrop with a luminance ramp when the brief allows one.

| Surface | Starting decision | Change it when |
|---|---|---|
| Floating toolbar, dock, tab bar, compact media or map control | Glass capsule; icons on the material, selected state as a denser chip | Uncontrolled imagery or device cost defeats legibility |
| Glanceable widget (weather, metric, now playing) | Glass card with large light numerals and short labels; ink chosen by backdrop luminance | Copy grows past a few short lines: add a plate or go solid |
| Popover, sheet, command palette, settings panel | Glass shell with a stabilized plate under fields, rows and code | Sustained reading: solid panel |
| Table, editor, long text, chart labels, forms | Solid reading plane; optional glass around it | Only after measuring the composite behind the text |
| Primary action, errors, important status | Stable foreground/background pairing and recognizable states | A native platform control already supplies verified material |
| Blocking modal backdrop | Dimming scrim | A measured, purposeful effect justifies more rendering |

## Load only the branch needed

Resolve paths from this skill directory. References are self-contained; the research vault is optional.

| Need | Read |
|---|---|
| Name the target look, judge a result, pick tokens, fix flat/muddy/plastic glass, author in Figma | [Art direction and materials](references/art-direction.md); the quality bar is [the reference board](references/reference-board.md) |
| Implement web glass, integrate with Tailwind/React/shadcn, diagnose missing blur, clipping, stacking | [Web engineering](references/web-engineering.md); adapt [glass.css](assets/glass.css) |
| Put text or controls on glass, accept arbitrary backdrops, measure contrast, honor preferences | [Accessibility and validation](references/accessibility-validation.md) |
| Add lens refraction, chromatic fringe, morphing, or target native Liquid Glass, visionOS, Fluent, Material, Expo | [Advanced effects and platforms](references/advanced-platforms.md), only the matching section |
| Trace a rule to its source or extend the skill | [Evidence and provenance](references/evidence.md); the coverage catalog is bookkeeping, not runtime reading |

[material-lab.html](assets/material-lab.html) with its stylesheet is a runnable reference: seven backdrops including a stress board, three thickness presets, light angle, fill and blur multipliers, refraction, grain, solid mode, a live contrast estimate and a token readout. It demonstrates the material; adapt it into the target stack rather than copying the page.

## Implement a coherent material

1. **Compose the backdrop and the ink.** Photos, wallpapers and gradients with a luminance ramp make glass; keep exposed environment around every surface. Choose the ink by the luminance behind the surface, and measure: white ink on a thin light tint only over dark scenes (relative luminance below ~.1); white ink on a smoked neutral tint (`--glass-tint: 56 50 58` at ~.66) over mid and pastel scenes; dark ink on white frost (~.62) over pale and near-white scenes (above ~.6). White on a light tint over a pastel photo renders below 2:1; the lab switches modes per backdrop and shows the estimate. Apple's material makes this switch automatically; on the web you decide it per scene.
2. **Lock one light direction and express it five ways.** `--glass-light` drives the 1px rim gradient (bright on the lit edge, dim opposite), the 1px inset highlight and its shadowed twin, the sheen falloff, and the elevation shadow. Highlights on every edge, or a broad white wash, read as plastic.
3. **Tune thin, then thicken only where reading needs it.** Start from the presets (clear `.07/6px`, regular `.16/14px`, frosted `.34/26px` with `saturate(1.4–1.7) brightness(1.05)`), then add a `.glass-plate` under dense text instead of raising the whole surface's alpha. Capsules for controls, 24–30px radii for cards, concentric inner radii.
4. **Protect content and interaction.** Filter the surface, never the label subtree; keep rim and sheen pseudo-elements non-interactive; change fill alpha, not element opacity. States are structural: hover raises fill and rim, press compresses shadow, disabled loses rim and sheen, focus is a crisp ring above the stack. Preserve native semantics, hit targets and dismissal.
5. **Ship the solid twin first.** Declare the opaque fill/ink pair, then enhance inside `@supports`. Provide an application-level solid mode; honor reduced transparency, forced colors, reduced motion and print explicitly.
6. **Add refraction only where the bend reads as thickness.** Small controls (buttons, knobs, pills) can use a geometry-derived displacement map through `backdrop-filter: url()`; it renders only in Chromium and must degrade to frost. Cards stay frosted.

Keep the number, area and overlap of live backdrop filters low: one to three glass surfaces per view, the navigation layer counting as one, at most one transient sheet above it. Glass is the chrome layer, not the content layer; never put glass on glass (use tinted plates inside a shell), never animate blur, never filter the full viewport by default. Do not mix Apple's regular and clear tiers on one screen; larger surfaces read thicker than small controls.

## Validate the requested result

Run the actual interface with the project's checks and inspect representative states in a browser or native simulator. Scope to what changed: a CSS-only material change needs UI checks, not a new suite that freezes styling.

- **Visual:** the material is recognizable at intended size; rim, highlight and shadow agree on one light direction; hierarchy survives light/dark and quiet/busy backdrops, including a hotspot and a high-contrast shape behind text; nothing collapses into full-screen blur on mobile.
- **Functional:** keyboard focus and pointer work on every control; menus, loading, error and disabled states stay understandable; decoration does not intercept input or clip focus.
- **Accessible:** measure ink against the rendered composite behind the letters (hide the text, sample the region, take the worst percentile), on the worst permitted backdrop. In the lab the adapted modes measure 5.7:1 to 15:1 at the median for titles and 4.8:1 to 9.9:1 for secondary text; white ink on a light tint over the same pastel photo measured 1.7:1, which is why the mode switches. Verify solid, forced-colors, reduced-motion and 320px reflow states.
- **Rendering:** exercise real scroll, resize, overlays and motion on target engines; compare with filters disabled when cost is uncertain. Feature detection proves syntax, not fidelity.

Fix failures in the owning layer: add a plate or change ink for contrast, remove filtered area for cost, repair ancestor compositing for missing blur. Do not add blur, `z-index` or `translateZ(0)` as a fix.

Deliver the artifact, state the checks performed, and disclose untested engines, devices and experimental effects. The research supports design and engineering guidance, not measured usability gains.
