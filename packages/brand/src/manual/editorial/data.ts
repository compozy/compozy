// Copy for the Editorial chapter. Components only map over these arrays.

export const ogRules: [string, string][] = [
  [
    "Size",
    "1200×630 (1.91:1), PNG under 300 KB. X, LinkedIn, Slack and WhatsApp all crop this ratio differently, so nothing important touches the outer 6%.",
  ],
  [
    "Safe area",
    "Text in the left 62% inside a 6% side and 11.4% top/bottom margin. The right side is the visual; if an app crops to square, the crop falls on art, not on words.",
  ],
  [
    "Content",
    "Lockup, one mono label (Blog, Docs, Changelog), the title, one mono meta line. Nothing else: no description, no author photo, no badge.",
  ],
  [
    "Title length",
    "Blog ≤ 70 characters, docs ≤ 60, changelog highlights ≤ 50 each. Longer strings are truncated with an ellipsis by the renderer, never shrunk.",
  ],
  [
    "Surface",
    "Default and essay posts are ink with an Atmosphere visual (converge, swarm). Changelog is ink with our UI (the board). Docs are paper with our UI (the desktop), so a docs link reads as reference material in a feed full of dark cards. Product pieces show the product; the visual never starts below 40% of the width.",
  ],
  [
    "Metadata",
    "og:title, og:description ≤ 160 characters, og:image absolute URL, og:image:alt describing the image. twitter:card is summary_large_image.",
  ],
];

export const ogImpl: [string, string][] = [
  [
    "Tokens",
    "lib/og/tokens.ts still carries the retired orange palette and Playfair. Replace COLORS with ink, ink-2, line, fg, muted, cyan and the blue-to-cyan stops from brand/theme.css.",
  ],
  [
    "Fonts",
    "Swap lib/og/fonts for Instrument Sans 400, Figtree 400/500 and Geist Mono 500 as TTF. Satori cannot read variable fonts or WOFF2, so export static instances.",
  ],
  [
    "Visual",
    "Satori draws flexbox and basic SVG, not the visual system. Export each visual once from /kit at 1200×630 without text, ship it as a PNG background and draw only text on top.",
  ],
  [
    "Templates",
    "blog.tsx, docs.tsx and landing.tsx map to og-blog, og-docs and og-default. Add changelog.tsx for og-changelog. Same margins: 72 px sides, 72 px top and bottom.",
  ],
  [
    "Routes",
    "app/opengraph-image.tsx renders og-default. app/og/[...slug]/route.tsx is force-static and already resolves docs (docsSource.getPage) and blog posts; add the section field and a changelog branch, keep the …/image.png suffix.",
  ],
  [
    "Cache",
    "Images are static per build. Revalidate on deploy only; never render per request with user input in the text.",
  ],
];

export const ogSnippet = `// packages/site/app/og/[...slug]/route.tsx: resolveDoc, after the rebrand
import { renderDocsOG } from "@/lib/og/templates/docs";
import { truncate } from "@/lib/og/tokens";
import { docsSource } from "@/lib/source";

export const dynamic = "force-static";   // one PNG per page, at build

function resolveDoc(rest: string[]) {
  const page = docsSource.getPage(rest);
  if (!page) return null;
  return {
    kind: "doc",
    section: sectionOf(rest[0]),           // "loops" -> "Loops", from the page tree
    title: truncate(page.data.title, 60),
    path: ["docs.compozy.com", ...rest].join("/"),
    background: "og/bg-docs.png",          // exported once from /kit/og-docs, no text
  };
}`;

