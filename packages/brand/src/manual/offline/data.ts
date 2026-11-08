// Content for chapter 11, Offline. Print values are starting points for press proofs, not certified matches:
// every CMYK and Pantone reference below must be confirmed on a drawdown from the printer.

/** hex, cmyk and pantone are displayed copy; swatch and text are the @compozy/ui tokens that paint it */
export interface Ink {
  name: string;
  hex: string;
  cmyk: string;
  pantone: string;
  note: string;
  swatch: string;
  text: string;
}

export const inks: Ink[] = [
  {
    name: "Ink",
    hex: "#0a0a0a",
    cmyk: "60 50 50 100 (neutral rich black)",
    pantone: "Black 6 C",
    note: "Never plain K 100: it prints grey. Small text under 10 pt in K 100 only, to avoid misregistration.",
    swatch: "var(--color-rail)",
    text: "var(--color-fg)",
  },
  {
    name: "Volt",
    hex: "#2F5BFF",
    cmyk: "82 62 0 0",
    pantone: "2726 C (closest: 2728 C)",
    note: "Process volt drifts purple; ask for a 2728 C spot on short runs where volt is the surface.",
    swatch: "var(--color-brand-volt)",
    text: "var(--color-fg-strong)",
  },
  {
    name: "Cyan",
    hex: "#3DF2E0",
    cmyk: "55 0 22 0",
    pantone: "3255 C · 802 C (fluorescent)",
    note: "Screen cyan is brighter than any process ink. Use 802 C fluorescent where cyan must glow.",
    swatch: "var(--color-brand-cyan)",
    text: "var(--color-rail)",
  },
  {
    name: "Deep",
    hex: "#0B31F7",
    cmyk: "96 78 0 0",
    pantone: "2728 C",
    note: "Shadow tone; gradients start at Volt. Holds well in process.",
    swatch: "var(--color-brand-deep)",
    text: "var(--color-fg-strong)",
  },
  {
    name: "Sky",
    hex: "#1F87FC",
    cmyk: "78 40 0 0",
    pantone: "2172 C",
    note: "Gradient middle.",
    swatch: "var(--color-brand-sky)",
    text: "var(--color-fg-strong)",
  },
  {
    name: "Glint",
    hex: "#B5FAF6",
    cmyk: "22 0 8 0",
    pantone: "317 C",
    note: "Gradient end. Disappears on white paper; only on ink or kraft.",
    swatch: "var(--color-brand-glint)",
    text: "var(--color-rail)",
  },
  {
    name: "Paper",
    hex: "#f6f6f6",
    cmyk: "3 1 1 0",
    pantone: "Paper white",
    note: "Print paper as the stock itself, never as a tint.",
    swatch: "var(--color-fg)",
    text: "var(--color-rail)",
  },
];

export const gradientPrint: [string, string][] = [
  [
    "Process (CMYK)",
    "Default. Four stops from Deep to Glint, built in the file, coated stock. Expect the cyan end to dull by about a third. Atmosphere crops print as four-colour on rich black.",
  ],
  [
    "Spot + fluorescent",
    "Recommended for posters, stickers and the business card back: 2728 C → 802 C fluorescent, overprinted blend. The cyan glows like the screen.",
  ],
  [
    "Risograph",
    "Blue + fluoro green drums, 60% overlap in the middle. Grain is welcome; registration may shift 1 mm.",
  ],
  [
    "One colour",
    "When the gradient is not possible: volt (2728 C) on paper, white on ink. Never a grey gradient.",
  ],
];

export const stocks: [string, string, string][] = [
  [
    "Business cards",
    "Uncoated, 400–600 g/m², black-dyed core or duplex",
    "Gradient side coated 350 g/m² + soft-touch",
  ],
  ["Letterhead", "Uncoated bright white, 120 g/m²", "Laser-safe, FSC"],
  ["Posters", "Coated matte, 170 g/m²", "Riso on 120 g/m² uncoated"],
  ["Stickers", "Matte white vinyl, 80 µm", "Clear vinyl for the symbol on laptops"],
  ["Badges", "Coated 350 g/m² or PVC 0.76 mm", "Round corners 3 mm"],
  ["Roll-up", "Anti-curl polyester 220 g/m², matte", "Never gloss: stage lights flare"],
];

