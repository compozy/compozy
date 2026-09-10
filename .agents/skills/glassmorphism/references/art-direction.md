# Art direction and materials

Read when naming the target look, judging a result against the bar, choosing tokens, fixing flat/muddy/plastic glass, or handing a design to implementation. The quality bar is the [reference board](reference-board.md); the numbers below are the lab's tuned values against those renders, not vendor specifications or measured usability optima.

## Compose before blurring

Glass is relational: the same panel reads differently over a sunset photo, a pale field, a studio gray, or a near-white UI. Arrange the actual backdrop, the material plane and the readable foreground together, and keep exposed environment around every surface so the layering stays visible. A tonal edge passing under a decorative part of a panel shows the frost; detail under text only competes with it.

The backdrop must carry light. In the board every reference sits on a photograph, a wallpaper-like gradient with a luminance ramp, or a neutral field with a bloom. A flat fill gives the blur nothing to sample; the result is a gray box with a border. When the product owns its backdrop, compose one: a base gradient with two or three large blurred shapes and, optionally, static grain at a few percent. When it does not, bound what the material can sit on, or protect content with a plate.

Practitioner rules from the corpus that the board confirms (`raw/youtube/glassmorphism-best-practices-tutorial.md`, `raw/youtube/glassmorphism-2-0-full-style-breakdown.md`, `raw/articles/12-glassmorphism-ui-features-best-practices-and-examples.md`):

- Leave a margin; "avoid cards that are full screen." A full-bleed panel "reads as a gray smudge."
- Place the panel where the backdrop changes: "one of the edges actually starts where the glassmorphic panel starts"; centered on an already blurry region "the effect is barely visible."
- Blur "until a little bit of that edge is still visible" (Malewicz works at 5–10 Figma units); keep the backdrop legible as shape and color, never as detail.
- Gradients diagonal, "top left corner… bottom right corner… most of the gradients in nature are simply like that."
- Avoid high-frequency noise in the source image; smooth color fields work, busy text behind a 12% white fill "can swing from 8:1 contrast to under 2:1."
- NN/g's opposite advice (more blur, simple backgrounds) answers legibility; the practitioners answer whether it reads as glass. The synthesis (`wiki/concepts/Glass Surface Decision Rules.md`) is a controlled backdrop with gentle tonal variation, neither an uncontrolled photo nor a flat fill.
- Stress the recipe on "bright photo, dark photo, textured, saturated gradient, plain white" plus a hotspot and a shape conflict.

Use the product's type, spacing, iconography and palette. No font, purple/cyan palette, bento grid, animated orb or giant radius is inherent to glass. Before polishing, look at the interface with the solid toggle on: size, proximity, alignment and emphasis must still explain it.

## What Apple says the material is

The corpus's Apple sources (WWDC25 *Meet Liquid Glass*, `raw/youtube/wwdc25-meet-liquid-glass-apple.md`; HIG Materials and Color; *Adopting Liquid Glass*) define the target in a few sentences worth keeping verbatim:

- **Lensing, not scattering.** "Where as previous materials scattered light, this new set of materials dynamically bends, shapes, and concentrates light in real time." Controls "feel ultra lightweight and transparent while still being visually distinguishable." Elements "materialize in and out by gradually modulating the light bending," not by fading.
- **Three named layers:** highlights "that respond to geometry" and travel around the silhouette; a shadow that "increases the opacity of its shadow when it is over text" and lowers it "over a solid light background"; illumination that "illuminates from within as a form of feedback."
- **No color of its own.** "Liquid Glass has no inherent color, and instead takes on colors from the content directly behind it." Small elements flip between light and dark by the background; "bigger elements, like menus or sidebars… don't flip," and glass "appears more opaque in larger elements like sidebars to preserve legibility." A solid fill "is completely opaque and breaks the visual character."
- **Size sets thickness.** Larger glass "simulates a thicker, more substantial material… deeper, richer shadows… more pronounced lensing." Nearby color "can subtly spill onto its surface."
- **Regular and Clear "should never be mixed."** Regular carries legibility, "anything can be placed on top of it." Clear "needs a dimming layer"; the only Apple number in the corpus: "If the underlying content is bright, consider adding a dark dimming layer of 35% opacity."
- **Layering.** Glass is "a distinct functional layer for controls and navigation… that floats above the content layer"; "don't use Liquid Glass in the content layer"; "always avoid glass on glass… use fills, transparency, and vibrancy for the top elements" (glass "can not sample other glass"); "limit these effects to the most important functional elements"; at rest "avoid intersections between content and Liquid Glass."
- **Geometry.** Capsule is the default glass shape; rounded rectangles only for larger components; corners "concentric to their containers." Controls in a context "morph between" each other on "a singular floating plane."
- **Scroll edge.** Content scrolling under glass "gently dissolves… into the background," switching to "a subtle dimming" over dark content. Glass chrome does not carry its own legibility; a header needs its gradient scrim.

