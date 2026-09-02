# Landing v3 — design notes

Prototype of the redesigned `compozy.com` homepage, rebuilt 2026-09-01 after the v2 pass (2026-08-27) landed far below the bar: it read as a wireframe (labeled placeholder plates, spec text inside capture frames, an eyebrow on every section). v3 keeps the operator-locked IA and copy deck and rebuilds the visual layer so the page reads as the finished product would. Source of truth for structure and copy: `.compozy/tasks/landing-redesign/landing-structure.md` (v2, 12 sections) and `landing-copy.md`. Tokens: `packages/ui/src/tokens.css` values mirrored in the `:root` block of `landing.css`; site extensions from `packages/site/app/global.css`. Iterate on these files, never regenerate.

Reference discipline applied on top of the Compozy authorities: `taste-skill` (Leonxlnx) sections 4, 9 and 14 (hero stack discipline, eyebrow budget, zero em-dashes, real imagery over div-screenshots, section-layout variety, zigzag cap), `imagegen-frontend-web` (composition anchors and background modes vary across sections; two full-bleed moments; rich sections alternate with calm ones), `impeccable` craft floor, `ui-craft` anti-defaults.

## Files

| File | Role |
| --- | --- |
| `landing-page.html` | The page: skip link → topnav → 12 sections → footer. `data-od-id` on every section, heading, CTA, tab strip and repeated card. Opens with the direction contract (THESIS · OWN-WORLD · STORY · FIRST VIEWPORT · FORM · FINISH). A Lucide subset is vendored as an inline `<symbol>` sprite (no CDN swap). |
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
| 2 | Providers | `providers` | stacked head + logo band | 26 real logos from the sprite, two rows of 13 at desktop (7/7/7/5 at 390px) |
| 3 | The DIY agent stack | `pain` | split copy / stage | nine dashed "parts" in a tray collapse into one CompozyOS block (the one authored motion); reduced motion: static parts → arrow → block |
| 4 | Use cases | `use-cases` | bento 3 + 2 | five spot images (stand-ins, see asset map) |
| 5 | Community | `community` | three editorial quotes + proof strip | none (calm section); live counts are skeleton bars |
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
| Hero backdrop | `hero-wave.webp` (from `packages/site/public/hero-bg.webp`) + CSS ember | reusable asset, final unless the generated full-bleed lands |
| Hero plate poster | `hero-poster.webp` (real capture, margins cropped) | stand-in for the six demo clips + posters; one poster serves all six tabs, two tabs pan it |
| Use-case spots ×5 | `spot-implement.webp` (`docs/design/generated/bento/runtime-v1.png`), `spot-review.webp` (`bento/trace-v1.png`), `spot-briefing.webp` (site `everything/` cron illustration), `spot-release.webp` (`generated/daemon-session/illustration3.png`, cropped above its replay control), `spot-gate.webp` (`generated/ig_0422…ebb6ed8acc….png`) | on-family stand-ins for the five generated spots |
| Features · Sessions | `feature-sessions.webp` (site `everything/` session timeline, own chrome cropped) | stand-in for the `/agents/$name/sessions/$id` capture |
| Features · Memory | `feature-memory.webp` (`bento/memory-v1.png`) | stand-in for the `/knowledge` capture |
| Features · Tasks | `capture-tasks-window.webp` (real) | stand-in for `/tasks?mode=kanban` (list view shown) |
| Features · Automation | `feature-automation.webp` (site `everything/` trace + events) | stand-in for the `/jobs` capture |
| Features · Desktops | `hero-poster.webp` (real) | final-grade: the live shell |
| Features · Profiles / Gateway / Workspaces | inline SVG diagrams over `backdrop-orbit.webp` / `backdrop-radar.webp` / `backdrop-traces.webp` (site background effects) | stand-ins for the three settings captures; diagrams show only truthful state (three switches off, no invented ids or counts) |
| Loops plate | `capture-loops-window.webp` (real) | stand-in for a `needs-approval` run at `/loop-runs/$runId`; the Needs-you strip is HTML (stand-in for `LoopRunNeedsYouCard`) |
| Extensibility | `ext-cartridges.webp` (site `bento-illustrations/extensibility-v2.png`) | reusable asset; the generated "one package → registries" concept may replace it |
| Bridges band | `bridges-inflow.webp` (`bento/bridges-v1.png`) | reusable asset, final |
| Final CTA closer | `closer-shell.webp` (site `bento-illustrations/os-v2.png`) | reusable asset; the generated closer may replace it |

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