export const whiteInk: string[] = [
  "On black fabric and kraft, print a white underbase under every colour, or cyan and volt sink into the fibre.",
  "The symbol on dark garments: underbase + 4 colours simulated process. On one-colour jobs it turns white, never grey.",
  "Clear stickers carry white behind the symbol only, not behind the points of a visual crop; those may stay translucent.",
  "White text below 8 pt on dark stock: no underbase choke; ask the printer for a 0.1 mm trap.",
];

export const fileSpecs: [string, string][] = [
  ["Format", "PDF/X-4, fonts outlined or embedded, one file per piece"],
  ["Resolution", "300 dpi at final size; 100 dpi at full size for roll-ups and backdrops"],
  ["Bleed", "3 mm (cards, A-sizes), 5 mm (roll-up), 2 mm (stickers), none for envelopes"],
  ["Safe area", "4 mm inside trim on cards and badges, 10 mm on A-sizes"],
  ["Colour", "FOGRA39 coated or FOGRA29 uncoated; spots named exactly as in the table"],
  ["Cut lines", "Spot colour “CutContour”, 0.25 pt, on its own layer, set to overprint"],
  ["Naming", "compozy_<piece>_<size>_<version>.pdf, e.g. compozy_poster_a3_v2.pdf"],
];

export const roles = [
  {
    id: "badge-team",
    role: "Team",
    surface: "Ink, Terrain",
    text: "Fg on ink, cyan chip",
    who: "People from Compozy",
  },
  {
    id: "badge-speaker",
    role: "Speaker",
    surface: "Ink, Swarm",
    text: "White, volt chip",
    who: "Everyone on stage",
  },
  {
    id: "badge-partner",
    role: "Partner",
    surface: "Paper, Lanes strip (Live, light)",
    text: "Ink, volt chip",
    who: "Design partners and hosts",
  },
  {
    id: "badge-attendee",
    role: "Attendee",
    surface: "Black, Field",
    text: "Fg on black, grey chip",
    who: "Everyone else",
  },
];

export const badgeRules: string[] = [
  "Name at 11% of badge width (about 11 mm cap height): readable at 2 m.",
  "Role in a mono chip, uppercase, never colour alone: people with colour blindness read the word.",
  "Top 11 mm is the lanyard slot. No text or art there.",
  "One name, one line where possible; long names wrap to two, never shrink below 8% width.",
  "Event and date at the foot in mono. Company is optional.",
];

export const stickers = [
  { name: "Symbol die-cut", size: "60 mm and 25 mm round", finish: "Matte vinyl, kiss-cut" },
  { name: "Wordmark", size: "70 × 22 mm", finish: "Matte vinyl, ink ground" },
  {
    name: "Keep every agent running",
    size: "90 × 20 mm pill",
    finish: "Paper-white vinyl, ink outline",
  },
  { name: "● running", size: "35 × 15 mm pill", finish: "Ink, cyan in 802 C fluorescent" },
  { name: "Agent face pill", size: "80 × 35 mm", finish: "Ink, face + name + state in cyan" },
  {
    name: "Agent faces",
    size: "25 mm round, set of 3+",
    finish: "Matte vinyl, ink ground; one per roster agent",
  },
  { name: "OS chip", size: "28 mm square", finish: "Ink, cyan text, line border" },
  {
    name: "Field crop",
    size: "55 × 90 mm contour",
    finish: "Matte vinyl on a black base: the lit points carry the colour",
  },
  { name: "Always-on", size: "40 × 20 mm pill", finish: "Ink, sky outline, gradient dot" },
];

