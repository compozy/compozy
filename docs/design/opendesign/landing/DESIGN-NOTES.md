# Landing v3 — design notes

Prototype of the redesigned `compozy.com` homepage, rebuilt 2026-09-01 after the v2 pass (2026-08-27) landed far below the bar: it read as a wireframe (labeled placeholder plates, spec text inside capture frames, an eyebrow on every section). v3 keeps the operator-locked IA and copy deck and rebuilds the visual layer so the page reads as the finished product would. Source of truth for structure and copy: `.compozy/tasks/landing-redesign/landing-structure.md` (v2, 12 sections) and `landing-copy.md`. Tokens: `packages/ui/src/tokens.css` values mirrored in the `:root` block of `landing.css`; site extensions from `packages/site/app/global.css`. Iterate on these files, never regenerate.

Reference discipline applied on top of the Compozy authorities: `taste-skill` (Leonxlnx) sections 4, 9 and 14 (hero stack discipline, eyebrow budget, zero em-dashes, real imagery over div-screenshots, section-layout variety, zigzag cap), `imagegen-frontend-web` (composition anchors and background modes vary across sections; two full-bleed moments; rich sections alternate with calm ones), `impeccable` craft floor, `ui-craft` anti-defaults.

## Revision 2026-09-10 — every raster pulled for regeneration

Operator feedback: the current images are not good enough; all of them will be generated again. Locked responses:

- Every `<img>` (12 slots) is now a `.ph` placeholder that keeps the slot's exact geometry (`.plate` 16:10, `.win__body` 16:10, `.uc__art` 16:9 / 21:9) and states, in place, the **name · slot ratio · one-line brief · the file it replaced**. The hero placeholder keeps `.plate__img` so the demo-tab pan logic in `landing.js` still runs (inert on a div).
- The six CSS-background rasters became hatch fills: `.ext__art`, `.bridges__canvas`, `.cta__art` carry a visible `.ph__tag` (content art); `.hero__wave`, `.pain__atmo`, `.loop__atmo` and the three `.dg__atmo` diagram backdrops are decorative atmospheres and are labeled only in `?annotate` mode (`data-stand-in` / `data-ph`).
- The `hero-poster.webp` preload is gone. No `assets/` reference remains in HTML, CSS, or JS. The files in `assets/` were **kept on disk** as reference for the regeneration pass and can be deleted once the new set lands.
- The pain-section animation (nine hand-assembled pieces collapsing into the CompozyOS block) is also a placeholder now (`.ph--motion` inside `.pain__stage--ph`); its `.parts`/`.core` CSS and the `[data-pain]` JS block stay dormant and leave with the rebuilt animation. The full brief for every image and animation to generate is `BRIEF-IMAGENS-ANIMACOES.md` (pt-BR, operator request).
- Regeneration brief per slot lives in the placeholder label itself; the asset map below records the previous sources. Remove the "image placeholders" chapter at the end of `landing.css` when the regenerated assets are wired.

| Slot | Ratio | Generate |
| --- | --- | --- |
| Hero demo poster | 16:10 | desktop with Loops + Tasks windows over dock and menubar (one poster, six tabs pan it) |
| Spot · Implement / Review / Briefing | 16:9 | daemon + task list · orbit rings around a run · scheduled job fanning out |
| Spot · Release / Gate | 21:9 | numbered run stream, current ringed · timeline paused at a gate |
| Capture · Session / Knowledge / Tasks / Jobs / Desktop shell / Loop run | 16:10 | real captures on a seeded lab at the routes named in each label |
| Extensions art · Bridges art · Closer art | panel fills | concept illustrations, see `.ph__tag` copy |
| Hero wave · traces · orbit · radar backdrops | atmospheres | low-opacity textures behind sections and diagrams |

## Revision 2026-09-10 (b) — visual polish pass

