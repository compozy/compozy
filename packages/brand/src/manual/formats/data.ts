// Copy and numbers of the Formats chapter. Margin, columns, gutter and logo size are derived
// from the canvas so the tables and the drawn frames cannot drift apart.

export type Group = "Social" | "Ads" | "Link and docs" | "Slides and video" | "Print";
export interface Canvas {
  id: string;
  name: string;
  group: Group;
  w: number;
  h: number;
  ratio: string;
  use: string;
  unit?: "mm";
  strip?: boolean;
}

export const canvases: Canvas[] = [
  {
    id: "x-post",
    name: "X post",
    group: "Social",
    w: 1600,
    h: 900,
    ratio: "16:9",
    use: "Releases and announcements",
    strip: true,
  },
  {
    id: "x-header",
    name: "X header",
    group: "Social",
    w: 1500,
    h: 500,
    ratio: "3:1",
    use: "Profile header",
    strip: true,
  },
  {
    id: "li-post",
    name: "LinkedIn post",
    group: "Social",
    w: 1200,
    h: 1500,
    ratio: "4:5",
    use: "Feed post, carousel page",
    strip: true,
  },
  {
    id: "li-banner",
    name: "LinkedIn banner",
    group: "Social",
    w: 1584,
    h: 396,
    ratio: "4:1",
    use: "Personal profile background",
  },
  {
    id: "square",
    name: "Square",
    group: "Social",
    w: 1080,
    h: 1080,
    ratio: "1:1",
    use: "When 4:5 does not fit, Discord",
    strip: true,
  },
  {
    id: "story",
    name: "Story and short",
    group: "Social",
    w: 1080,
    h: 1920,
    ratio: "9:16",
    use: "Stories, Shorts, vertical cuts",
    strip: true,
  },
  {
    id: "yt-banner",
    name: "YouTube banner",
    group: "Social",
    w: 2560,
    h: 1440,
    ratio: "16:9",
    use: "Channel art, safe 1546 × 423",
  },
  {
    id: "yt-thumb",
    name: "YouTube thumbnail",
    group: "Social",
    w: 1280,
    h: 720,
    ratio: "16:9",
    use: "Read at 168 px wide",
  },
  {
    id: "avatar",
    name: "Avatar",
    group: "Social",
    w: 800,
    h: 800,
    ratio: "1:1",
    use: "Every profile, cropped to a circle",
  },
  {
    id: "ad-feed",
    name: "Feed ad",
    group: "Ads",
    w: 1080,
    h: 1350,
    ratio: "4:5",
    use: "Meta, LinkedIn sponsored",
  },
  {
    id: "ad-story",
    name: "Story ad",
    group: "Ads",
    w: 1080,
    h: 1920,
    ratio: "9:16",
    use: "Full-screen placements",
  },
  {
    id: "ad-x-card",
    name: "X website card",
    group: "Ads",
    w: 800,
    h: 418,
    ratio: "1.91:1",
    use: "Promoted post with link",
  },
  {
    id: "og",
    name: "Link image",
    group: "Link and docs",
    w: 1200,
    h: 630,
    ratio: "1.91:1",
    use: "Open Graph for site, docs, blog",
    strip: true,
  },
  {
    id: "gh-social",
    name: "GitHub preview",
    group: "Link and docs",
    w: 1280,
    h: 640,
    ratio: "2:1",
    use: "Repository social preview",
  },
  {
    id: "docs-hero",
    name: "Docs header",
    group: "Link and docs",
    w: 2400,
    h: 800,
    ratio: "3:1",
    use: "Guide and section headers",
  },
  {
    id: "slide",
    name: "Slide",
    group: "Slides and video",
    w: 1920,
    h: 1080,
    ratio: "16:9",
    use: "Talks, meetups, demos",
  },
  {
    id: "video",
    name: "Video 4K",
    group: "Slides and video",
    w: 3840,
    h: 2160,
    ratio: "16:9",
    use: "Product videos, stage screens",
  },
  {
    id: "wallpaper",
    name: "Wallpaper",
    group: "Slides and video",
    w: 2560,
    h: 1600,
    ratio: "16:10",
    use: "Desktop wallpaper, key visuals",
  },
  {
    id: "a3",
    name: "Poster A3",
    group: "Print",
    w: 297,
    h: 420,
    ratio: "1:√2",
    use: "Event posters, riso",
    unit: "mm",
    strip: true,
  },
  {
    id: "card",
    name: "Business card",
    group: "Print",
    w: 85,
    h: 55,
    ratio: "17:11",
    use: "Ink front, gradient edge",
    unit: "mm",
  },
  {
    id: "sticker",
    name: "Sticker",
    group: "Print",
    w: 76,
    h: 76,
    ratio: "1:1",
    use: "Die-cut symbol, 3 inch",
    unit: "mm",
  },
];

const snap8 = (v: number) => Math.max(8, Math.round(v / 8) * 8);
/** margin: 1/16 of the width on every side, the same rule the kit templates use (print: 8 % of the short side, 5 mm minimum) */
export const MARGIN_PCT = 6.25;
export const marginOf = (c: Canvas) =>
  c.unit === "mm"
    ? Math.max(5, Math.round(Math.min(c.w, c.h) * 0.08))
    : Math.round((c.w * MARGIN_PCT) / 100);