Apple publishes almost no other numbers, and CSS cannot read the sampled backdrop, so adaptive tint has no web equivalent; the lab's scene-level ink modes and the plate are the deterministic substitutes. Apple itself raised chrome opacity after the first iOS 26 beta, added a tinted mode in 26.1 and reduced default transparency at WWDC 2026 (`wiki/concepts/Text Contrast on Translucent Surfaces.md`): adaptivity narrows the contrast distribution but never bounds it, so a scrim or a plate outranks adaptive tint for text.

## Material anatomy

| Lever | Perceptual job | Lab token | Tuning decision |
|---|---|---|---|
| Backdrop | Light and color behind the surface | scene | Choose crop/placement and the permitted range of user content |
| Tint and fill alpha | Color cast and suppression of detail | `--glass-tint`, `--glass-alpha` | Thin and light over photos; warm gray on pale scenes; white frost with dark ink on near-white UI |
| Blur, saturation, brightness | Detail suppression; color survives the blur; a slight lift | `--glass-blur`, `--glass-saturate`, `--glass-brightness` | `saturate(1.4–1.7)` keeps the scene vivid through the frost; brightness above 1.1 washes |
| Rim | Boundary and apparent thickness | `--glass-rim`, `--glass-rim-near/-far`, `--glass-light` | 1px gradient ring, bright on the lit edge, quiet opposite; gray (not white) on light backdrops |
| Inset highlight | Bevel | same light angle | 1px inset on the lit edge at ~.5 alpha, an opposite 1px at ~.14 |
| Sheen | Gloss falling off from the lit edge | `--glass-sheen` | Narrow and directional; a broad wash reads as plastic |
| Shadow | Contact and elevation | `--glass-shade`, `--glass-elevation` | Tinted by the scene, displaced away from the light, large blur, low alpha |
| Grain | Frost texture, banding relief | `--glass-grain` | Optional, static, ≤4–6% |
| Refraction | Edge lensing | `--glass-lens` | Small controls only; calm center, bend at the bezel |

Declare `--glass-light` once and let every cue derive from it. A brighter top-left rim pairs with a shadow pushed to the bottom-right; random highlights are the fastest route to CGI.

## Ink follows the backdrop

The board contains three legible pairings and nothing in between:

| Scene luminance (median behind the surface) | Material | Ink | Lab preset and measured title contrast |
|---|---|---|---|
| Dark: below ~.1 (deep gradient, night wallpaper) | White tint `.07–.34` | White; secondary at 90% | `data-ink="light"`: 7.6:1 regular, 10.8:1 clear |
| Mid to pastel: ~.1–.6 (sunset, poppy, most photos) | Smoked neutral tint `56 50 58` at ~.66 | White; secondary at 90% | `data-ink="dim"`: 5.7:1 sunset, 7.3:1 poppy |
| Pale to near-white: above ~.6 (hazy field, studio gray, pastel UI) | White frost `.42–.80` | Near-black `#1b2129`; secondary at 85% | `data-ink="dark"`: 14–16:1 |

