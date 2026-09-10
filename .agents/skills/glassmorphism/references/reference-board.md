# Reference board

Twenty-two images chosen by the author on 2026-09-10 from Savee as the quality bar for this skill. They are third-party work and are not bundled; the Savee links resolve to the originals. Read this when a brief says "like Apple Liquid Glass", "like the iOS weather widgets", or when a result looks flat, gray, or plastic and you need to name what is missing. The lab reproduces the traits below with CSS; the numbers in [art-direction.md](art-direction.md) were tuned against these renders.

## What the board has in common

- **The backdrop carries light.** Every reference sits on a photograph, a wallpaper-like gradient with a luminance ramp, or a neutral studio field with a bloom. None sits on flat color.
- **The fill is thin and light, not dark and opaque.** Over photos the tint is white (or a warm gray on pale scenes) at low alpha; the scene shows through as color, not detail.
- **One light direction.** A bright 1px rim on the lit edge, a dimmer rim opposite, a soft inner highlight and a shadow displaced away from the light. Highlights never appear on all edges at once.
- **Capsules and large concentric radii.** Pills for controls, 24–30px radii for cards, circles for media buttons.
- **White ink at large sizes** on photographic scenes, dark ink on white frosted glass over pale UI. Secondary text at ~70% alpha, never blended.
- **Refraction only on small controls.** Lensing appears on buttons, knobs and pills, where the edge bend reads as thickness; cards stay frosted.

## Items

| # | Reference | Copy this |
|---|---|---|
| 01, 33 | Andrei Rybin, weather widget stack over a sunset photo ([savee](https://savee.com/i/umAE8J_/), [tall](https://savee.com/i/-xL0Vun/)) | Card at ~28px radius, warm-gray/white tint that takes the sky's pink, 1px light rim, big light-weight numerals with a small unit superscript, labels at ~70% white, metrics in a 2×2 grid, three stacked cards with decreasing height. |
| 02 | Rybin, hourly forecast over a green field ([savee](https://savee.com/i/kso00Sb/)) | On a pale backdrop the tint goes warm gray at higher alpha so white text survives; soft colored glow bands live inside the glass, not on top of text. |
| 03 | Apple, Liquid Glass lock-screen widgets on a blue wallpaper ([savee](https://savee.com/i/qDqOFh1/)) | Widget tint borrows the wallpaper hue; the dock is a translucent capsule; specular rim only on the lit side. |
| 06 | Volume slider concept over a photo ([savee](https://savee.com/i/XcLPePD/)) | Smoked dark capsule track with a bright refractive knob; label outside the track; 36px knob on a 52px pill. |
| 07 | Play/pause lenses over a poppy ([savee](https://savee.com/i/f6jk268/)) | Circular controls that bend the backdrop at the rim while the center stays clear; white icons with a faint drop shadow; the middle button ~1.5× the others. |
| 08 | WWDC-style glass controls on a light studio field ([savee](https://savee.com/i/kDvXJEs/)) | Toggle and slider knobs as clear lenses over saturated tracks; gray rim (not white) on a light backdrop; almost no fill. |
| 09 | "Ask agents" capsule on a gray gradient ([savee](https://savee.com/i/mV1vzHw/)) | Pill with an iridescent orb icon, bright top rim, faint chromatic fringe at the bottom edge, soft shadow; text bold white. |
| 10 | Minimal "Button" capsule on a deep blue gradient ([savee](https://savee.com/i/dH2_6ay/)) | Near-zero fill, all the material lives in the 1px rim and the shadow; label with a subtle light-to-dark gradient. |
| 11 | Blue tinted capsule with arrow ([savee](https://savee.com/i/8jSz3s0/)) | Brand-tinted glass: accent color at ~55% alpha, white 1px rim, glow shadow in the same hue. |
| 12 | White circular button with an iridescent halo ([savee](https://savee.com/i/nKmFQOv/)) | Light theme: white disc, gray hairline, diffuse shadow, a faint colored bloom behind the control instead of a colored fill. |
| 18 | Liquid Glass button close-up over a map ([savee](https://savee.com/i/aL3KCv-/)) | Edge lensing with a calm center; refraction strength scales with the bezel width, not the whole control. |
| 19 | Vision Pro acrylic control (physical object) ([savee](https://savee.com/i/KFdK3UP/)) | Thickness cues of real acrylic: a bright polished edge, embossed icons catching light, soft contact shadow. |
| 21 | Gray glass camera pill (jsngr) ([savee](https://savee.com/i/-wZgjrS/)) | On a mid-gray scene: white frosted pill with a darker gray rim on the shaded side and a 1px white inner highlight. |
| 22 | Frosted white search bar over a pastel bloom ([savee](https://savee.com/i/Vi52Urx/)) | White ~60% frost, dark ink, keyboard chip as a denser plate inside the glass, hairline divider between groups. |
| 25 | Light toolbar with a black tooltip ([savee](https://savee.com/i/XjfEpZs/)) | Hover state as a slightly brighter chip; the tooltip is solid black: not everything is glass. |
| 26 | "Ask Computer" input capsule ([savee](https://savee.com/i/ZVLgB5A/)) | White capsule, very diffuse large shadow, no visible border; placeholder at ~55%. |
| 27 | Embossed "Tab" capsule in a rail ([savee](https://savee.com/i/IYVtJfA/)) | Segmented control: the selected segment is a raised light capsule with an inner highlight inside a recessed track. |
| 31 | Fanned translucent cards render ([savee](https://savee.com/i/jae_Xl3/)) | Layered glass reads through color shifts and edge lines, not through repeated blur. |
| 32 | Pink glass blob study ([savee](https://savee.com/i/l5nqYb4/)) | Rim light plus a darker core; the specular is narrow and sits on the curvature, never a broad white wash. |
| 35 | Chrome-edged blobs with a warm reflection ([savee](https://savee.com/i/-BWTci1/)) | A single warm highlight on one side and a cool rim on the other: the light model is what makes the material readable. |

Items 04, 05, 13–17, 20, 23, 24, 28–30 and 34 were reviewed and dropped by the author (dark smoked panels, app screenshots, pure gradients); do not treat them as the bar.

## Reading the board against the lab

| Lab element | Reference | Difference to keep in mind |
|---|---|---|
| Weather stack | 01, 33, 02 | The lab's backdrop is a CSS gradient with blurred shapes, not a photo; over the sunset the tint is a smoked `56 50 58 / .66` (the reference card is also darker than its sky), over the pale field it is white frost with dark ink, and only over the deep gradient is it the light `255 255 255 / .16`. |
| Media buttons | 07, 18 | Refraction comes from a canvas-generated displacement map; it only renders in Chromium. |
| Capsule buttons | 09, 10, 11 | The iridescent orb is a radial gradient; chromatic fringe is not reproduced on capsules. |
| Search bar, dock | 22, 03 | `data-ink="dark"` switches the whole scene to white frost + dark ink on pale backdrops. |
| Slider, switch | 06, 08 | Native `input[type=range]` and a `role="switch"` button drive decorative glass knobs. |