export const columnsOf = (c: Canvas) => (c.w / c.h >= 1.2 ? 12 : 4);
export const gutterOf = (c: Canvas) => (c.unit === "mm" ? 4 : Math.min(c.w, c.h) >= 900 ? 24 : 16);
export const logoOf = (c: Canvas) =>
  c.unit === "mm"
    ? Math.max(6, Math.round(Math.min(c.w, c.h) * 0.05))
    : snap8(Math.min(c.w, c.h) * 0.04);

export const gridRules: [string, string][] = [
  [
    "One margin",
    "1/16 of the width (6.25 %) on all four sides. 1600 × 900 → 100 px; 1080 × 1920 → 68 px. Print: 8 % of the short side.",
  ],
  [
    "Columns by shape",
    "Landscape (ratio ≥ 1.2) uses 12 columns, everything else 4. The quiet zone spans 5–6 of 12 or 2 of 4.",
  ],
  ["Gutter", "24 px when the short side is 900 px or more, 16 px below that."],
  ["8 px rhythm", "Every gap is a step of the 8 px scale. Type sits on a 4 px baseline."],
];

export const spacing: [number, string][] = [
  [8, "mono label to headline"],
  [16, "headline to body"],
  [24, "gutter, chip to chip"],
  [32, "body to call to action"],
  [48, "block to block"],
  [64, "logo to headline on 4:5"],
  [96, "section break on slides"],
  [128, "quiet edge on 4K"],
];

export interface Zone {
  label: string;
  x: number;
  y: number;
  w: number;
  h: number;
  kind?: "cover" | "safe";
}
export interface SafeCase {
  id: string;
  name: string;
  w: number;
  h: number;
  zones: Zone[];
  note: string;
  circle?: boolean;
}
export const safeCases: SafeCase[] = [
  {
    id: "x-header",
    name: "X header · 1500 × 500",
    w: 1500,
    h: 500,
    note: "The avatar covers the bottom-left; mobile crops 60 px top and bottom.",
    zones: [
      { label: "avatar", x: 2.7, y: 58, w: 22, h: 52, kind: "cover" },
      { label: "mobile crop", x: 0, y: 0, w: 100, h: 12, kind: "cover" },
      { label: "", x: 0, y: 88, w: 100, h: 12, kind: "cover" },
    ],
  },
  {
    id: "li-banner",
    name: "LinkedIn banner · 1584 × 396",
    w: 1584,
    h: 396,
    note: "Photo and name sit over the left 30 %. Keep the visual right, text out.",
    zones: [{ label: "profile photo", x: 3, y: 40, w: 22, h: 75, kind: "cover" }],
  },
  {
    id: "yt-banner",
    name: "YouTube banner · 2560 × 1440",
    w: 2560,
    h: 1440,
    note: "Only the centre 1546 × 423 shows on every device. Text and logo live there.",
    zones: [
      {
        label: "safe on all devices · 1546 × 423",
        x: 19.8,
        y: 35.3,
        w: 60.4,
        h: 29.4,
        kind: "safe",
      },
      { label: "desktop · 2560 × 423", x: 0, y: 35.3, w: 100, h: 29.4 },
    ],
  },
  {
    id: "story",
    name: "Story and short · 1080 × 1920",
    w: 1080,
    h: 1920,
    note: "Top 250 px for the progress bar and handle, bottom 380 px for caption and reply.",
    zones: [
      { label: "progress, handle · 250", x: 0, y: 0, w: 100, h: 13, kind: "cover" },
      { label: "caption, actions · 380", x: 0, y: 80.2, w: 100, h: 19.8, kind: "cover" },
      { label: "UI", x: 85, y: 50, w: 15, h: 30, kind: "cover" },
    ],
  },
  {
    id: "avatar",
    name: "Avatar · 800 × 800",
    w: 800,
    h: 800,
    circle: true,
    note: "Every network crops to a circle. The symbol fills 64 % of the diameter, centred.",
    zones: [{ label: "symbol · 64 %", x: 18, y: 18, w: 64, h: 64, kind: "safe" }],
  },
  {
    id: "gh-social",
    name: "GitHub preview · 1280 × 640",
    w: 1280,
    h: 640,
    note: "Shown at 2:1 and centre-cropped to 1.91:1 and 1:1 in link previews. Keep text in the centre 640.",
    zones: [
      { label: "1:1 crop", x: 25, y: 0, w: 50, h: 100, kind: "safe" },
      { label: "1.91:1", x: 2.4, y: 0, w: 95.2, h: 100 },
    ],
  },
];

export const compositionParts: [string, string, string][] = [
  [
    "Visual",
    "One named visual from the system, Atmosphere or Live, pushed to one side.",
    "1 per piece",
  ],
  ["Headline", "Instrument Sans 400, −0.03 em, 1.05 line height.", "≤ 8 words"],
  ["Mono metadata", "Geist Mono, uppercase, +0.12 em, cyan or muted.", "≤ 5 words"],
  ["Logo", "Lockup, height 4 % of the short side, in a corner.", "one per canvas"],
];