export const merch = [
  {
    item: "Tee",
    fabric: "Black 100% organic cotton, 180–200 g/m²",
    specs: [
      ["Centre chest", "250 × 120 mm, 70 mm below collar"],
      ["Back", "Field crop 300 × 380 mm, optional"],
      ["Inks", "White underbase + 4-colour simulated process"],
      ["Sleeve", "Wordmark 60 mm, white"],
    ],
  },
  {
    item: "Hoodie",
    fabric: "Black heavyweight fleece, 400 g/m²",
    specs: [
      ["Left chest", "Symbol 70 mm, embroidered"],
      ["Back", "Field crop 320 × 400 mm"],
      ["Inks", "Underbase + deep, sky, cyan, glint"],
      ["Hood", "Nothing. The hood stays plain"],
    ],
  },
  {
    item: "Cap",
    fabric: "Black 6-panel cotton twill, unstructured",
    specs: [
      ["Front", "Symbol 45 mm, flat embroidery"],
      ["Thread", "One colour: white or cyan 802"],
      ["Back strap", "Wordmark 30 mm, tonal"],
      ["Never", "Gradient in thread"],
    ],
  },
  {
    item: "Tote",
    fabric: "Natural 12 oz cotton canvas",
    specs: [
      ["Front", "Wordmark 220 mm wide, centred"],
      ["Ink", "One colour ink #0a0a0a"],
      ["Handles", "70 cm, natural"],
      ["Back", "compozy.com, mono, 40 mm"],
    ],
  },
  {
    item: "Mug",
    fabric: "Matte black ceramic, 330 ml",
    specs: [
      ["Mark", "Symbol 35 mm, off-centre, handle right"],
      ["Print", "Ceramic decal, full colour"],
      ["Inside", "Black"],
      ["Text", "None"],
    ],
  },
  {
    item: "Notebook",
    fabric: "A5 black cloth cover, dot grid 5 mm",
    specs: [
      ["Cover", "Symbol 40 mm, blind deboss"],
      ["Spine", "Wordmark, white foil"],
      ["Paper", "100 g/m² ivory, 160 pages"],
      ["Ribbon", "Cyan 802"],
    ],
  },
];

export const distances: [string, string, string, string][] = [
  ["Badge name", "2 m", "11 mm cap height", "A6 badge"],
  ["Directional sign", "8 m", "40 mm cap height", "A2 / A1 boards"],
  ["Poster headline", "5 m", "30 mm cap height", "A3 / A2"],
  ["Roll-up headline", "10 m", "95 mm cap height", "850 × 2000 mm"],
  ["Backdrop line", "20 m", "200 mm cap height", "3 × 2.4 m wall"],
  ["Stage screen", "30 m", "1/12 of screen height", "16:9, 4K"],
];

export const signRules: string[] = [
  "Cap height = reading distance ÷ 200 is the minimum; double it for the main line.",
  "Arrows are a plain mono glyph (→ ← ↑) at the same height as the text, never an icon in a circle.",
  "One message per sign. “Talks →” and “Coffee ←” are two signs.",
  "Message above 1,600 mm on roll-ups and backdrops: lower area is hidden by people and tables.",
];

export const stationery: [string, string, string][] = [
  [
    "Business card",
    "85 × 55 mm",
    "Ink front with name and contact, back on ink with a Swarm crop. Both sides edge to edge.",
  ],
  [
    "Letterhead",
    "A4 210 × 297 mm",
    "Paper, lockup top left with a 6 mm cyan-ink rule, contact top right, a thin Lanes strip (Live, light) at the foot.",
  ],
  [
    "Envelope",
    "DL 220 × 110 mm",
    "Window left, 90 × 45 mm. Return address in mono, a thin Bars strip (Live, light) along the flap edge.",
  ],
  [
    "Email signature",
    "600 × 160 px",
    "White, name and role, one mono line, the light lockup. No banner, no quotes.",
  ],
];

export const world = [
  "tee-front",
  "hoodie-back",
  "cap",
  "tote",
  "laptop-stickers",
  "enamel-pin",
  "poster-wall",
  "mug",
];
