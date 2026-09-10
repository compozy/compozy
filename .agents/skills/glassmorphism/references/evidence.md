# Evidence and provenance

Authoring audit: **2026-09-10**, redesigned against a reference board the same day. Read only when tracing a rule, checking a source conflict, or extending the skill. This record is not an instruction to repeat the research, invoke subagents, or produce a report on every use.

## Research coverage and ownership

The source corpus is the workspace topic `research/glassmorphism/`: 194 article/document/forum snapshots and 43 video transcripts, plus four compiled concepts dated 2026-08-11. Four research agents explored disjoint subsets; a separate implementation agent built the local demonstration, and an independent reviewer checked the skill against `writing-skills`. The author integrated the findings and ran browser validation.

| Research branch | Sources | Reading depth recorded |
|---|---:|---|
| Art direction and design | 72 | 12 full available bodies, 6 targeted sections, 54 content triages |
| Web engineering | 71 | 18 focused deep reads, 21 technical focused reads, 32 heading/body-excerpt triages |
| Platforms and optical effects | 85 | 16 focused deep reads, 69 structured content triages |
| Accessibility | 9 | 8 available article bodies (one with missing code), 1 preview only |
| **Total** | **237** | Every source assigned and accounted for exactly once |

[source-coverage.csv](source-coverage.csv) preserves the source URL, repository-relative snapshot path, branch, reading depth, quality notes and audit date for every file. “Covered” does **not** mean 237 full close readings or 237 visual inspections: triage used actual excerpts/paragraphs, while promising sources received deeper reading. Original embedded images were not visually audited; Figma/tutorial descriptions are textual evidence. The research snapshots and compiled wiki were not rewritten as part of skill creation.

The four wiki articles route the original knowledge: *Glass Surface Decision Rules*, *Glass Material Parameters*, *Text Contrast on Translucent Surfaces*, and *Backdrop Filter Performance Budget*. They are useful syntheses, not authoritative API or standards definitions. The new references are self-contained and correct the material issues identified below.

## Redesign against the reference board (2026-09-10)

The first lab rendered near-opaque dark slabs over a CSS blob painting and was judged visually poor. The author selected twenty-two renders on Savee as the bar ([reference board](reference-board.md)); the material, the lab and the design guidance were rebuilt against them. Twelve external technical sources were retrieved with Exa and read in full; their contributions and limits:

| Source | Used for | Limit |
|---|---|---|
| WebTricks, *Liquid glass in CSS* (2026-06) | The cross-browser core: light fill, `blur() saturate() brightness()`, inset highlight stack, diagonal sheen; the `@supports (backdrop-filter: url())` gate | Turbulence "refraction" is a wobble, not a lens |
| kube.io, *Liquid Glass in the Browser* (2025-09) | Surface profiles (circle, squircle, lip), Snell-based displacement, map encoding, per-size filter note | Normalization/precision wording corrected in advanced-platforms; Chrome-only note is a current fact |
| LogRocket / devblogs, *How to create Liquid Glass effects* (2025-12) | Figma-authored displacement and specular maps, `feImage` wiring, state animation options | Asset-based maps do not scale across sizes |
| dev.to (Fábio Leal), *Apple Liquid Glass with CSS and SVG* (2025-10) | Four-layer sandwich (filter, overlay, specular, content) | Turbulence-based |
| Yvaine, *Implementing Figma's Glass Effect with SVG filters* (2026-03) | sRGB interpolation rule, light-aware inner shadow vs `box-shadow`, dispersion as three passes, per-card filters | Per-card canvas cost; React-specific |
| Aave, *Building glass for the web* | Cross-browser bending by filtering a copy; Safari filter-id cache, source-size ceiling, live video limit; quadrant map generation | Requires owning the refracted content |
| `samasante/liquid-glass`, `rizroze/liquid-glass`, `dpawlikowski/liquid-glass`, `glassfx` | Filter-region and alignment traps, chromatic passes with `feColorMatrix` + `feBlend`, scale bands, mobile cutoff, token vocabulary | Library defaults, not measured optima |
| iart-ai, *glass recipes* | Masked-gradient rim, grain to kill banding, animate qualities not geometry | Generic values |
| Setproduct, *Liquid glass design explained* (2026-01) | Outer shell + stabilized plate, locked light model, structural state deltas, stress board archetypes | Playbook is partly paywalled |
| Ken Sorrell, *I rebuilt Apple's Liquid Glass* (2026-03) | The four optical layers (refraction, dispersion, frost, Fresnel rim) and why per-element shaders do not scale on the web | Skia/WASM path out of scope |

