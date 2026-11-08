// The brand package as published in public/brand/. Static prerender has no filesystem at runtime,
// so each published folder lists its files here (name and bytes); the public-assets test fails
// when this list and the folder drift apart. Groups not published in this package (digital set,
// kit exports, tokens and fonts) render as unlinked rows: exports live outside git in
// ~/Dev/compozy/_rebranding/exports (ADR-003).

export const BRAND_BASE = "/brand";

export interface FileRow {
  path: string;
  name: string;
  size: number;
}

export interface FileGroup {
  title: string;
  dir: string;
  note: string;
  /** files published under public/brand/<dir>; empty when the folder is not published yet */
  files: FileRow[];
}

const rowsOf = (dir: string, table: string): FileRow[] =>
  table
    .trim()
    .split("\n")
    .map(line => {
      const [name = "", bytes = "0"] = line.split(" ");
      return { path: `${BRAND_BASE}/${dir}/${name}`, name, size: Number(bytes) };
    });

const SVG = `
compozy-app-icon.svg 311382
compozy-lockup-dark.svg 315410
compozy-lockup-light.svg 315410
compozy-lockup-mono-ink.svg 47675
compozy-lockup-mono-white.svg 47675
compozy-lockup-os-dark.svg 317622
compozy-lockup-os-light.svg 317622
compozy-lockup-stacked-dark.svg 315027
compozy-lockup-stacked-light.svg 315027
compozy-symbol-mono-black.svg 43497
compozy-symbol-mono-ink.svg 43497
compozy-symbol-mono-white.svg 43497
compozy-symbol.svg 311217
compozy-wordmark-ink.svg 3198
compozy-wordmark-white.svg 3198
`;

const PNG = `
compozy-app-icon-1024.png 296977
compozy-app-icon-128h.png 10042
compozy-app-icon-180.png 18875
compozy-app-icon-192.png 20564
compozy-app-icon-256h.png 28985
compozy-app-icon-512.png 99673
compozy-app-icon-64h.png 3871
compozy-lockup-dark-128h.png 23462
compozy-lockup-dark-256h.png 64519
compozy-lockup-dark-64h.png 9360
compozy-lockup-light-128h.png 25456
compozy-lockup-light-256h.png 68646
compozy-lockup-light-64h.png 10401
compozy-lockup-mono-ink-128h.png 15710
compozy-lockup-mono-ink-256h.png 33990
compozy-lockup-mono-ink-64h.png 7483
compozy-lockup-mono-white-128h.png 12201
compozy-lockup-mono-white-256h.png 27692
compozy-lockup-mono-white-64h.png 5460
compozy-lockup-os-dark-128h.png 26003
compozy-lockup-os-dark-256h.png 70114
compozy-lockup-os-dark-64h.png 10424
compozy-lockup-os-light-128h.png 28000
compozy-lockup-os-light-256h.png 74518
compozy-lockup-os-light-64h.png 11494
compozy-lockup-stacked-dark-128h.png 10249
compozy-lockup-stacked-dark-256h.png 27394
compozy-lockup-stacked-dark-64h.png 4137
compozy-lockup-stacked-light-128h.png 11283
compozy-lockup-stacked-light-256h.png 29352
compozy-lockup-stacked-light-64h.png 4591
compozy-maskable-180.png 14696
compozy-maskable-512.png 79267
compozy-symbol-128.png 16997
compozy-symbol-128h.png 13593
compozy-symbol-16.png 897
compozy-symbol-180.png 28127
compozy-symbol-192.png 31124
compozy-symbol-24.png 1608
compozy-symbol-256.png 48768
compozy-symbol-256h.png 42105
compozy-symbol-32.png 2400
compozy-symbol-48.png 4326
compozy-symbol-512.png 159084
compozy-symbol-618.png 233233
compozy-symbol-64.png 6519
compozy-symbol-64h.png 4848
compozy-symbol-96.png 11361
compozy-symbol-mono-black-128h.png 3225
compozy-symbol-mono-black-256h.png 7335
compozy-symbol-mono-black-64h.png 1554
compozy-symbol-mono-ink-128h.png 5117
compozy-symbol-mono-ink-256h.png 11114
compozy-symbol-mono-ink-64h.png 2465
compozy-symbol-mono-white-128h.png 3882
compozy-symbol-mono-white-256h.png 9122
compozy-symbol-mono-white-64h.png 1692
compozy-wordmark-ink-128h.png 13615
compozy-wordmark-ink-256h.png 28449
compozy-wordmark-ink-64h.png 6572
compozy-wordmark-white-128h.png 10656
compozy-wordmark-white-256h.png 23233
compozy-wordmark-white-64h.png 4938
favicon.ico 7677
`;