Apple's material performs this switch itself. On the web decide it per scene, or measure the backdrop and pick. The board's weather widgets look like the first row but sit on the second: Rybin's card is visibly darker than the sky around it, a smoked tint that lets the sunset through as color. The literal first-row recipe (white ink on a `.16` white tint) over the same pastel sunset measures about 1.7:1 in the lab, so the lab never ships it on a pastel scene. Secondary text on glass needs at least 85–90% ink alpha to clear 4.5:1; 60–75% reads as "muted" and fails.

## Direction recipes

Values are the lab's tuned CSS at intended size (1440px desktop); recalibrate for other sizes and device pixel ratios.

| Direction | Environment | Material | Foreground | Board |
|---|---|---|---|---|
| Widget stack on a dark wallpaper | Deep gradient, night photo | Tint `255 255 255 / .16`, blur 14px, saturate 1.6, brightness 1.05, rim .8, radius 28–30px, elevation 18px | White ink, 64px light numerals with a 20px unit superscript, labels 13px at 90% | 03 |
| Widget stack on a sunset or colorful photo | Sunset, poppy, most photos | Smoked tint `56 50 58 / .66`, blur 18px, saturate 1.25, brightness .98, rim .75, same geometry | White ink; the scene shows through as color | 01, 33 |
| Widget on a pale scene | Green field, hazy sky | White frost `255 255 255 / .62`, blur 18px, saturate 1.3, gray far rim | Dark ink | 02, 22 |
| Clear capsule control | Deep or studio gradient | Tint `.06–.07`, blur 6px, saturate 1.7, brightness 1.08, rim .95–1, sheen .12–.16, 48px tall pill | White label, faint text shadow on photos | 09, 10 |
| Tinted capsule | Any | `--accent-rgb / .55`, blur 12px, rim 1, shadow in the accent hue | White icon | 11 |
| Smoked control | Bright photo | Tint `16 18 24 / .38`, blur 12px, rim .7 | White label; bright knob `255 255 255 / .5` with rim 1 | 06 |
| Lens button | Any colorful photo | Tint `.10`, blur 4px, rim 1, radius 50%, refraction strength .24–.26 of the diameter | White icon with a 1px drop shadow | 07, 18 |
| Frosted white bar | Pastel or studio field | Tint `255 255 255 / .62`, blur 18px, saturate 1.3, rim .7 with a gray far rim `30 38 48`, diffuse shadow `.16` | Dark ink, kbd chip as a denser plate | 22, 21, 26 |
| Reading panel | Any | Clear/regular shell with a `.glass-plate` at `tint / .22` under fields and code; on light scenes a smoked shell `44 40 50 / .5` | Ink per shell | 25, 27 |