export const wordLimits: [string, string, string][] = [
  [
    "Headline",
    "≤ 8 words",
    "Read at feed speed: one idea, sentence case, no full stop on short labels.",
  ],
  ["Kicker", "≤ 5 words", "Release, feature or channel in mono: Release 0.6 · Loops."],
  ["Body", "≤ 20 words", "Only on 4:5 and slides. Nothing on stories and thumbnails."],
  ["Call to action", "≤ 4 words", "Install the beta. Start the daemon. Read the docs."],
  ["Footer meta", "1 line", "URL, date or command in mono. Never a second headline."],
];

export const logoCorners: [string, string][] = [
  ["Top-left", "Default on every landscape canvas: posts, slides, link images."],
  ["Bottom-left", "Portrait and story, under the headline, above the 380 px caption band."],
  [
    "Top-right",
    "Only when the visual sits on the left and the text moves right (flipped layouts).",
  ],
  ["Bottom-right", "Never: the visual’s focal point usually sits there."],
];

/**
 * The three grounds. `bg`/`fg`/`muted`/`accent` are the published hex values (shown as text and
 * used for the contrast maths); `fill` holds the same colours as @compozy/ui token CSS values.
 */
export interface SurfaceRecipe {
  id: "ink" | "black" | "paper";
  name: string;
  bg: string;
  fg: string;
  muted: string;
  accent: string;
  fill: { bg: string; fg: string; muted: string; accent: string };
  /** [token label, hex, CSS fill] */
  tokens: [string, string, string][];
  when: string;
  avoid: string;
  share: string;
}

const v = (token: string) => `var(${token})`;
const PAPER_MUTED = "color-mix(in oklab, var(--color-rail) 62%, var(--color-fg))";
const PAPER_ACCENT = "color-mix(in oklab, var(--color-brand-volt) 88%, var(--color-rail))";

export const surfaces: SurfaceRecipe[] = [
  {
    id: "ink",
    name: "Ink",
    bg: "#0a0a0a",
    fg: "#ececec",
    muted: "#989898",
    accent: "#3DF2E0",
    fill: {
      bg: v("--color-rail"),
      fg: v("--color-fg"),
      muted: v("--color-muted-foreground"),
      accent: v("--color-brand-cyan"),
    },
    share: "70 %",
    tokens: [
      ["--color-rail", "#0a0a0a", v("--color-rail")],
      ["--color-fg", "#ececec", v("--color-fg")],
      ["--color-muted-foreground", "#989898", v("--color-muted-foreground")],
      ["--color-brand-cyan", "#3DF2E0", v("--color-brand-cyan")],
    ],
    when: "The default for everything: product, social, slides, video. Carries an Atmosphere visual for mood or a Live visual for product.",
    avoid: "Long reading on paper-heavy channels like docs body and print.",
  },
  {
    id: "black",
    name: "Black",
    bg: "#000000",
    fg: "#ececec",
    muted: "#989898",
    accent: "#3DF2E0",
    fill: {
      bg: v("--color-black"),
      fg: v("--color-fg"),
      muted: v("--color-muted-foreground"),
      accent: v("--color-brand-cyan"),
    },
    share: "10 %",
    tokens: [
      ["black", "#000000", v("--color-black")],
      ["--color-fg", "#ececec", v("--color-fg")],
      ["--color-muted-foreground", "#989898", v("--color-muted-foreground")],
      ["--color-brand-cyan", "#3DF2E0", v("--color-brand-cyan")],
    ],
    when: "Closing slides, video bumpers, event screens and the last section of a page, where the visual should fade to nothing.",
    avoid: "Cards and UI: panels need a ground one step above, and #000 has none.",
  },
  {
    id: "paper",
    name: "Paper",
    bg: "#f6f6f6",
    fg: "#0a0a0a",
    muted: "#616161",
    accent: "#2448E6",
    fill: { bg: v("--color-fg"), fg: v("--color-rail"), muted: PAPER_MUTED, accent: PAPER_ACCENT },
    share: "20 %",
    tokens: [
      ["paper", "#f6f6f6", v("--color-fg")],
      ["--color-rail", "#0a0a0a", v("--color-rail")],
      ["ink muted", "#616161", PAPER_MUTED],
      ["volt light", "#2448E6", PAPER_ACCENT],
    ],
    when: "Docs, blog, print, LinkedIn documents. Carries a Live visual in its light theme.",
    avoid:
      "Atmosphere visuals (they are dark only) and cyan text: 1.4:1 on paper. Use #2448E6 for links and labels.",
  },
];

/** Volt is not a ground: it is the accent for buttons, links on paper, small marks and focus. */
export const voltAccent = {
  hex: "#2F5BFF",
  use: "Buttons, selected states, focus rings, small marks. 16 px or larger when it carries text.",
  avoid: "A full background, with or without a visual on it.",
};