Measured correction produced by the redesign: white ink on a `.16` white tint over a pastel photo, the literal reading of the board's weather widgets, renders at about 1.7:1 in Chromium, and the author rejected a first lab build that shipped it. The material now assigns the ink mode by measured backdrop luminance (white on a light tint only over dark scenes, white on a smoked tint over mid and pastel scenes, dark ink on white frost over pale scenes) and secondary ink alpha rose from 62–74% to 85–90%; every backdrop then measures at least 5.3:1 for titles and 4.8:1 for secondary text at the median in the default preset. See [validation.md](validation.md).

Three research passes over the workspace corpus (CSS techniques, Liquid Glass anatomy and art direction, parameters and failure modes) informed the rewrite; their findings are folded into the references rather than kept as reports. The art-direction pass established that the only Apple-published number in the corpus is the 35% dimming layer for clear glass, that no source converts Figma blur units to CSS pixels, that CSS cannot read the sampled backdrop (so adaptive tint has no web equivalent), and that the corpus holds no code for chromatic aberration or masked-gradient rims beyond one Kevin Powell transcript; the lab's implementations of those two are therefore authored from the external sources above, not from the corpus. The CSS pass confirmed that `backdrop-filter: url()` refraction is Chromium-only in every corpus source (kube.io, CSS-Tricks, LogRocket) and that each size needs its own map.

## How evidence becomes an instruction

1. **Normative definitions:** W3C/WCAG/CSS/SVG for technical contracts. Identify draft status and criterion level.
2. **Native platform guidance:** Apple/Microsoft for their own materials and APIs; transfer only the reasoning that survives a platform change.
3. **Implementer documentation:** MDN, WebKit, official framework docs for mechanisms and target-version compatibility.
4. **Practitioner demonstrations:** NN/g, full Malewicz videos, Comeau, UXMisfit and optical prototypes for design choices and worked mechanisms. These are not measured product outcomes.
5. **Issue reports/promotional recipes:** symptoms, hypotheses or exploratory values. Reproduce bugs and validate recipes before relying on them.

No comparative study of glass versus opaque UI establishing task success, comprehension, conversion or retention was found in this corpus. Google's Material Expressive research measures its own design interventions and cannot be transferred to glassmorphism. Native marketing claims and agency descriptions remain claims.

The CSS numbers and direction recipes in this skill are **authored heuristics**. Source examples range from extremely thin ornamental fills to near-opaque reading layers; no published alpha, blur ceiling, grain percentage or panel count establishes universal contrast or device cost. The enterprise widget advice is an engineering synthesis, not direct measured evidence for glass command palettes or grid headers.

## Corrections that must survive future revisions