const VISUALS = `
block-1280.jpg 100786
block.jpg 308260
cone-1280.jpg 56392
cone-portrait-1280.jpg 48649
cone-portrait.jpg 105424
cone.jpg 192438
converge-1280.jpg 105820
converge-portrait-1280.jpg 95812
converge-portrait.jpg 283186
converge.jpg 252170
field-1280.jpg 110984
field-portrait-1280.jpg 91490
field-portrait.jpg 186300
field.jpg 168241
horizon-1280.jpg 40975
horizon.jpg 104200
swarm-1280.jpg 126706
swarm-portrait-1280.jpg 144802
swarm-portrait.jpg 604431
swarm.jpg 542248
terrain-1280.jpg 93403
terrain-portrait-1280.jpg 76725
terrain-portrait.jpg 211931
terrain.jpg 265055
`;

const PRODUCT = `
agents-dark.webp 44252
agents-light.webp 46496
approval-dark.webp 15056
approval-light.webp 15360
board-dark.webp 93730
board-light.webp 97066
desktop-dark.webp 39786
desktop-light.webp 39634
orchestrate-dark.webp 68682
orchestrate-light.webp 71258
sessions-dark.webp 40704
sessions-light.webp 40682
terminal-palette.json 4433
`;

const TRAY = `
compozy-tray-attentionTemplate.png 359
compozy-tray-attentionTemplate.svg 1668
compozy-tray-attentionTemplate@2x.png 729
compozy-tray-attentionTemplate@2x.svg 1668
compozy-tray-idleTemplate.png 306
compozy-tray-idleTemplate.svg 265
compozy-tray-idleTemplate@2x.png 550
compozy-tray-idleTemplate@2x.svg 265
compozy-tray-workingTemplate.png 351
compozy-tray-workingTemplate.svg 1418
compozy-tray-workingTemplate@2x.png 684
compozy-tray-workingTemplate@2x.svg 1418
`;

export const kb = (b: number) =>
  b > 1e6 ? `${(b / 1e6).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1e3))} KB`;

export const groups: FileGroup[] = [
  {
    title: "Logos · SVG",
    dir: "svg",
    note: "Lockups, wordmark, symbol (gradient raster inside, one-colour vector), app icon.",
    files: rowsOf("svg", SVG),
  },
  {
    title: "Logos · PNG",
    dir: "png",
    note: "Symbol 16–618 px, app icons 180–1024, lockups at 64/128/256 px height, favicon.ico.",
    files: rowsOf("png", PNG),
  },
  {
    title: "Web icons",
    dir: "digital/web",
    note: "Drop into the site’s public root. head.html has the tags.",
    files: [],
  },
  {
    title: "Social profiles",
    dir: "digital",
    note: "Banners and avatars for X, LinkedIn, YouTube, GitHub and Discord, plus OG images and wallpapers.",
    files: [],
  },
  {
    title: "Visuals",
    dir: "visuals",
    note: "Atmosphere key visuals as JPG (1280 and 2560 px), landscape and portrait crops, for dark surfaces. Product visuals are captures of the real app, below.",
    files: rowsOf("visuals", VISUALS),
  },
  {
    title: "Product",
    dir: "product",
    note: "Captures of the real web/ components in the new theme, dark and light (export:product-shots), and the terminal palette.",
    files: rowsOf("product", PRODUCT),
  },
  {
    title: "Tray icons",
    dir: "product/tray",
    note: "macOS menu bar template icons for idle, working and attention, 1x and 2x.",
    files: rowsOf("product/tray", TRAY),
  },
  {
    title: "Kit exports",
    dir: "kit",
    note: "Every template at native size; print pieces also as PDF. Rendered by export:kit from /kit/<id> into $BRAND_EXPORT_DIR, outside git.",
    files: [],
  },
  {
    title: "Tokens and fonts",
    dir: ".",
    note: "tokens.css, theme.css, motion.json and the outlined wordmark fonts.",
    files: [],
  },
];