Operator feedback: with the rasters gone the page still read far below the bar (flat surfaces, few details, not visual enough). Pass applied with the four skills installed under `.agents/skills/` (`design-taste-frontend`, `high-end-visual-design`, `imagegen-frontend-web` for composition variety only, `redesign-existing-projects` for the audit and fix order). Placeholders untouched; nothing generated.

Design read: redesign in preserve mode (tokens, IA, copy deck and the Playfair / Geist / JetBrains stack stay). Dials `7 / 6 / 4` (motion +1: blur-up reveal, spotlight borders, one marquee). Eyebrows still three; accent still the download action plus the beta dot, with one new tint on the comparison header cell.

What changed, per layer:

- **Topnav**: a floating glass island detached 10px from the top (`.tn__island`, 14px radius, blur, tinted shadow); the gutters around it pass clicks through. On mobile the compact row sits inside the island; the GitHub icon hides ≤640px.
- **Surfaces**: one fixed grain layer (`body::before`, 5%, screen), section rules that fade at both ends, section padding raised to 96–140px.
- **Trays (nested shell + core)**: `--tray` (6px glaze ring + hairline) on the hero plate, every `.win`, the pain stage, the extensions and bridges panels, the comparison table, the desktop install panel and the logo grid. Use-case cards are now 6px shells around a 12px art core (18 / 12 concentric).
- **Buttons**: the trailing icon lives in its own well (`.btn__ico`), hover moves it, active scales to .98. Applied to every download and arrow CTA.
- **Segmented strips** (`.seg`): hero demo tabs, feature tabs, install tabs.
- **Title marks** (`.mark`, `.mark--sm`, `.mark--xs`): an icon tile above each section heading (plug · wrench · list-checks · quote · blocks · repeat · puzzle · waypoints · columns-3 · download), inside the eight feature-panel titles, the five use-case titles, the five Loops proofs and the three install steps. Seventeen Lucide symbols were added to the inline sprite (copied from `lucide-react` 1.27.0).
- **Hero**: italic emphasis on “already built.” (same family; line-height 1.08 for the descender), a dot field behind the copy, a lower reveal threshold so the plate top is visible in the first fold at 1440×900, and bottom padding (72–104px) so the demo caption no longer sits on the next section's hairline; the ember still clips at that hairline, which is the intended "lit from below" edge.
- **Atmosphere sections** (`.hero`, `.pain`, `.loop`, `.cta`) use `overflow:clip` instead of `overflow:hidden`: the oversized backdrops (the hero wave overflows by 12%) made those sections programmatically scrollable, so a keyboard focus or `scrollIntoView` could shift the whole atmosphere upward.
- **Providers**: the 26 logos as a 13×2 gapless hairline grid; tile wrap below 1024px.
- **Community → quote wall**: three columns drifting vertically (46 / 54 / 50s, the middle one reversed), edge fades, paused on hover; under reduced motion the first set lays out as three still cards. Card = photo slot · name · `Beta program` · source slot · hairline · quote. Only the three real quotes; the Vedovelli card uses the deck's optional second line. Photos: `.avatar[data-stand-in]` with initials until the portraits arrive. Source slot: quote glyph until the origin network of each quote is confirmed, then the network mark from the sprite.
- **Extensibility catalog**: real marks. GitHub and Linear come from the generated sprite; Context7 (context7.com glyph), Notion, Sentry, Stripe, PostHog, PostgreSQL, Supabase (Simple Icons) and Playwright (playwright.dev) are vendored mono in a second inline sprite (`cl-*`) in the HTML. For production add them to `packages/ui/src/logos` and regenerate `landing-logos.js`. Extensions and skills use the package / book icons.
- **Spotlight border** (`.spot`): the hairline lights up under the cursor on use-case cards, quote cards, catalog and bridge tiles; one delegated `pointermove` in `landing.js` feeds `--mx` / `--my`.

