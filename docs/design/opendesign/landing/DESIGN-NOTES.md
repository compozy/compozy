# Landing v2 — design notes

Prototype of the redesigned `compozy.com` homepage (2026-08-27). Source of truth for structure and copy: `.compozy/tasks/landing-redesign/landing-structure.md` (v2, 12 sections) and `landing-copy.md`; wireframe `_mockup.png`. Tokens: `packages/ui/src/tokens.css` via `design-system/ds-core.css` values (bare names), site extensions from `packages/site/app/global.css`. Iterate on these files — never regenerate.

## Files

| File | Role |
| --- | --- |
| `landing-page.html` | The page: nav → 12 sections → footer. `data-od-id` on every section, heading, CTA, tab strip, and repeated card. |
| `landing.css` | Tokens (`:root` mirror of `ds-core.css` + site display/clamps) + landing components. No color literal outside `:root`. |
| `landing.js` | ARIA tabs (arrow/Home/End), hero demo auto-advance (paused on interaction, hover, hidden tab, out of view), OS auto-detect for the download CTAs, copy buttons, DIY-stack collapse, install step numbering. |
| `landing-logos.js` | SVG sprite rendered from `@compozy/ui/logos` + `Logo` (26 providers, 8 bridges, `cz-logo`, `cz-symbol`). Regenerate, never hand-edit (recipe below). |
| `assets/os-shell-capture-v1.png` | Real OS-shell capture (`packages/site/public/images/hero/`). Stand-in poster on the hero plate only. |

## Direction contract

Recorded as the first comment in `landing-page.html` (THESIS · OWN-WORLD · STORY · FIRST VIEWPORT · FORM · FINISH). Register: **brand** (PRODUCT.md override for `packages/site`). Dials: `VISUAL_VARIANCE 5` (system-aligned, structure carries the difference) · `MOTION_INTENSITY 4` (one authored moment: the DIY-stack collapse; demo progress is functional) · `INFORMATION_DENSITY 4`. Accent budget per viewport: the download action (hero, final CTA) + the beta dot. Eyebrows follow the copy deck, sentence case 12/510, never accent.

## Section map (copy deck §)

| # | Section | `data-od-id` | Visual | Notes |
| --- | --- | --- | --- | --- |
| 1 | Hero | `hero` | 6 demo tabs over one window plate; full-bleed backdrop placeholder | Hero Lock verbatim; CTA label auto-detected (`Download for macOS/Linux`, else `Download` → /download). Home tab kept; drop to five if no seeded lab. |
| 2 | Providers | `providers` | 14×4 `ProviderQuilt` with real logos | Count `26` is `BUILTIN_PROVIDER_COUNT` in production — never hardcoded. |
| 3 | The DIY agent stack | `pain` | 9 HTML chips collapse into the CompozyOS block on scroll | Reduced motion: static chips → arrow → block. Replay control. |
| 4 | Use cases | `use-cases` | 5 cards (3 + 2 wide), spot-image placeholders | Order D2 → D1 → O1 → D3 → O3. O2 omitted (not rendered). |
| 5 | Community | `community` | 3 rule-top quotes + proof strip | Live numbers (stars, releases) render as skeleton bars until fetched. Provenance label = open item 1. |
| 6 | Features | `features` | 8 tabs, text left / capture frame right | Capture frames carry route + what to film; real captures from a seeded lab replace the body. |
| 7 | Loops | `loops` | run-page capture frame + Needs-you strip | Only deep-dive. Bullet 5 included because the run-page visual is present. |
| 8 | Extensibility | `extensibility` | concept placeholder + real `extension.json` (trimmed) + catalog grid | Catalog names only from `catalog/`. |
| 9 | Bridges | `bridges` | 8 logo tiles, one row | Caveat verbatim. |
| 10 | Comparison | `comparison` | 6-column table, CompozyOS column on the `--elevated` plate | Cells from deck slide 07; `<date>` footnote is a required placeholder (open item 7). |
| 11 | Install | `install` | Desktop · Installer · npm · Go tabs + steps | npm/Go reveal the bootstrap step and renumber. |
| 12 | Final CTA | `final-cta` | full-bleed closer placeholder | `Download CompozyOS` + one-liner + `Star on GitHub`. |

## Placeholder inventory (appeal layer — generated at execution)

All use `.gen` (canvas-tint plate, wave-line geometry, ember glow, mono tag). Family: `docs/design/generated/` — near-black, `#E8572A` glow as the light, wireframe-wave terrain as texture, no purple/blue, no people, no real screenshots.

| Placement | Element | Aspect / size | Prompt anchor |
| --- | --- | --- | --- |
| Hero backdrop | `.hero .gen--bleed` | full-bleed, ≥2400×1200, tonal overlay keeps text AA | deck-wave terrain low on the frame, ember light from below the plate |
| Use-case spots ×5 | `.uc .gen--spot` | 16:9, ~1200×675 | abstract job motifs: implement · review · briefing · release · gate |
| Extensibility concept | `.ext .gen--concept` | 16:8, ~1600×800 | one package → registries (skills, tools, agents, Loops) |
| Final-CTA closer | `.cta .gen--bleed` | full-bleed, ≥2400×1000 | wave inverted, light rising — the page's last cinematic moment |

Proof layer (real captures, never generated): 6 hero clips + posters, 8 feature-tab captures, 1 loop-run capture. Frames in the prototype state the route and what to film.

## Authorized deltas from the plan

- Install-tab download button uses the default (glaze) button, not the accent primary — keeps two accent primaries on the page (hero, final CTA) per the accent budget; the plan does not mandate its style.
- The hero plate reuses the real Loops/Tasks capture as a stand-in poster for all six tabs; alt text names it honestly. Replace per tab at demo production.
- `Batteries included.` renders inline with the `Built in` eyebrow (COPY §6 label) instead of a separate pill.
- Star counts / release counts are skeleton bars (the component's pre-fetch state), not invented numbers.
- Go install pin shows `v0.3.0-beta.21` (current desktop beta per the briefing); production reads the release tag from the changelog source.

## Verification (2026-08-27)

- Static: tags balanced, every `aria-controls` / `aria-labelledby` / anchor id resolves, all 36 sprite symbols referenced exist, no color literal outside `:root`, `node --check landing.js` clean, no `scrollIntoView`.
- Rendered once through the OD exporter at 1440 (desktop). Fixed from that pass: `hidden` steps were overridden by `.step{display:grid}` (now `[hidden]{display:none!important}`); Lucide dropped brand icons, so the GitHub mark comes from the sprite (`#lg-github`); manifest `tools` line split; pain heading one type step down; SDK row restructured. Repeated topnavs in the full-page PNG are a stitching artifact of `position: sticky`, not a page defect.
- Mobile (≤920 / ≤640) verified by reading the breakpoint rules only — the render budget was one desktop capture. Walk it in the OD preview before handoff.
- Recommendation for demo production: capture the six posters on a warm wallpaper; the current stand-in capture carries a purple/pink desktop backdrop that the appeal-layer palette bans.

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
