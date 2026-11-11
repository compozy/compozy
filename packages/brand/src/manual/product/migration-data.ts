// Migration table for chapter 05: every surface the previous identity touched, its path in the
// Compozy repo and its replacement. Token rows follow the spec's Data Models table (peer review
// round 1). The "Product adoption (desktop UI)" group of v1.2.0 is out of scope (ADR-005 §5):
// the product UI is re-skinned by tokens, never redesigned. The retired orange is named, never
// written as a value (UT-012).

export type MigrationRow = [asset: string, path: string, before: string, after: string];

export const migration: { group: string; rows: MigrationRow[] }[] = [
  {
    group: "Icons and manifest",
    rows: [
      [
        "Favicon (SVG)",
        "web/public/favicon.svg · packages/site/public/favicon.svg",
        "ꝏ on orange square",
        "brand/png/compozy-symbol-32.png + -16.png (gradient raster); SVG fallback compozy-symbol-mono-ink.svg",
      ],
      [
        "Favicon (ICO)",
        "web/public/favicon.ico · packages/site/public/favicon.ico",
        "orange 16/32",
        "ICO built from compozy-symbol-16/32/48.png",
      ],
      [
        "Apple touch icon",
        "web/public/apple-touch-icon.png · site",
        "180, orange",
        "brand/png/compozy-app-icon-180.png",
      ],
      [
        "PWA icons",
        "web/public/icon-192.png, icon-512.png · site",
        "orange",
        "compozy-app-icon-192.png, -512.png; maskable: compozy-maskable-512.png",
      ],
      [
        "Web manifest",
        "web/public/site.webmanifest · site",
        "theme: the retired orange · bg #141312",
        "theme_color #0a0a0a · background_color #0a0a0a",
      ],
      [
        "Desktop icon",
        "desktop/build/icon.icns, icon.png",
        "orange ꝏ",
        "iconutil from compozy-app-icon-1024.png (16–1024 iconset); icon.png = -512",
      ],
      [
        "Windows / Linux icon",
        "desktop/build (new)",
        "none",
        ".ico from app-icon 16–256; Linux 512 PNG",
      ],
      [
        "Tray icon",
        "desktop (new)",
        "none",
        "brand/product/tray/compozy-tray-{idle,working,attention}Template(@2x).png",
      ],
    ],
  },
  {
    group: "Logos and generated images",
    rows: [
      [
        "Logo component (site)",
        "packages/site/lib/og/logo.tsx",
        "lockup 1965×355, orange",
        "brand/svg/compozy-lockup-dark.svg (on ink), -light.svg (on paper)",
      ],
      [
        "Logo component (video)",
        "packages/launch-video/src/brand/logo.tsx",
        "same lockup",
        "compozy-lockup-dark.svg; symbol alone compozy-symbol.svg",
      ],
      [
        "OG templates",
        "packages/site/lib/og/templates/{landing,blog,docs}.tsx",
        "warm dark + orange",
        "Editorial chapter link-image templates (kit/og-*)",
      ],
      [
        "OG tokens and fonts",
        "packages/site/lib/og/tokens.ts, fonts.ts",
        "Geist, Playfair, JetBrains Mono",
        "@compozy/ui token values; Figtree, Instrument Sans, Geist Mono",
      ],
      [
        "README hero",
        "README.md (icon-512 @96 px)",
        "orange icon",
        "compozy-lockup-dark-256h.png (dark) with a light <picture> source",
      ],
      [
        "README images",
        "imgs/screenshot.png 3124×2002, imgs/how-it-works-flow.png/.svg",
        "old theme",
        "Re-shoot in the new theme; redraw the flow in the diagram language (chapter 04)",
      ],
      [
        "Site illustrations",
        "packages/site/public/images/* (hero-bg.webp, orbit, radar, traces, storyboards)",
        "orange glow art",
        "Visuals in brand/visuals/ (swarm, converge, horizon, field) and product shots captured from the running app against the demo seed",
      ],
      [
        "GitHub social preview",
        "repo settings (new)",
        "none",
        "Social chapter github template, 1280×640",
      ],
    ],
  },
  {
    group: "Tokens (packages/ui/src/tokens.css · tokens-light.css · terminal-tokens.css)",
    rows: [
      ["--color-rail / sunken", "tokens.css", "#0A0A0A / #0E0E0E", "#0a0a0a (ink)"],
      ["--color-canvas / desk", "tokens.css", "#131313", "#131313 (ink-2)"],
      ["--color-elevated", "tokens.css", "#1C1C1C", "#1b1b1b (ink-3)"],
      [
        "--color-surface-2 / selected",
        "tokens.css",
        "#232323 / #262626",
        "#232323 (ink-4, proposed)",
      ],
      ["--color-line / line-strong", "tokens.css", "#2A2A2A / #3A3A3A", "#262626 / #383838"],
      [
        "--color-line-focus, --color-ring",
        "tokens.css",
        "#5C5C5C / line-strong",
        "#3DF2E0 focus ring, 2 px, offset 2 px",
      ],
      ["--color-fg / muted", "tokens.css", "#F5F5F5 / #A3A3A3", "#ececec / #989898"],
      ["--color-disabled", "tokens.css", "#4D4D4D", "#5e5e5e (faint)"],
      [
        "--color-brand",
        "tokens.css · tokens-light.css",
        "the retired orange",
        "#1F87FC dark (signal blue) / #2448E6 light (volt): fills and marks",
      ],
      [
        "--color-brand-hover",
        "tokens.css · tokens-light.css",
        "darker orange",
        "#3D95FF dark (lighter, dark ink 6.5:1) / #1F3FCC light (darker, white ink keeps AA)",
      ],
      [
        "--color-brand-strong",
        "tokens.css · tokens-light.css",
        "lighter orange",
        "#4A9DFF dark / #2448E6 light: brand as text (AA on every surface), not cyan",
      ],
      [
        "--color-brand-ink",
        "tokens.css · tokens-light.css",
        "dark ink on orange",
        "#0A0A0A dark (white is 3.5:1 on signal blue) / #FFFFFF light: text on brand fills",
      ],
      [
        "--color-destructive-ink",
        "tokens.css · tokens-light.css (new)",
        "none",
        "Takes the previous brand-ink values; destructive-solid fills only",
      ],
      [
        "--color-brand-tint / tint-strong / dim / glow",
        "tokens.css · tokens-light.css",
        "orange tints",
        "rgba(47,91,255,.10/.16/.24/.05); light equivalents from #2448E6",
      ],
      [
        "--color-attn",
        "tokens.css · tokens-light.css",
        "alias of the brand colour",
        "#F5B14C dark / #9A5B00 light: its own amber value, never an alias of brand",
      ],
      ["--color-primary", "tokens.css", "near-white inverted action", "Unchanged"],
      [
        "--color-info / blue",
        "tokens.css",
        "#8E8EB5 / #4EA7FC",
        "Info unchanged (#8E8EB5 on ink, #5F5F94 on paper) so it never reads as the brand blue",
      ],
      ["--color-violet", "tokens.css", "#C26AD6", "Retired in UI; ANSI 5 #A78BFA only"],
      [
        "success / warning / destructive",
        "tokens.css",
        "#34D399 / #F5B14C / #F87171",
        "Unchanged on dark",
      ],
      [
        "success (light)",
        "tokens-light.css",
        "—",
        "#0A7650 (the brand token #0F8A5F is 4.05:1 on paper, below AA)",
      ],
      [
        "--terminal-*",
        "terminal-tokens.css",
        "grey #1A1A1A palette",
        "brand/product/terminal-palette.json",
      ],
      ["Radii", "tokens.css", "3 / 6 / 7 / 8 / 12 / 18 / pill", "4 / 6 / 6 / 10 / 14 / 20 / pill"],
      ["Motion", "tokens.css", "120 / 180 / 260 ms", "Unchanged: --duration-fast/base/slow"],
    ],
  },
  {
    group: "Type",
    rows: [
      [
        "App sans",
        "web/src/styles.css, packages/ui",
        "Inter Variable 425",
        "Figtree Variable 450 (UI), 400 (body). Inter is no longer a default; it stays a choice in the Appearance font catalog",
      ],
      ["App mono", "web/src/styles.css", "Geist Mono", "Geist Mono Variable 500 (kept)"],
      [
        "Display",
        "packages/ui --font-display = sans; OG Playfair",
        "Inter / Playfair",
        "Instrument Sans Variable 400, tracking −0.025em, as --font-display: logo, wordmark and site display only, never the product type ramp",
      ],
      ["OG mono", "packages/site/lib/og/fonts.ts", "JetBrains Mono", "Geist Mono"],
      ["Site", "packages/site/app/global.css", "Geist", "Figtree + Instrument Sans"],
    ],
  },
];

/** Before → after swatches on the migration card; the retired orange is named, not drawn. */
export const swatchesBefore = ["#141312", "#131313", "#F5F5F5"];
export const swatchesAfter = ["#1F87FC", "#2F5BFF", "#0a0a0a", "#131313", "#ececec"];

export const steps: [n: string, title: string, body: string][] = [
  [
    "01",
    "Tokens",
    "Swap tokens.css, tokens-light.css and terminal-tokens.css values; keep the variable names so components do not change.",
  ],
  [
    "02",
    "Fonts",
    "Replace Inter with Figtree for body and UI, add Instrument Sans for display sizes (400, tracking −0.025em); Geist Mono stays.",
  ],
  [
    "03",
    "Icons",
    "Replace favicon, touch, PWA, desktop and add tray icons in one release, so no surface shows two brands.",
  ],
  [
    "04",
    "Images",
    "Regenerate OG templates, README hero and site illustrations from the kit, brand/visuals/ and the product shots captured from web/.",
  ],
  [
    "05",
    "Check",
    "Run the sensors: search the repo for the retired orange and for Inter Variable, Playfair and JetBrains Mono in default font stacks; all must return nothing.",
  ],
];
