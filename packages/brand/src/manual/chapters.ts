// Registry of the manual: drives the page nav, the in-page index, chapter covers,
// prev/next cards and the library. One entry per route file in src/routes (UT-001).
import type { VisualName } from "./art/visuals";

export const VERSION = "v1.2.0";
export const EDITION = "November 2026";

/** approved = decided by the founder and already binding; proposal = direction until approved */
export type Status = "approved" | "proposal";

export interface Chapter {
  slug: string;
  n: string;
  title: string;
  summary: string;
  status: Status;
  /** cover visual: theme light renders the chapter cover on paper (Live visuals only) */
  cover: { visual: VisualName; theme?: "dark" | "light"; x?: number; y?: number };
  sections: [id: string, label: string][];
}

export const pages: Chapter[] = [
  {
    slug: "",
    n: "01",
    title: "Foundations",
    summary:
      "Essence, audience, symbol, colour, type, voice and the tokens everything else is built on.",
    status: "approved",
    cover: { visual: "swarm", x: 0.7 },
    sections: [
      ["essence", "Essence"],
      ["audience", "Audience"],
      ["story", "Symbol story"],
      ["logo", "Logo"],
      ["color", "Colour"],
      ["type", "Typography"],
      ["elements", "Elements"],
      ["form", "Form"],
      ["voice", "Voice"],
      ["messaging", "Messaging"],
      ["applications", "Applications"],
      ["tokens", "Tokens"],
    ],
  },
  {
    slug: "visuals",
    n: "02",
    title: "Visual system",
    summary:
      "Our UI is the image: Live visuals of the desktop and the always-on agents, with Atmosphere for mood.",
    status: "proposal",
    cover: { visual: "converge" },
    sections: [
      ["principle", "Principle"],
      ["families", "Families"],
      ["atmosphere", "Atmosphere"],
      ["live", "Live"],
      ["faces", "Faces"],
      ["light", "Light and colour"],
      ["composition", "Composition"],
      ["pairing", "Pairing"],
      ["library", "Library"],
      ["donts", "Don'ts"],
    ],
  },
  {
    slug: "formats",
    n: "03",
    title: "Formats",
    summary: "Canvases, grid, safe areas and the three surfaces every piece starts from.",
    status: "proposal",
    cover: { visual: "field" },
    sections: [
      ["canvas", "Canvases"],
      ["grid", "Grid"],
      ["safe-areas", "Safe areas"],
      ["composition", "Composition"],
      ["surfaces", "Surfaces"],
    ],
  },
  {
    slug: "graphics",
    n: "04",
    title: "Graphic system",
    summary: "Shapes, product UI as image, icons, diagrams and the marks that organise a page.",
    status: "proposal",
    cover: { visual: "orchestrate", x: 0.62 },
    sections: [
      ["shapes", "Shapes"],
      ["ui", "Product UI"],
      ["diagrams", "Diagrams"],
      ["icons", "Icons"],
      ["marks", "Marks"],
      ["patterns", "Patterns"],
    ],
  },
  {
    slug: "product",
    n: "05",
    title: "Product",
    summary:
      "How the brand lives inside Compozy: the desktop control plane, density, agent states, data, and the CLI as a secondary surface.",
    status: "proposal",
    cover: { visual: "desktop", x: 0.82 },
    sections: [
      ["principles", "Principles"],
      ["surfaces", "Surfaces"],
      ["status", "Status colours"],
      ["controls", "Controls"],
      ["data", "Data"],
      ["cli", "CLI and API"],
      ["desktop", "Desktop app"],
      ["vocabulary", "Vocabulary"],
      ["migration", "Migration"],
    ],
  },
  {
    slug: "imagery",
    n: "06",
    title: "Imagery",
    summary:
      "Photography, product renders and generated images: what we show, how we treat it, what we avoid.",
    status: "proposal",
    cover: { visual: "cone", x: 0.2 },
    sections: [
      ["direction", "Direction"],
      ["types", "Types"],
      ["product-shots", "Product shots"],
      ["treatment", "Treatment"],
      ["generated", "Generated images"],
      ["avoid", "Avoid"],
      ["consent", "Consent"],
    ],
  },
  {
    slug: "social",
    n: "07",
    title: "Social",
    summary: "X, LinkedIn, YouTube, GitHub and Discord: one idea, the right shape for each.",
    status: "proposal",
    cover: { visual: "desktop" },
    sections: [
      ["networks", "Networks"],
      ["x", "X"],
      ["linkedin", "LinkedIn"],
      ["carousel", "Carousel"],
      ["youtube", "YouTube"],
      ["github", "GitHub"],
      ["captions", "Captions"],
    ],
  },
  {
    slug: "ads",
    n: "08",
    title: "Ads",
    summary:
      "Two modes, brand and proof, from hook to call to action, with live templates and specs per channel.",
    status: "proposal",
    cover: { visual: "board" },
    sections: [
      ["principles", "Principles"],
      ["structure", "Structure"],
      ["models", "Models"],
      ["copy", "Copy bank"],
      ["specs", "Specs"],
      ["review", "Review"],
    ],
  },
  {
    slug: "motion",
    n: "09",
    title: "Motion",
    summary:
      "Motion is the system working: particles settling, light sweeping, sessions changing state.",
    status: "proposal",
    cover: { visual: "horizon" },
    sections: [
      ["principles", "Principles"],
      ["tokens", "Tokens"],
      ["signatures", "Signatures"],
      ["kinetic-type", "Kinetic type"],
      ["video", "Video"],
      ["sound", "Sound"],
      ["delivery", "Delivery"],
    ],
  },
  {
    slug: "editorial",
    n: "10",
    title: "Editorial",
    summary: "Link previews, docs, blog, changelog, newsletter, slides and code as image.",
    status: "proposal",
    cover: { visual: "stack", theme: "light", x: 0.7 },
    sections: [
      ["og", "Link images"],
      ["docs", "Docs"],
      ["blog", "Blog"],
      ["changelog", "Changelog"],
      ["newsletter", "Newsletter"],
      ["slides", "Slides"],
      ["code", "Code"],
    ],
  },
  {
    slug: "offline",
    n: "11",
    title: "Offline",
    summary:
      "Print colour, badges, stickers, merch, event signage, stationery, and the brand on real materials.",
    status: "proposal",
    cover: { visual: "terrain" },
    sections: [
      ["print", "Print"],
      ["badges", "Badges"],
      ["stickers", "Stickers"],
      ["merch", "Merch"],
      ["signage", "Signage"],
      ["stationery", "Stationery"],
      ["world", "In the world"],
    ],
  },
  {
    slug: "library",
    n: "12",
    title: "Library",
    summary:
      "Every official file, the template kit, the pre-publish checklist and how the manual changes.",
    status: "approved",
    cover: { visual: "bars" },
    sections: [
      ["downloads", "Downloads"],
      ["kit", "Template kit"],
      ["checklist", "Checklist"],
      ["governance", "Governance"],
      ["references", "References"],
    ],
  },
];

export const hrefOf = (c: Chapter) => (c.slug ? `/${c.slug}` : "/");
export const pageBySlug = (slug: string) => {
  const p = pages.find(c => c.slug === slug);
  if (!p) throw new Error(`unknown chapter ${slug}`);
  return p;
};