Verification: static (177 `<use>` refs resolve, every `aria-controls` / `aria-labelledby` resolves, zero em/en dashes in visible copy outside placeholder labels, three eyebrows, `node --check`), rendered with agent-browser Chromium at 1440×900 (first fold, plate, full page) and 390×844 (nav, hero, providers, use cases, wall, features, catalog, bridges, install), plus `?annotate`, the reduced-motion wall fallback and the island hit-test.

Open: portraits and source networks for the three quotes (the deck is names-only); the placeholder labels keep their em-dashes until the assets land.

## Revision 2026-09-10 (c) — generated set landed (12 of the 14 image slots)

The twelve generated slots from `BRIEF-IMAGENS-ANIMACOES.md` (items 2, 4 and 5) are wired; the placeholders that remain are the ones that depend on real captures or motion (hero poster + six clips, six feature captures, the pain animation, the three quote portraits).

- **Model and method**: `gpt-image-2.5-sunburst` through the `imagegen` skill CLI (`edit` endpoint, `quality=high`, `--no-augment`), every slot generated with the same two style references chosen by the operator: Dribbble 27555299 (Modular Infra, isometric matte modules) and Dribbble 27087150 (Minimal Stack, dark with one orange strip). Three variants per slot; the operator picked one each. Prompts and all 36 variants are in `/tmp/compozy-landing-gen/` (per-slot prompt files, `index.html` gallery, `manifest.json`); not committed.
- **Picks**: spot-implement v2 · spot-review v3 · spot-briefing v3 · spot-release v2 · spot-gate v1 · ext-cartridges v3 · bridges-inflow v2 · closer-shell v1 · hero-wave v1 · backdrop-traces v3 · backdrop-orbit v1 · backdrop-radar v1. Files overwrite the old same-named `.webp` in `assets/` (the previous stand-ins are gone).
- **Sizes**: the CLI limits models other than `gpt-image-2` to the legacy sizes, so the set is 1536×1024 (spots, bridges, closer, hero wave), 1024×1536 (extensions panel) and 1024×1024 (three backdrops). That is below the brief's floor (hero and captures ≥ 2400 px, spots ≥ 1600 px). Acceptable for the prototype; for production either upscale the chosen files 2× or regenerate the same prompts in `gpt-image-2` at the brief's sizes.
- **Wiring**: the five spots are `<img>` again inside `.uc__art` (`loading="lazy"`, explicit `width`/`height`, `--pos`/`--zoom` kept at neutral). The 21:9 cards crop the 3:2 renders through `object-fit`; the motif sits on the horizontal band so the crop holds. The three panels and four atmospheres are CSS backgrounds again (`.ext__art`, `.bridges__canvas`, `.cta__art`, `.hero__wave`, `.pain__atmo`, `.loop__atmo`, `.dg__atmo` via `--img`), with `mix-blend-mode:screen` and the opacities from the polish pass. The `.gen` orange badges used to confirm which slots would be generated are removed, as is the `.ph__tag` corner-tag CSS (no slot uses it any more).
- **Spot background pass (same day)**: the five picked spots rendered on a near-black floor (≈ 8–16 RGB) that read as a dark well inside the card (`--canvas-soft`, #1f1e1c). Each pick was re-run through the `edit` endpoint with itself as the edit target (`--input-fidelity high`, three variants) and one instruction: replace only the backdrop and ground with flat #1F1E1C, no vignette, keep every object, light and accent. Measured corners land at 30–33 / 29–32 / 26–30 on every variant; picks by closest match: implement v1, review v1, briefing v3, release v3, gate v1 (`/tmp/compozy-landing-gen/bg-pass/`, gallery with the originals side by side). `.uc__art` lost its `--rail` fill and hairline border and now carries the card glaze (`--surface-glaze` to transparent at 85%) instead of the bottom darkening, so the art sits on the card with no seam.
- **Extensions art as a backdrop (same day)**: the operator wants the extensions illustration to be the section's background, not a boxed panel. `.ext__art` left the grid: it is now absolutely positioned on `.sec--ext` (`overflow:clip`), bleeding from the viewport's right edge under the last five columns (`width:clamp(420px,46vw,700px)`, full section height, `cover`), with two intersecting masks so it dissolves toward the copy (transparent at the inner edge, solid from 42%) and at the top and bottom (12% fades). The copy moved to the first seven columns (`.ext__content{grid-column:1/8}`), the operator's call after seeing the left-hand version. No border, radius, tray or overlay. The render's floor was re-edited to the page color (#171615, same edit pass as the spots, `input_fidelity` is not accepted by this model so the edit runs without it); variant 1 measured 23,22,20 on every edge and is the one wired. Below 1024px the art returns to the flow above the copy (4:3, bleeding into the wrap gutters, fading out at the bottom and both sides).
- **Verification**: agent-browser Chromium at 1440×900 (hero, use cases incl. the two wide cards, pain, loops, extensibility, bridges, closer, Gateway and Workspaces diagrams) and 390×844 (hero, use cases, extensibility, bridges, closer); zero broken images, five spot `<img>` present, no `class="gen"` or `ph__tag` left in the HTML.

## Files

| File | Role |
| --- | --- |
| `landing-page.html` | The page: skip link → topnav → 12 sections → footer. `data-od-id` on every section, heading, CTA, tab strip and repeated card. Opens with the direction contract (THESIS · OWN-WORLD · STORY · FIRST VIEWPORT · FORM · FINISH). A Lucide subset is vendored as an inline `<symbol>` sprite (no CDN swap), plus a second inline sprite (`cl-*`) for the catalog marks not yet in `@compozy/ui/logos`. |
| `landing.css` | `:root` token mirror + components. No color literal outside `:root` (masks use `#000` only as a mask alpha stop). One reveal grammar (`[data-reveal]`), one authored motion (the DIY-stack collapse). |
| `landing.js` | Reveal-on-scroll (once), ARIA tabs (hero demo · features · install), hero demo auto-advance with per-tab plate pans, OS auto-detect for the download CTAs, copy buttons, DIY-stack collapse + replay, install step renumbering, `?annotate` mode. No scroll listeners. |
| `landing-logos.js` | SVG sprite rendered from `@compozy/ui/logos` + `Logo` (26 providers, 8 bridges, `cz-logo`, `cz-symbol`). Regenerate, never hand-edit (recipe at the end). |
| `assets/` | Reused imagery, converted to WebP (see the asset map). `hero-poster.webp` is the real OS-shell capture with the desktop wallpaper margins cropped off; `capture-loops-window.webp` / `capture-tasks-window.webp` are exact window crops of it. |

## Direction contract

Register **brand** (PRODUCT.md override for `packages/site`). Mode **Persuade**. Dials: `VISUAL_VARIANCE 7` (asymmetric hero, bento, image-as-canvas band, spotlight plate; still system-aligned) · `MOTION_INTENSITY 5` (reveal grammar + one authored moment + demo progress; every animation names its job) · `INFORMATION_DENSITY 4`. Accent budget per viewport: the download action (hero, install, final CTA) and the nav beta dot; the DIY-stack core block borrows the ember as its rim glow at the end of the collapse. Eyebrows: three on the whole page (hero category label, `Built in`, `Built to be built on`); every other section carries its heading alone.

Scene: a senior engineer at 11pm, laptop at 60% brightness, tabs full of agent CLIs they keep re-wiring by hand, arrives from a GitHub link and wants to know in ten seconds whether this replaces the pile and how to install it.

## Section map

| # | Section | `data-od-id` | Layout family | Visual |
| --- | --- | --- | --- | --- |
| 1 | Hero | `hero` | left-led copy + full-width plate | wave terrain (`hero-wave.webp`, real site asset) + CSS ember; six demo tabs over the real shell capture; `Loop editor` and `Tasks inbox` pan the plate onto their window |
| 2 | Providers | `providers` | stacked head + gapless logo grid | 26 real logos from the sprite as a 13×2 hairline grid at desktop (tile wrap below 1024px) |
| 3 | The DIY agent stack | `pain` | split copy / stage | nine dashed "parts" in a tray collapse into one CompozyOS block (the one authored motion); reduced motion: static parts → arrow → block |
| 4 | Use cases | `use-cases` | bento 3 + 2 | five spot images (stand-ins, see asset map) |
| 5 | Community | `community` | centered head + quote wall (three drifting columns) + proof strip | the page's one marquee; photo and source slots are stand-ins; live counts are skeleton bars |
| 6 | Features | `features` | tab chips + split panel | eight window frames: 3 illustrations, 2 real captures, 3 SVG diagrams |
| 7 | Loops | `loops` | spotlight: full-width plate + five-column proof strip | the real Loops window crop + Needs-you strip; orbit backdrop |
| 8 | Extensibility | `extensibility` | tall image + content column | cartridge illustration; real `extension.json` (trimmed); catalog chips; SDK row |
| 9 | Bridges | `bridges` | image-as-canvas band + tile row | `bridges-inflow.webp` under a tonal overlay; eight real brand tiles; caveat verbatim |
| 10 | Comparison | `comparison` | table | CompozyOS column on `--elevated` with the symbol; `✓` / `Partial` / hairline dash |
| 11 | Install | `install` | centered head + tabs + hairline steps | Desktop · Installer · npm · Go; npm/Go reveal the bootstrap step and renumber |
| 12 | Final CTA | `final-cta` | full-bleed closer | `closer-shell.webp` masked at right, ember at left, copy bottom-left |

No two consecutive sections share a layout family more than twice (6 and 7, 8 and 9 are the only adjacent split-like pairs), the page has two full-bleed moments (hero, closer), and rich sections alternate with calm ones (providers, community, comparison, install).

## Asset map (what is final, what stands in)

Rule from the operator: placeholders only for images that will be generated; reusable site images and SVG illustrations ship as final. Every stand-in carries a `data-stand-in="…"` attribute naming what replaces it; open the page with `?annotate` to see them outlined and labeled in place.

| Placement | File | Status |
| --- | --- | --- |
| Hero backdrop | `hero-wave.webp` (generated 2026-09-10, `gpt-image-2.5-sunburst`, variant 1) + CSS ember | final for the prototype; 1536×1024, upscale or regenerate larger for production |
| Hero plate poster | `hero-poster.webp` (real capture, margins cropped) | stand-in for the six demo clips + posters; one poster serves all six tabs, two tabs pan it |
| Use-case spots ×5 | `spot-implement.webp`, `spot-review.webp`, `spot-briefing.webp`, `spot-release.webp`, `spot-gate.webp` (generated 2026-09-10, one family, two Dribbble style references; floor re-rendered to the card color #1f1e1c) | final for the prototype; 1536×1024 each, the two wide cards crop to 21:9, no frame around the art |
| Features · Sessions | `feature-sessions.webp` (site `everything/` session timeline, own chrome cropped) | stand-in for the `/agents/$name/sessions/$id` capture |
| Features · Memory | `feature-memory.webp` (`bento/memory-v1.png`) | stand-in for the `/knowledge` capture |
| Features · Tasks | `capture-tasks-window.webp` (real) | stand-in for `/tasks?mode=kanban` (list view shown) |
| Features · Automation | `feature-automation.webp` (site `everything/` trace + events) | stand-in for the `/jobs` capture |
| Features · Desktops | `hero-poster.webp` (real) | final-grade: the live shell |
| Features · Profiles / Gateway / Workspaces | inline SVG diagrams over `backdrop-orbit.webp` / `backdrop-radar.webp` / `backdrop-traces.webp` (generated 2026-09-10, 1024×1024) | diagrams are stand-ins for the three settings captures and show only truthful state (three switches off, no invented ids or counts); backdrops final |
| Loops plate | `capture-loops-window.webp` (real) | stand-in for a `needs-approval` run at `/loop-runs/$runId`; the Needs-you strip is HTML (stand-in for `LoopRunNeedsYouCard`) |
| Extensibility | `ext-cartridges.webp` (generated 2026-09-10, 1024×1536; floor re-rendered to the page color #171615) | final for the prototype; section backdrop, not a panel |
| Bridges band | `bridges-inflow.webp` (generated 2026-09-10, 1536×1024) | final for the prototype |
| Final CTA closer | `closer-shell.webp` (generated 2026-09-10, 1536×1024) | final for the prototype |

Rejected for reuse: anything carrying the legacy `agh` name (`bento_grid.png`, `memory-dream-landing-v1.png`, the workspaces illustration with `.agh/`), the `playbook.yaml` illustration (banned vocabulary), `deploy-staging.skill.md` art (fictional asset the plan deletes), the docs storyboard set (cream paper, off-theme), and `hero.png` / `hero_illustration.png` (their protocol-kind chips would re-introduce Network semantics the homepage limits to one sentence).

## Authorized deltas from the copy deck

- **Em-dashes and en-dashes removed from visible text** (taste-skill 9.G). `[draft]` strings were re-punctuated with periods, colons or parentheses; `[lock]` and `[kept]` strings were untouched except `Memory, automation, … — core objects…` (kept-trimmed) which now uses a colon. The `<title>` keeps the canonical dash.
- **Hero stack held to four text elements**: category label, headline, subhead, actions (Download + docs + the one-liner chip). The beta pill (the nav already shows Beta), the installer caption and the platform microcopy left the hero; the platform line lives in the install section.
- **Eyebrows budgeted to three** (PRODUCT.md anti-reference: eyebrow-on-every-section scaffolding). `The problem`, `Use cases`, `Community`, `Loops`, `Bridges`, `Side by side`, `Getting started`, `CompozyOS beta` are not rendered; their headings carry the sections.
- **Community provenance** reads `Beta program, Dec 2025 to Feb 2026` (open item 1 still decides the public wording).
- **Middle dots rationed**: proof strip items are hairline-separated instead of `·`-joined; `Local-first, no telemetry`.
- Install-tab download button stays the default (glaze) button; the accent primaries are the hero and the closer.
- Go install pin shows `v0.3.0-beta.21`; production reads the release tag from the changelog source. Provider count `26` is `BUILTIN_PROVIDER_COUNT` in production.
- Comparison footnote keeps the required `<date>` placeholder (open item 7).

## Verification (2026-09-01)

- Static: every `<use href>` resolves (126 refs, icon sprite + logo sprite), every `aria-controls` / `aria-labelledby` / anchor resolves, zero em/en dashes in visible text, three eyebrows, no color literal outside `:root`, `node --check landing.js` clean.
- Rendered with Playwright Chromium at 1440×900 and 390×844 (full page + first viewport + feature-tab, install and hero-tab states). Fixed from that pass: atmospheres painting over copy (paint order), horizontal overflow at 390px from grid items with code blocks (`min-width:0`), tab chips shrink-clipping in scroll strips, illustration chrome doubling the window frame (crops), the illustration's replay control showing inside the release card, the collapsed DIY stage reading as empty space (tray), and the mobile bridges / closer compositions.
- Second pass (bare frames for real captures, focus rings as `outline` so component shadows cannot hide them, crop retunes) re-rendered and checked at both widths, plus `?annotate`, reduced-motion and keyboard-focus states.
- A fresh-context finish reviewer was spawned with the renders and the direction contract; it did not return within the session window, so the finish critique above is the in-thread pass (disclosed, not a reviewer verdict).

## Handoff for demo production and image generation

- Six hero clips ≤40s + posters, recorded on a lab seeded through real product paths; tabs already carry `data-route` and `data-caption`; the plate pans (`data-pos` / `data-zoom`) become per-clip posters. Home ships only with a seeded lab, else five tabs.
- Eight feature captures replace the frames in `#fp-1 … #fp-8`; keep the 16:10 body, drop the `data-stand-in`.
- Generated set (`imagegen-frontend-web` discipline, ink + `#E8572A`, deck-wave motif, no purple, no people, no real screenshots): hero full-bleed, five use-case spots (16:9 ×3, 21:9 ×2), one extensibility concept (4:5), one closer (≥2400×1000). The current stand-ins set the crop and tone.

## Regenerate the logo sprite

Run from inside `packages/ui` (so `react`/`react-dom` resolve from the monorepo), then delete the temp file:

```tsx
// packages/ui/.gen-logos.tmp.tsx — bun run .gen-logos.tmp.tsx
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import * as L from "./src/logos/index";
import { Logo } from "./src/components/custom/logo";
const providers: Array<[string, any, any?]> = [
  ["claude", L.ClaudeLogo], ["codex", L.OpenAILogo, { mode: "dark" }], ["gemini", L.GeminiLogo],
  ["opencode", L.OpenCodeLogo], ["copilot", L.GithubLogo], ["cursor", L.CursorLogo], ["kiro", L.KiroLogo],
  ["pi", L.PiLogo], ["blackbox", L.BlackboxLogo], ["cline", L.ClineLogo], ["goose", L.GooseLogo],
  ["hermes", L.HermesLogo], ["junie", L.JunieLogo], ["kimi-cli", L.KimiLogo], ["openclaw", L.OpenClawLogo],
  ["openhands", L.OpenHandsLogo], ["qoder", L.QoderLogo], ["qwen-code", L.QwenLogo], ["openrouter", L.OpenRouterLogo],
  ["zai", L.ZAILogo], ["moonshot", L.KimiLogo], ["vercel-ai-gateway", L.VercelLogo], ["xai", L.XAILogo],
  ["minimax", L.MinimaxLogo], ["mistral", L.MistralLogo], ["groq", L.GroqLogo],
  ["slack", L.SlackLogo], ["discord", L.DiscordLogo], ["telegram", L.TelegramLogo], ["whatsapp", L.WhatsAppLogo],
  ["teams", L.MicrosoftTeamsLogo], ["google-chat", L.GoogleChatLogo], ["github", L.GithubLogo], ["linear", L.LinearLogo, { mode: "dark" }],
];
const sym = (id: string, html: string) => {
  const m = html.match(/<svg([^>]*)>([\s\S]*)<\/svg>/)!;
  const vb = m[1].match(/viewBox="([^"]+)"/)?.[1] ?? "0 0 24 24";
  return `<symbol id="${id}" viewBox="${vb}">${m[2].replace(/<title>[^<]*<\/title>/g, "")}</symbol>`;
};
const symbols = providers.map(([id, C, p]) => sym(`lg-${id}`, renderToStaticMarkup(React.createElement(C, p || {}))));
for (const v of ["logo", "symbol"] as const) symbols.push(sym(`cz-${v}`, renderToStaticMarkup(React.createElement(Logo, { variant: v, decorative: true }))));
const sprite = `<svg xmlns="http://www.w3.org/2000/svg" style="display:none" aria-hidden="true">${symbols.join("")}</svg>`;
await Bun.write("../../docs/design/opendesign/landing/landing-logos.js",
  `/* Generated from @compozy/ui/logos + Logo via react-dom/server. Do not hand-edit. */\ndocument.addEventListener("DOMContentLoaded",function(){var d=document.createElement("div");d.innerHTML=${JSON.stringify(sprite)};document.body.insertBefore(d.firstChild,document.body.firstChild);});\n`);
```
