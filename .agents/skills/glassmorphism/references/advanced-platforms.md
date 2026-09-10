# Advanced effects and platform routes

Read only the route matching the actual target. A web approximation does not inherit native sampling, adaptation, vibrancy, accessibility, or battery behavior.

## Web refraction and lensing

Frosted glass blurs the backdrop. Refraction displaces it according to surface geometry: a calm center and a bend at the bezel, with a faint color split where the bend is strongest. A white gradient is a highlight, not refraction; procedural turbulence is a wobble, not a lens. Choose the simplest technique that achieves the brief:

| Technique | Useful result | Limits |
|---|---|---|
| Fill + backdrop blur + rim and sheen | Reliable frosted/smoked material everywhere | No lensing |
| Geometry-derived map through `backdrop-filter: url()` | Live backdrop bends at the rim of a small control | Chromium only; per-size filter; adds a filter pass |
| Geometry-derived map through `filter: url()` on a copy of the content | Bending in Chromium, Safari and Firefox | You must own and duplicate what sits behind the glass; WebKit quirks |
| `feTurbulence` + `feDisplacementMap` | Cheap animated "liquid" wobble | Not a lens; distorts text if applied to the component |
| Canvas/WebGL renderer | Optical simulation over assets the app owns, including live video | Cost, semantics, synchronization |
| Native platform glass | Adaptation and transitions | Native validation required |

Keep readable content outside the displaced layer: a lens applied with `filter` warps its own label. The lab keeps refraction on `backdrop-filter` of the control and renders the icon on top.

### Geometry-derived displacement map

The lab generates the map on a canvas the size of the control (`makeMap` in [material-lab.html](../assets/material-lab.html)):

1. **Signed distance to a rounded rectangle** gives, per pixel, the distance inward from the edge and the outward normal (straight sides and corners treated uniformly, so the bend wraps the whole perimeter).
2. **Height profile** over the bezel width `B` (≈ .42 of the shorter side): a convex squircle `h(t) = (1 − (1 − t)⁴)^¼`, which keeps the flat→curve transition smooth when the shape is stretched into a pill.
3. **Snell's law** with `n = 1.5`: from the slope, `θ₁ = atan(slope)`, `θ₂ = asin(sin θ₁ / n)`, lateral shift `= h(t) · tan(θ₁ − θ₂)`. The shift peaks a few pixels inside the edge and fades to zero by mid-bezel, which is the "calm center, bend at the rim" look.
4. **Encode inward.** `R = 128 − nₓ·m·127`, `G = 128 − n_y·m·127`, neutral `128` outside the shape. Sampling inward means the filter never asks for pixels beyond the element, which avoids the filter-region trap below.
5. **Scale.** `feDisplacementMap` moves each pixel by `scale × (channel/255 − .5)`, so `scale = 2 × maxShiftPx`; the lab sets `maxShiftPx = strength × min(w, h)` with strength `.12–.30`.

```html
<filter id="lens-1" x="0" y="0" width="100%" height="100%" color-interpolation-filters="sRGB">
  <feImage href="data:image/png;base64,…" x="0" y="0" width="64" height="64" preserveAspectRatio="none" result="map"/>
  <feDisplacementMap in="SourceGraphic" in2="map" scale="28.2" xChannelSelector="R" yChannelSelector="G" result="d0"/>
  <feColorMatrix in="d0" type="matrix" values="1 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1 0" result="c0"/>
  <feDisplacementMap in="SourceGraphic" in2="map" scale="30" … result="d1"/><feColorMatrix in="d1" … result="c1"/>
  <feDisplacementMap in="SourceGraphic" in2="map" scale="31.8" … result="d2"/><feColorMatrix in="d2" … result="c2"/>
  <feBlend in="c0" in2="c1" mode="screen" result="c01"/><feBlend in="c01" in2="c2" mode="screen"/>
</filter>
```

```css
.glass[data-lens] { backdrop-filter: var(--glass-lens) blur(4px) saturate(1.6); } /* JS sets --glass-lens: url(#lens-1) */
```

Rules that came out of building and testing it:

- **`color-interpolation-filters="sRGB"`** on the filter. In the default linearRGB, the map's `128` is no longer neutral and the whole control shifts.
- **Chromatic fringe** is three displacement passes at ±6% scale, each reduced to one channel with `feColorMatrix` and recombined with `feBlend mode="screen"`. Keep the spread small; wide spreads read as a broken monitor.
- **One filter per size and radius.** `feImage` dimensions are user-space pixels of the element; share a filter between identical controls, regenerate on resize (ResizeObserver), and only when the shape changes, never on every move.
- **Filter region and alignment traps.** Maps that displace outward need an enlarged filter region (`x/y/width/height` beyond 100%) and a canvas padded with neutral gray to keep the shape aligned; inward-only maps avoid both.
- **Scope the selector.** `[data-lens]` on `<html>` or a container matched every `.glass` in an early lab build and gave cards page-sized maps. Use `.glass[data-lens]` and never inherit `--glass-lens`.
- **Gate by engine, not `@supports`.** `backdrop-filter: url()` parses in other engines but only Chromium renders it; the lab checks the user agent and shows the frosted fallback elsewhere, and disables lensing under reduced transparency, forced colors and manual solid mode.
- **Validate visually.** Screenshot a control over a hard color edge; the edge must bend inside the rim and stay straight in the center. Headless Chromium software rendering reproduces it.

### Cross-browser bending with a copy

Aave's [Building glass for the web](https://aave.com/design/building-glass-for-the-web) and the `samasante/liquid-glass` React lens apply the same map with `filter: url()` to a *copy* of the content behind the glass (a hero, a track fill, a wallpaper image) while the crisp children render on top. That works in Safari and Firefox because nothing is sampled from underneath; the copied pixels move. The costs they document: Safari caches filter output by id, so a changed map needs a fresh id; Safari has a ceiling on the source-graphic size a filter processes; Safari will not filter a live `<video>`, which needs a WebGL path; regenerate only a quadrant of the map and mirror it when performance matters. The copy must match scale, crop, scroll and position of the real scene, and it is not an arbitrary-DOM backdrop solution.

### Turbulence shortcut