export const docsRules: [string, string][] = [
  [
    "Surface",
    "Paper with white cards. Docs are read for long stretches in daylight; the dark manual is for marketing, the light docs are for work.",
  ],
  [
    "Measure",
    "Body text at 16 px Figtree, line height 1.65, 68 to 76 characters per line. Code blocks may run to 1.3× the measure and scroll horizontally.",
  ],
  [
    "Headings",
    "H1 at 40 px Instrument Sans 400, H2 at 26 px, H3 at 19 px. Three levels at most; a fourth level means the page should split.",
  ],
  [
    "Maturity",
    "Every page carries a mono label: stable, beta or experimental. Beta pages say what is missing in the first paragraph.",
  ],
  [
    "Code",
    "Ink block on paper, Geist Mono 13.5/1.7, a copy button, the shell prompt in muted so copying never includes it.",
  ],
  [
    "Visuals",
    "Docs open with a screenshot-grade UI: the desktop, the agents list, the board or the orchestrate view, drawn on paper (theme light), 16:5 at most. Real screenshots of the app replace them as soon as they exist. Atmosphere images are dark only and never sit on paper.",
  ],
  [
    "Diagrams",
    "Line work in muted ink on paper, 1.5 px strokes, boxes with 10 px radius, labels in Geist Mono. One accent colour per diagram: the active step.",
  ],
];

/** cyan ink on paper: the cyan token darkened toward ink (Tailwind needs the literal class) */
export const cyanInk = "text-[color-mix(in_oklab,var(--color-brand-cyan)_42%,var(--color-rail))]";

export const callouts = [
  {
    kind: "Note",
    border: "border-(--b-accent)",
    text: "text-(--b-accent)",
    body: "A Loop is forked from an existing one; there is no blank-canvas authoring. Start from a bundled Loop and edit it.",
  },
  {
    kind: "Beta",
    border: "border-[color-mix(in_oklab,var(--color-brand-cyan)_42%,var(--color-rail))]",
    text: cyanInk,
    body: "Goal judges support command, agent-judge and extension. Human judges are rejected for now.",
  },
  {
    kind: "Warning",
    border: "border-warning",
    text: "text-[color-mix(in_oklab,var(--color-warning)_54%,var(--color-rail))]",
    body: "The filesystem write path is last-write-wins. Agents editing the same Loop should publish with expected_version.",
  },
];

export const blogRules: [string, string][] = [
  [
    "Layout",
    "One column at 680 px for text, images to 880 px, the hero full width at 1600×840. No sidebar; a table of contents only past 1,800 words.",
  ],
  [
    "Title",
    "Instrument Sans 400 at 52 px, tracking −0.025em, sentence case, ≤ 70 characters so the OG image never truncates it. One idea, stated plainly.",
  ],
  [
    "Byline",
    "Mono line under the title: author, date, reading time. Pedro’s posts are first person; posts from the product account never use “we”.",
  ],
  [
    "Body",
    "Figtree 18 px, line height 1.7, paragraphs of 2 to 4 sentences. Pull quotes in Instrument Sans 26 px with a 2 px cyan rule.",
  ],
  [
    "Proof",
    "Every claim gets a command, a file or a link to the commit. Numbers carry their date: “2.7k stars, Nov 2026”.",
  ],
  [
    "Ending",
    "The last paragraph is one next step and one link. No newsletter pop-up, no “thanks for reading”.",
  ],
];

export const changelogRules: [string, string][] = [
  [
    "Screenshot first",
    "Every release entry opens with a screenshot or Live visual of the changed screen, then the list. The changelog OG shows the board.",
  ],
  [
    "Heading",
    "Version in Instrument Sans 400 (v0.6.0), date in Geist Mono uppercase. Semantic versions only; no release names.",
  ],
  [
    "Highlights",
    "Three at most, each one line, written as what the reader can now do. They become the og-changelog highlights verbatim.",
  ],
  [
    "Sections",
    "Added, Changed, Fixed, Removed, as mono labels. Each item is one line, a verb in the past tense, with the PR number in muted mono.",
  ],
  [
    "Breaking",
    "A red-bordered block at the top with the migration command. Never buried in Changed.",
  ],
  [
    "Honesty",
    "Known issues get their own section when they exist. A release that ships with a limit says so.",
  ],
];

