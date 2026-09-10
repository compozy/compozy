# Web engineering

Use for CSS implementation, framework integration, rendering failures and performance diagnosis. For visual direction use [art-direction.md](art-direction.md); for contrast and preferences use [accessibility-validation.md](accessibility-validation.md); for refraction use [advanced-platforms.md](advanced-platforms.md#web-refraction-and-lensing).

## Implementation contract

The renderer samples the pixels behind the surface, applies `backdrop-filter`, then composites fill, decoration and content. An opaque fill hides the sampled pixels; a uniform backdrop shows no blur. `filter: blur()` on the component blurs its own text; use it only on a decorative sibling.

Start from [glass.css](../assets/glass.css) and read its selectors before adapting. Its layer order, bottom to top:

1. **Surface** (`.glass`): translucent fill, `backdrop-filter`, inset highlights and elevation shadow.
2. **Sheen** (`.glass::after`, `z-index: 0`): a directional gloss gradient, non-interactive.
3. **Content** (`.glass > *`, `z-index: 1`): children are lifted above the sheen; the rule sets `position: relative`, so absolutely positioned children need their own later rule.
4. **Rim** (`.glass::before`, `z-index: 2`): a 1px masked gradient ring, non-interactive.

`isolation: isolate` on `.glass` keeps those z-indexes local. Cascade order in the file:

1. Theme tokens and material primitives on `:root`; thickness presets on `:root[data-material]`.
2. Variants on the element (`.glass-light`, `.glass-smoke`, `.glass-tinted`, `[data-ink="dark"] .glass`).
3. Solid baseline for every browser.
4. `@supports (backdrop-filter)` enhancement.
5. States.
6. Preference overrides (`data-transparency="reduce"`, `prefers-reduced-transparency`, `prefers-reduced-motion`, `forced-colors`, `print`) that beat everything above.

The essential surface, condensed:

```css
:root {
  --glass-tint: 255 255 255; --glass-alpha: .16; --glass-blur: 14px; --glass-saturate: 1.6; --glass-brightness: 1.05;
  --glass-light: 135deg; --glass-rim: .8; --glass-rim-near: 255 255 255; --glass-rim-far: 255 255 255;
  --glass-sheen: .22; --glass-ink: #fff; --glass-shade: 20 18 30; --glass-elevation: 18px;
  --glass-grain: none; --glass-lens: ; --glass-solid: #3b3f4b;
}
.glass { position: relative; isolation: isolate; color: var(--glass-ink); background: var(--glass-solid); border-radius: var(--glass-radius, 24px); }
.glass > * { position: relative; z-index: 1; }
@supports (backdrop-filter: blur(1px)) {
  .glass {
    background: var(--glass-grain) center / 160px, rgb(var(--glass-tint) / var(--glass-alpha));
    background-blend-mode: overlay, normal;
    backdrop-filter: var(--glass-lens) blur(var(--glass-blur)) saturate(var(--glass-saturate)) brightness(var(--glass-brightness));
    box-shadow:
      inset calc(sin(var(--glass-light)) * 1px) calc(cos(var(--glass-light)) * -1px) 0 rgb(var(--glass-rim-near) / calc(.5 * var(--glass-rim))),
      inset calc(sin(var(--glass-light)) * -1px) calc(cos(var(--glass-light)) * 1px) 0 rgb(var(--glass-rim-far) / calc(.14 * var(--glass-rim))),
      0 1px 2px rgb(var(--glass-shade) / .1),
      0 var(--glass-elevation) calc(var(--glass-elevation) * 2.4) calc(var(--glass-elevation) * -.8) rgb(var(--glass-shade) / .28);
  }
  .glass::before { /* rim */
    content: ""; position: absolute; inset: 0; z-index: 2; border-radius: inherit; padding: 1px; pointer-events: none;
    background: linear-gradient(var(--glass-light),
      rgb(var(--glass-rim-near) / calc(.95 * var(--glass-rim))), rgb(var(--glass-rim-near) / calc(.35 * var(--glass-rim))) 32%,
      rgb(var(--glass-rim-far) / calc(.1 * var(--glass-rim))) 62%, rgb(var(--glass-rim-far) / calc(.45 * var(--glass-rim))));
    mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); mask-composite: exclude;
  }
  .glass::after { /* sheen */
    content: ""; position: absolute; inset: 0; z-index: 0; border-radius: inherit; pointer-events: none;
    background: linear-gradient(var(--glass-light), rgb(var(--glass-rim-near) / var(--glass-sheen)), rgb(var(--glass-rim-near) / calc(var(--glass-sheen) * .25)) 34%, transparent 60%);
  }
}
```

Notes that matter when adapting:

- **Primitive tokens live on `:root`, derived values on the element.** `rgb(var(--glass-tint) / var(--glass-alpha))` is written in the `background` of `.glass`; a `--glass-fill` declared at `:root` would resolve there and ignore descendant overrides. Global multipliers (`--glass-alpha-mul`, `--glass-blur-mul`) keep tuning UIs from fighting variants.
- **Empty custom property as an optional filter slot.** `--glass-lens: ;` is valid and substitutes to nothing, so `backdrop-filter: var(--glass-lens) blur(…)` stays valid; JS sets it to `url(#id)`.
- **No self-referencing tokens in states.** `:hover { --glass-alpha: calc(var(--glass-alpha) + .06) }` is a cycle and invalidates the fill. Use a second token (`--glass-hover`) added in the consuming expression.
- **Masked rim** needs `mask-composite: exclude` (standard) and `-webkit-mask-composite: xor` (WebKit); include both. The corpus's only walkthrough (`raw/youtube/create-this-trendy-blurry-glass-effect-with-css.md`) uses the equivalent `subtract` with two identical gradients that differ only by box (`border-box` minus `padding-box`), a `--border-width` alias, and `border-radius: inherit` instead of `overflow: hidden`; single-stop gradients are Chrome-only, so repeat the color. Under `forced-colors` the mask image is suppressed; the file removes the pseudo-elements there.
- **Trigonometric offsets** (`sin()`, `cos()`) are gated behind `@supports (top: calc(sin(1deg) * 1px))`; the fallback keeps a fixed top highlight.
- **Grain** is an inline SVG `feTurbulence` data-URI (`baseFrequency` .65–.9, `numOctaves` 2–3, `stitchTiles="stitch"`, 160–182px tile) with baked opacity, layered as a second background and blended with `background-blend-mode`; the corpus's `opacity: .12` is a full-bleed background default, on glass keep the visible amount at 1–3% (`raw/articles/creating-grainy-backgrounds-with-css.md`, Malewicz). Forced colors and print remove it with the rest.
- **Apple's own CSS pen** (`raw/articles/getting-clarity-on-apple-s-liquid-glass-css-tricks.md`) splits the material into an effect node (`backdrop-filter: blur(3px)` + `filter: url(#glass-distortion)`), a tint node (`rgba(255 255 255 / .25)`) and a shine node (`inset 2px 2px 1px 0 rgba(255 255 255 / .5), inset -1px -1px 1px 1px rgba(255 255 255 / .5)`) with the text above; merging them onto one node produced "weird color bleed on the corners in Chrome" and blurred the bevel in the Show HN generator thread. `glass.css` keeps the same separation with pseudo-elements.
- **Plates, not nested filters.** `.glass-plate` is a denser tinted region (`rgb(tint / .22)`, white `.55` in dark-ink mode) with no `backdrop-filter`.
- **Ink modes** are scene-level: set `data-ink="dark"` (white frost, dark ink) or the lab's `dim` on a container; every `.glass` inside inherits the pairing, including `--glass-focus` for rings.

Do not change transparency with `opacity` on the wrapper: it fades content and changes backdrop sampling.

## Backdrop roots, stacking, and overlays

A **stacking context** controls paint order; a **backdrop root** limits what a filter can sample. They are not interchangeable.

- Ancestors with `opacity < 1`, `filter`, certain masks/clipping, `backdrop-filter` or a non-normal `mix-blend-mode` bound sampling; some `will-change` values create the boundary before any effect is visible.
- `transform`, fixed/sticky positioning and a positioned `z-index` create stacking behavior without being backdrop roots; `isolation: isolate` is neither a universal cause nor a cure for missing blur.
- A backdrop-filtered element becomes the containing block for positioned descendants; a viewport-fixed menu inside it may misbehave.
- Portals and top-layer dialogs solve overlay ownership and clipping; keep the app's existing dialog/popover system.

The [Filter Effects Level 2 draft](https://drafts.csswg.org/filter-effects-2/#BackdropRoot) defines the backdrop root and notes the definition lacks full consensus. Explain likely behavior with it, then reproduce on target engines.

| Symptom | Inspect | Repair |
|---|---|---|
| No visible blur | Uniform/opaque backdrop; opaque fill; unsupported value | Put a test pattern behind it; inspect computed fill/filter; test the solid branch |
| Child glass cannot see the page | Filter/opacity/mask/backdrop-filter on ancestors | Remove the boundary or move the material to the sampling level |
| Rim missing on one browser | `mask-composite` prefix | Add the `-webkit-` pair |
| Highlight ignores the light angle | No trig support | Expected fallback; verify the `@supports` branch |
| Header clips menus/focus | Overflow and containing blocks | Clip decoration only; use the overlay layer |
| Seam or dirty overlap | Adjacent or nested filtered panes | One material plane; plates inside |
| Refraction shows on everything | A `[data-lens]`-style selector also matched the root or a container | Scope the selector to `.glass[data-lens]`; never inherit `--glass-lens` |
| A preset changes labels but not fill | Derived custom property resolved at the root | Consume primitives on the element |
| Solid mode shows white on white | Theme and fallback cascade | Pair each mode's `--glass-solid` with its `--glass-ink` |

## Tailwind, React, and shadcn

Inspect the installed version and conventions. Keep semantic tokens (`surface-chrome`, `surface-reading`, `surface-solid`, `ink`, `edge`, `focus`) as the material's boundary rather than scattering blur/alpha utilities.

- **Tailwind:** keep the material as a reusable class or the project's variant mechanism so the solid-first cascade and the pseudo-element rim survive; `backdrop-blur-*` alone supplies no fill, rim, contrast or fallback. Confirm the configured scale before assuming a utility maps to a radius; check the version's theme syntax before writing `@theme`.
- **React:** a material wrapper needs no JavaScript unless it refracts. Keep semantic elements and existing primitive props/refs/ARIA; do not wrap buttons in buttons or replace a dialog with a `div`. Drive theme, material and ink modes with data attributes and the app's state conventions. Refraction filters are generated per size (see advanced effects) and injected once into a shared `<svg><defs>`.
- **shadcn/Radix:** theme the installed primitives; keep focus management, portals, keyboard dismissal and states. Menus and palettes get glass shells and solid rows; inputs and chart planes stay solid. Third-party glass libraries are optional; check maintenance, browser behavior and bundle cost. shadcn's Luma is inspired by Liquid Glass **without the glass effects**.

No dependency or framework migration is required by this skill.

## Measure cost, then simplify the cause

The corpus's performance article (`wiki/concepts/Backdrop Filter Performance Budget.md`) prices the operation: the browser renders the scene behind the element, filters it and re-composites, every frame the backdrop changes, and "every `backdrop-filter: blur()` call is a separate compositing pass on the GPU." The trigger is a product, not a threshold: **area × radius × count × change rate**, counted in device pixels ("extremely noticeable on my 2560×1440" where a 1080p laptop showed nothing). Fill opacity is nearly free; radius is not. The review metric that follows: sum the rendered area of every visible element carrying `backdrop-filter`, divide by viewport area, and keep it small.

| Surface | Dominant driver | Corpus verdict |
|---|---|---|
| Fixed nav, sticky header | the whole page moves behind a small box | live filter inside the panel band (Apple's nav audits at 20px); always the paired `-webkit-` declaration |
| Modal or sheet scrim | repeated overlays, hover latency on big screens | flat alpha dim, not blur (shadcn/ui #327 fixed `backdrop-blur-sm` with `bg-slate-950/40`; tailwindcss #5023) |
| Full-height sidebar | largest filtered area, re-filtered every frame | opaque or gradient |
| Card grid | count × area | pre-blurred asset (a 50-card `filter: blur()` grid hit ~2s click-to-response) |
| Overlay on video | backdrop changes 30–60× per second | no live filter; controls carry their rim and highlight for legibility |
| Pill, badge, tooltip, small lens | tiny area | affordable |
| Glass over flat color | nothing to sample | opaque: "you paid the GPU tax for nothing" |

Documented pathologies to recognize: viewport-edge color flicker because off-screen pixels are never sampled (fix with a gradient, not more filtered area); Safari's element-local stall around `filter: blur()` with a disputed `translate3d` workaround; Intel machines choppy on video where Apple silicon was fine; low-end Android frame drops on full-screen large radii. A `backdrop-filter` also makes the element a containing block, "the classic 'my modal broke' bug" for fixed descendants.

Levers, in the corpus's priority order: shrink the filtered area (inset the panel), raise fill instead of radius, fix edge flicker with a gradient, never animate the blur radius, de-glass off-screen and inactive surfaces (`content-visibility`, unmounting, inactive tabs), pre-render surfaces whose source never changes, tier by device, and only then layer-promotion hints scoped to the animating state and removed afterward. An SVG `url()` lens multiplies the chain; keep lensing to one or two small controls, regenerate maps only when the shape changes, and drop lensing on phones if the target demands it.

Triage when a page got slow after adding glass:

1. Isolate one interaction (scroll the list, open the sheet, hover the cards).
2. Record DevTools Performance for that interaction; read Recalculate Style, Layout, Paint, Composite and dropped frames; check composited areas and paint flashing.
3. The culprit signature is a long paint/composite band right after the input with negligible JavaScript.
4. Bisect: disable one declaration, repeat the identical interaction; if unchanged, the blur was not the cause.
5. Scope the confirmed cause: element too large, repeated too often, radius excessive, active off-screen, changing every frame, pre-renderable?
6. Verify on low-powered mobile, several pixel densities, Chrome, Firefox and Safari, CPU throttling, real data volumes, and at least one machine without a dedicated GPU.

Support: `backdrop-filter` is Baseline since 2024, but WebKit shipped it prefixed and a fixed header that "did nothing in Safari" until `-webkit-backdrop-filter` was added is a documented regression, so emit both declarations; the claim that Safari drops `var()` inside the property is disputed and did not reproduce in Playwright WebKit 26.6 against `glass.css`. SVG-filter backdrops remain Chromium-only.

## Primary references

- [MDN backdrop-filter](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/backdrop-filter), [CSS custom properties](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_cascading_variables/Using_CSS_custom_properties), [WebKit introducing backdrop filters](https://webkit.org/blog/3632/introducing-backdrop-filters/).
- WebTricks, [Liquid glass in CSS — frost, rim, sheen and real refraction](https://webtricks.dev/blog/liquid-glass-css) (2026): the three-ingredient cross-browser core and the Chromium-only `@supports (backdrop-filter: url())` gate.
- iart-ai, [glass recipes](https://github.com/iart-ai/web-animation-skills/blob/main/skills/glassmorphism/references/glass-recipes.md): masked-gradient rim, grain to kill banding, animating qualities not geometry, `@starting-style` entrance.
- SquareMediaGroup, [glassfx](https://github.com/SquareMediaGroup/glassfx): token vocabulary (`--glass-tint`, `--glass-alpha`, `--glass-blur`, `--glass-rim`, `--glass-sheen`), cursor bloom, mobile cutoff.
- [Tailwind backdrop blur](https://tailwindcss.com/docs/backdrop-filter-blur), [shadcn Luma announcement](https://ui.shadcn.com/docs/changelog/2026-03-luma).