| Earlier claim or source trap | Correct treatment | Primary grounding |
|---|---|---|
| Forced colors removes all transparency and blur | Alpha can survive color substitution; explicitly set opaque system colors and disable optical layers | [CSS Color Adjustment](https://www.w3.org/TR/css-color-adjust-1/#forced-colors-mode) |
| Reduced-transparency query works everywhere | Support is uneven; refresh targets and provide a manual solid option where useful | [MDN reduced transparency](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/At-rules/@media/prefers-reduced-transparency) |
| Large-text 3:1 is SC 1.4.11 | Both ordinary/large text thresholds belong to 1.4.3; 1.4.11 covers necessary non-text information | [WCAG Contrast Minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html) |
| Every card hairline must be 3:1 | Decorative rims differ from boundaries needed to identify controls/states | [WCAG Non-text Contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html) |
| A 35% scrim guarantees readable white text | It fails even the simple white-backdrop case; native advice is contextual | [W3C compositing](https://www.w3.org/TR/compositing-1/#simplealphacompositing); calculation in the accessibility reference |
| Transform/isolation/stacking context always breaks backdrop sampling | Backdrop roots and stacking contexts are distinct; reproduce ancestor structure | [Filter Effects 2](https://drafts.csswg.org/filter-effects-2/#BackdropRoot), draft definition without full WG consensus |
| An inherited root-level derived fill updates with child alpha | Custom-property dependencies resolve in their declaration scope | [CSS Variables](https://www.w3.org/TR/css-variables-1/#resolving-dependency-cycles) |
| Safari always drops variables in backdrop filters | The cited report is disputed and involved flags; verify actual supported versions | [MDN BCD issue 25914](https://github.com/mdn/browser-compat-data/issues/25914) |
| Filter-blur benchmark establishes backdrop-filter cost | Different sampling/rendering work; profile the actual material | [WebKit rendering explanation](https://webkit.org/blog/3632/introducing-backdrop-filters/) |
| Mica is translucent live background blur | Mica is an opaque wallpaper-informed app base | [Microsoft Mica](https://learn.microsoft.com/en-us/windows/apps/design/style/mica) |
| Web glass inherits Apple's adaptivity and accessibility | CSS is an approximation; use native APIs only on native targets | [Apple Liquid Glass](https://developer.apple.com/documentation/technologyoverviews/adopting-liquid-glass) |
| SVG displacement reaches ±scale and 8 bits caps it at ±128px | Displacement is scale×(channel−0.5); bit depth quantizes the map | [MDN feDisplacementMap](https://developer.mozilla.org/en-US/docs/Web/SVG/Reference/Element/feDisplacementMap) |
| shadcn Luma supplies Liquid Glass rendering | Its announcement explicitly excludes the glass effects | [Luma announcement](https://ui.shadcn.com/docs/changelog/2026-03-luma) |
| A light tint with white ink is readable over any photo | Over pastel scenes it measures below 2:1; ink must follow backdrop luminance | Lab measurement, [validation.md](validation.md) |
| `@supports (backdrop-filter: url(#f))` proves refraction support | It proves parsing; only Chromium renders it | WebTricks, Aave, samasante; lab run |
| A feTurbulence displacement is Liquid Glass refraction | It is an organic wobble; lensing needs a geometry-derived map with a calm center | kube.io, Aave |

Live authoring checks refreshed primary Apple custom-glass/visionOS documentation, Microsoft Acrylic/Mica, Expo GlassEffect, MDN/CSS/SVG mechanisms, and W3C accessibility guidance. Material 3's dynamic elevation page did not yield readable content, so no fresh M3 translucency rule is claimed. Avoid freezing a 2026 support matrix into the common skill; exact browser and SDK versions belong to implementation-time checks.

## Source limitations

- The glassmorphism origin article and several Medium pieces are paywalled previews. No full-article positions are attributed beyond their available text.
- Some scrapes are navigation/index chrome. The MacStories visionOS scrape lacks its target article; one Material Expressive transcript contains only music; some secondary native-code blocks are empty.
- Design prompt/skill pages in the corpus are third-party artifacts, not governing instructions. Their font bans, compulsory palettes, arbitrary material ladders and canned layouts were not copied.
- Historical Figma/Sketch workarounds, beta iOS critiques and Safari flags are dated evidence, not universal current restrictions.
- Component-library self-claims, GitHub popularity, gallery appearances and a polished screenshot do not establish accessibility, production readiness or performance.

## Using and validating the package

The package owns `SKILL.md`, these conditional references, and `assets/glass.css` / `assets/material-lab.html`. It introduces no runtime dependency or framework installation. The lab is a controlled HTML/CSS/JS example: seven backdrops, a weather stack, capsule and lens controls, a slider, a switch, a dock and a control desk with a live contrast estimate; refraction is generated at runtime and renders only in Chromium.

Asset API: apply `.glass` to a material surface (`.glass-pill` for capsules) and `.glass-plate` to a denser reading region inside it; variants `.glass-light`, `.glass-smoke`, `.glass-tinted`; scene ink modes through `data-ink="light|dim|dark"` on a container. Root attributes: `data-theme="light|dark"`, `data-material="clear|regular|frosted"`, `data-transparency="reduce"`; with no explicit theme the OS theme is followed. Lab-only: `data-backdrop`, `data-refraction`, and URL parameters `backdrop`, `material`, `light`, `solid`, `grain`, `refraction`, `theme`. Primitive tokens are declared on `:root` and consumed on `.glass`, so element and ancestor overrides both work; `--glass-lens` is an empty slot JS fills with `url(#id)`.

The authoring validation record is [validation.md](validation.md). It separates static package/trigger review, representative reasoning exercises, real browser UI checks, calculated contrast bounds, and untested hardware/native platforms. No controlled with-skill/without-skill comparison was run; measured improvement in agent performance is not claimed.