export const changelogSample = {
  version: "v0.6.0",
  date: "Nov 18, 2026",
  highlights: [
    "Orchestrator hands work to other harnesses",
    "Board by who has the next step",
    "Always-on agents, with faces",
  ],
  sections: [
    [
      "Added",
      [
        "Board filter “Needs you” for everything waiting on you (#2841)",
        "Linked work: PRs attach to the card that produced them (#2856)",
        "Slack bridge posts approvals as threads (#2870)",
      ],
    ],
    [
      "Changed",
      [
        "“Stays on” is now “Always-on” in the UI and CLI help (#2833)",
        "Loop runs report best_generation on exhausted and stalled (#2862)",
      ],
    ],
    [
      "Fixed",
      [
        "Child sessions no longer inherit a wider permission scope on resume (#2849)",
        "Daemon restart keeps queued task claims (#2866)",
      ],
    ],
  ] as [string, string[]][],
  known:
    "Gateway delivery has no store-and-forward yet; offline peers miss events sent while they were away.",
};

export const newsletterRules: [string, string][] = [
  [
    "Format",
    "Text-first HTML at 600 px, one column, system fallbacks after Figtree. It must read fully with images off; the header is the only image.",
  ],
  [
    "Subject",
    "≤ 50 characters, sentence case, what shipped or what was learned. Preview text ≤ 90 characters adds the second fact.",
  ],
  [
    "Body",
    "Three blocks at most: what shipped, one thing learned, one thing next. Each block is a short paragraph and one link.",
  ],
  [
    "Links",
    "Plain underlined links in volt on white. One volt button at most, at the end, for the single action of the issue.",
  ],
  [
    "Footer",
    "Lockup in mono ink, why the reader gets this, unsubscribe in one click, the postal address the law requires. No social icon row.",
  ],
];

export const slideRules: [string, string][] = [
  [
    "Canvas",
    "1920×1080 (16:9), 5% side and 8.9% top/bottom margin. Every deck uses the five templates and nothing else.",
  ],
  [
    "Type",
    "Titles 3.6 to 6.4cqi (69 to 123 px), body 1.4 to 1.8cqi (27 to 35 px), mono labels 0.85cqi at the smallest. Anything smaller is a handout, not a slide.",
  ],
  ["One idea", "One claim per slide, written as a sentence. Three points at most under it."],
  [
    "Rhythm",
    "Title and section slides carry Atmosphere (block, horizon). Content and diagram slides carry our UI (board, orchestrate, desktop), so the audience sees the product, not a drawing of it.",
  ],
  [
    "Code",
    "Only in engineering talks: ≤ 14 lines at 1.25cqi (24 px), real syntax from the current release, the file path in the header. A product talk uses UI slides.",
  ],
  [
    "Footer",
    "Deck title left, page number right, in Geist Mono 0.85cqi, inside the bottom margin band. Nothing else lives in the margin.",
  ],
];

export const codeRules: [string, string][] = [
  [
    "Primary",
    "The UI card is the default image for a feature on X, LinkedIn and the blog: one Live visual (approval, board) in a window frame, one caption, the lockup.",
  ],
  [
    "Code is secondary",
    "A code image belongs to docs and to engineering posts, never to product, launch or ad pieces. Copied from the repo or a real session, then shortened. Never a flag or field that does not exist in the current release.",
  ],
  [
    "Length",
    "≤ 12 lines on a code excerpt, ≤ 70 columns. Longer snippets link to the file instead.",
  ],
  [
    "Colour",
    "Keys and prompts in cyan, strings in glint, numbers in sky, comments in faint. Four colours; no rainbow themes.",
  ],
  [
    "Frame",
    "UI window over the cone visual, 8cqi margin, caption bottom left, lockup bottom right. A code excerpt uses the same frame with the filename in the title bar.",
  ],
  [
    "Calls",
    "Call backgrounds (Atmosphere terrain) keep the centre 40% empty for a face; the lockup sits top left where it is not mirrored behind the head.",
  ],
  [
    "Wallpapers",
    "No text. A small mono-white symbol bottom right, above the macOS dock. Menu bar strip and macOS dock zone stay quiet.",
  ],
];