`feTurbulence` → `feGaussianBlur` → `feDisplacementMap` (`scale` 12–25, `baseFrequency` ≈ .008) produces an organic ripple in one filter and animates cheaply by moving `baseFrequency` or `scale`. It is a style, not optics: use it for a playful hero, keep it off text, pause it under reduced motion, and clamp `scale` (Safari artifacts above ~18 in one library's testing).

[Kube's article](https://kube.io/blog/liquid-glass-css-svg/) explains the physics and the 8-bit map encoding; note that the normalized range is `±scale/2` (displacement is `scale × (channel − .5)`), 8 bits quantize the map rather than capping it at ±128px, and its Chrome-only note is a current engine fact, not a permanent one. LogRocket's tutorial authors the map and a specular rim in Figma; Yvaine's Figma-glass port adds light-aware inner shadows and confirms the sRGB rule.

## Motion and material continuity

Use motion to explain where a control or sheet came from. Keep a consistent light direction during a transition; let a nearby highlight or restrained press response carry the tactile impression. Preserve reading contrast throughout the animation, including intermediate fills. Avoid permanent pointer-following distortion behind text.

Nearby surfaces can visually merge, but a convincing morph requires compatible shape, identity, position and timing. On the web, animate a decorative shell or use existing transition primitives while the semantic controls keep stable focus and hit targets. Do not clone active buttons for an animation or introduce a second motion library solely for glass. Under reduced motion, preserve the final state and causal relationship with an immediate or gentle non-spatial transition; disable tilt, parallax, lens motion and animated backdrop where appropriate.

## Apple Liquid Glass and SwiftUI

Apple's Liquid Glass is an adaptive functional layer for navigation and controls. Standard materials serve content organization. Prefer the platform's navigation, toolbar, sheet and button components; audit custom backgrounds that cover the system material or scroll-edge treatment. Do not turn a content list/table into a sheet of custom Liquid Glass simply to match a screenshot.

For a supported OS/SDK, a native button can use `.buttonStyle(.glass)` or `.glassProminent`. Custom `.glassEffect` belongs after size, padding and appearance modifiers. Use `Button` for a button: `.interactive()` supplies material response, not application event handling or accessibility semantics.

```swift
// Illustrative OS 26 branch; integrate with the app's deployment checks.
if #available(iOS 26.0, macOS 26.0, *) {
    Button("Add item", action: addItem)
        .buttonStyle(.glass)
} else {
    Button("Add item", action: addItem)
        .buttonStyle(.bordered)
}
```

Use `.regular` as the normal material choice. `.clear` is for media-rich backgrounds with suitably strong foreground treatment, not a universal “more premium” setting. Apple's guidance about a 35% dark dimming layer for some bright backgrounds under clear glass is native context-specific advice, not a web contrast floor. Do not translate semantic native foreground colors into purportedly official CSS hex values.

For related custom glass shapes, `GlassEffectContainer(spacing:)` coordinates rendering and merging. Its spacing is a merge threshold, not layout padding. Use stable `glassEffectID(_:in:)` identities and a namespace for related transitions; `glassEffectUnion(id:namespace:)` and materialization transitions are specialized tools, not required on every control. Verify signatures and availability against the installed SDK before writing a larger implementation.

Test custom work with reduced transparency, increased contrast, reduced motion, Dynamic Type, themes and the actual input methods. Native adaptation does not validate custom tints or animations automatically. Sources: [Applying Liquid Glass to custom views](https://developer.apple.com/documentation/swiftui/applying-liquid-glass-to-custom-views), [Adopting Liquid Glass](https://developer.apple.com/documentation/technologyoverviews/adopting-liquid-glass), [HIG Materials](https://developer.apple.com/design/human-interface-guidelines/materials), [WWDC25 Meet Liquid Glass](https://developer.apple.com/videos/play/wwdc2025/219/).

## visionOS

`glassBackgroundEffect(in:displayMode:)` is a separate visionOS material API that participates in z-axis layout. It is not an alias for OS 26 `.glassEffect`. Apply the effect to the intended container; Apple's documentation calls out explicit `ZStack` ownership when background/overlay modifiers otherwise imply a stack. Spatial material must work with physical surroundings, gaze targets and depth.

Do not infer spatial comfort or interaction validity from a web screenshot. Keep depth away from text, avoid excess simultaneous windows and large head-anchored panels, and test in the native environment. See [glassBackgroundEffect](https://developer.apple.com/documentation/swiftui/view/glassbackgroundeffect(in:displaymode:)).

## Windows Fluent

| Material | Role | Honest web analogy |
|---|---|---|
| Acrylic | Translucent backdrop/blur/blend/tint/noise material for transient surfaces and appropriate in-app overlays | Frosted overlay with explicitly engineered contrast |
| Mica / Mica Alt | Opaque long-lived app base informed by wallpaper and window state | Subtly tinted opaque base, not live wallpaper blur |
| Smoke | Dimming layer indicating blocked underlying interaction | A scrim plus actual modal behavior |
| Solid | Stable content/control surfaces | Solid reading/interaction regions |

Acrylic's full native blend stack and text resources matter; blur plus white alpha does not inherit Microsoft's contrast tuning. Prefer solid permanent partitions; use in-app Acrylic when navigation genuinely overlays content. Avoid adjacent independent acrylic panes that produce seams and large stacked material regions. Detailed Acrylic guidance is more nuanced than the short “transient only” overview; do not ban every overlay sidebar.

Use the app's installed Windows framework and its theme resources. Native material changes for high contrast, transparency settings, power/device conditions and inactive windows do not transfer to CSS automatically. Sources: [Acrylic](https://learn.microsoft.com/en-us/windows/apps/design/style/acrylic), [Mica](https://learn.microsoft.com/en-us/windows/apps/design/style/mica), [Fluent 2 Material](https://fluent2.microsoft.design/material).

## Material Design

Borrow hierarchy, containment, consistent elevation, shape and meaningful motion while preserving the chosen system's components. Do not invent Material glass tokens, map M2 elevation dp to blur pixels, or call Material 3 Expressive a glass system. The corpus's surface/elevation material is explicitly [Material 2](https://m2.material.io/design/environment/surfaces.html); Google's [Expressive research](https://design.google/library/expressive-material-design-google-research) studies its own design changes, not glass-versus-opaque usability.

## React Native / Expo

When the actual project uses Expo, inspect its installed `expo-glass-effect` version. Current documentation exposes native-backed `GlassView` and `GlassContainer`, with capability checks `isLiquidGlassAvailable()` and `isGlassEffectAPIAvailable()` addressing different build/runtime conditions. An unsupported target may fall back to a regular `View`; supply a readable fallback style yourself. A BlurView/Skia implementation is a different approximation with its own performance and platform behavior.

Do not install or replace packages solely because this route exists. Check current props, compiler/OS requirements, and animation/ancestor-opacity caveats in [Expo GlassEffect](https://docs.expo.dev/versions/latest/sdk/glass-effect/), then run the app on intended platforms. These native snippets and API notes are reference guidance, not compiled multi-platform validation of the bundled web lab.