Thickness presets scale these together: clear `.07 / 6px`, regular `.16 / 14px`, frosted `.34 / 26px`; the dark-ink variants run `.42 / 10px`, `.62 / 18px`, `.80 / 28px`. Larger surfaces read thicker (the lab's desk runs `.4–.5` with a 36px blur while pills run `.06`), which matches Apple's size rule.

Corpus bands to sanity-check against (`wiki/concepts/Glass Material Parameters.md`): audited production blurs are Apple's nav 20px, Framer 10px, Reflect 4–22px, Resend 25px, Clerk 1–10px, so 8–24px for panels, 4–8px for pills, and never animated on scroll; `saturate(150–180%)` is the consensus lift; fill by role, `.10–.30` for chrome and decoration, `.55–.80` under body text; dark glass is a dark tint at higher alpha with a much fainter white edge (`rgba(17 25 40 / .55)` + `rgba(255 255 255 / .12)`), not light glass with less alpha; radius is the least standardized parameter in any source.

Malewicz's authored stack (`raw/youtube/glassmorphism-best-practices-tutorial.md`, `glassmorphism-2-0-full-style-breakdown.md`): find the scene's light source first and reverse every layer when it moves; fill white 20% over a photo, 10% over a dark one, upgraded to a radial gradient that is lighter and slightly more transparent toward the lit corner; border 1–1.5pt as a gradient from near-white to a slightly darker gray at ~50%, direction matching the fill; grain 1–3% ("anything under five is good"); shadow color sampled from the object and desaturated; saturation maxed under the blur because it "amplifies the lightness." His deep-glass inner-shadow stack, top to bottom: `y20 blur40 white 70%`, `y4 blur4 black 50% overlay`, `y16 blur8 white 40%`, `y1 blur8 white 70–100%`, with "blur double the Y value." Figma's own glass panel envelope (`raw/articles/apple-s-new-liquid-glass-design-practical-guidance-for-designers-designed-for-humans.md`): depth ≤20 for controls, frost 10–25 ("above 30 looks milky plastic"), specular amplitude ≤6px, blur ceilings 40px on phone and 60px on tablet/desktop, ≤4 compositing layers per screen. No source converts Figma blur units to CSS pixels; match in the renderer.

## Geometry and hierarchy

Capsules for controls, 24–30px for cards, circles for media buttons, and concentric inner radii (`inner = outer − inset`). Scale rim thickness with object size: 1px on a 48px control, still 1px on a 300px card; a 3px illustrated rim belongs to a hero object. One glass shell with tinted, non-filtered sections inside beats nested live filters; represent a stack with differentiated fills, rim strength and offsets.

Budget from the corpus's decision rules: one to three glass surfaces per view, the navigation layer counting as one however many edges it has, at most one transient sheet above it, zero stacked glass; "on a screen made of glass, the way to emphasize something is to make it solid." On dense screens, put material on navigation, limited floating controls and intentional overlays. Data hierarchy belongs to typography, grouping, spacing and state contrast. A rich hero can carry one sculpted lens; explanatory copy stays stable.

## Quality tells

Premium glass shows these; missing three or more usually means flat, muddy or plastic.

1. The backdrop has visible luminance variation through the surface.
2. The fill is lighter than the average scene, not a dark slab.
3. One light direction: rim, highlight, sheen and shadow agree.
4. A 1px rim whose brightness varies along the edge.
5. A thin inset highlight on the lit edge, nothing on the opposite edge beyond a faint line.
6. Shadow tinted by the scene and displaced away from the light.
7. Scene color survives the blur (saturation ≥ 1.3) and reads as tint, not detail.
8. Large, light-weight numerals or bold short labels; secondary text by alpha, not blend modes.
9. Capsule and concentric geometry; no square corners on glass.
10. Dense text sits on a plate; nothing important depends on a favorable backdrop.
11. Refraction, if any, bends only at the bezel of small controls.
12. Hover, press, focus and disabled change structure, not just tint.
13. A solid twin exists and looks designed.
14. The panel overlaps a feature of the backdrop (an edge, a color boundary, a light), not a bland region.
15. Larger surfaces read thicker than small controls; one shadow token for every size is a tell.
16. Exactly one tinted accent; the rest of the glass is monochrome.
17. Grain, if present, sits at 1–3%; visible grain reads as amateur, none reads as plastic.
18. A header over scrolling content carries its own gradient scrim; blur alone is not legibility.

## Sculpted or lensed hero as an optional study

Build the readable interface first. Then, for one hero object or a small tactile control: shell over a meaningful backdrop, localized radial fill, a broad soft inner reflection, a smaller opposite dark inner edge, one narrow bright inner edge, optional faint grain, and a cast shadow re-tuned to the scene. Check at rest, hover, focus and press. For actual backdrop displacement use [Advanced effects: web refraction](advanced-platforms.md#web-refraction-and-lensing); more shadows alone do not create refraction.

## Figma and handoff

Work over the actual frame with representative content. Background Blur frosts; Layer Blur belongs to decorative layers. Keep text and icons out of low-opacity groups. Represent materials as styles/components with theme and state variants, and record: material role and permitted backdrop, tint/alpha, blur, rim gradient and light angle, sheen, shadow, grain, ink pairs, which regions carry a plate, solid and forced-colors variants, and which details may degrade. The corpus provides no calibrated conversion from Figma blur values to CSS pixels; match appearance in the target renderer.

## Diagnose visual failure

| Visible failure | Material cause | Adjustment |
|---|---|---|
| Gray flat box | Flat backdrop, dark opaque fill, uniform border | Give the scene light; lighten and thin the fill; replace the border with the light-driven rim |
| Milky plastic slab | Broad sheen or bright fill without a crisp rim | Narrow the sheen, restore the 1px rim, lower fill |
| Muddy double glass | Overlapping filtered planes | One shell, tinted plates inside |
| Highlights everywhere | No light model | Set `--glass-light` once; derive rim, inset and shadow from it |
| Text washes out on a pale scene | White ink on a light tint | Switch to the warm-gray tint or to dark ink on white frost; measure |
| Grain reads as dirt | Amplitude too high | ≤4–6%, static, or remove |
| Effect fails on a new image | Tuned against a favorable still | Bound the backdrop or add a plate; run the stress board |
| Dark theme loses edges | Automatic inversion | Tune rim, fill and ink per theme |
| Icon vanishes on a lens | White icon over a bright lens region | Icon in the scene's ink with a 1px drop shadow; keep the lens center calm |
| Everything is glass, nothing reads as glass | Material used as a uniform | Keep glass on chrome and one hero; solid for data, forms and long text |
| Two panes meet edge to edge | Adjacent independent surfaces | One plane, or a visible gap; Fluent calls the seam "undesirable" |
| Toasts or banners unreadable on phones | Text on raw glass over live content | Solid or plated notification surface (the corpus's clearest shipped failure) |
| Halos around small glyphs | Tables, code or long reading on a translucent panel | Content layer goes opaque |
| "Is that text on the bar, or behind it?" | Transparent app bar with no defined edge | Define the edge with rim and plate, or go opaque |
| Hover lag, sluggish modal open | Backdrop blur on an overlay or scrim | Scrims dim, they do not blur |
| Critical information over motion | Text on glass above video or animation | Solid container for that content |

## Source anchors

- [Reference board](reference-board.md): the twenty-two selected renders and what each contributes.
- Apple, [HIG Materials](https://developer.apple.com/design/human-interface-guidelines/materials) and [Adopting Liquid Glass](https://developer.apple.com/documentation/technologyoverviews/adopting-liquid-glass): adaptive light/dark switching, lensing at the edge, glass for navigation and controls rather than content.
- Setproduct, [Liquid glass design explained](https://www.setproduct.com/blog/liquid-glass-design-explained-a-practical-guide): outer shell plus inner stabilized plate, a locked light model, thickness cues, structural state deltas, and the background stress board (calm, textured, hotspot, shape conflict, dark with accents).
- [NN/g: Glassmorphism](https://www.nngroup.com/articles/glassmorphism/), [UXMisfit: visual hierarchy](https://uxmisfit.com/2021/02/03/glassmorphism-guide-to-visual-hierarchy/) and Malewicz's practical videos (see [source coverage](source-coverage.csv)) for stroke, light and subtractive hierarchy.
- Corpus paths quoted above live under `research/glassmorphism/`: `raw/youtube/wwdc25-meet-liquid-glass-apple.md`, `raw/youtube/wwdc25-build-a-swiftui-app-with-the-new-design-apple.md`, `raw/articles/materials-apple-developer-documentation.md`, `raw/articles/color-apple-developer-documentation.md`, `raw/articles/adopting-liquid-glass-apple-developer-documentation.md`, `raw/youtube/glassmorphism-best-practices-tutorial.md`, `raw/youtube/glassmorphism-2-0-full-style-breakdown.md`, `raw/articles/how-to-create-glassmorphic-card-ui-design-uxmisfit-com.md`, `raw/articles/apple-s-new-liquid-glass-design-practical-guidance-for-designers-designed-for-humans.md`, and the wiki concepts *Glass Material Parameters*, *Glass Surface Decision Rules*, *Text Contrast on Translucent Surfaces*. Malewicz's origin article is paywalled in the corpus; cite the transcripts, not `glassmorphism-in-user-interfaces-squareplanet.md`.
