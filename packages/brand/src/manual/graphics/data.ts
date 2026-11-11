// Copy for chapter 04, Graphic system. Components only map over these arrays.

export const uiSpec: [string, string][] = [
  [
    "Source",
    "Live visuals drawn from product state (desktop, agents, board, orchestrate, handoff, approval). Never a raw screenshot of a window with personal data",
  ],
  [
    "Names",
    "Real product nouns and the sample roster: checkout-lead, qa-runner, docs-steward. Harness names as the product spells them: Claude Code, Codex, Gemini CLI, OpenCode, Cursor",
  ],
  [
    "Crop",
    "Whole desktop for a launch or a hero; one window (agents, board, approval) when the piece makes one point",
  ],
  [
    "Tilt",
    "Use the built-in angles only (at most 18° on X, 14° on Y, 4° roll); one tilted visual per piece; flat (tilt false) when the piece carries a table, a form or small text",
  ],
  [
    "Text floor",
    "Body text inside the UI at least 14 px at export size; mono labels at least 12 px. Below that the visual is cropped tighter, not shrunk",
  ],
  [
    "Faces",
    "24 px minimum at export size, with the state mark and the harness chip. Below 24 px use the state dot only",
  ],
  [
    "Scrim",
    "Copy never sits on UI. Put the visual on one side and a left gradient scrim under the headline",
  ],
  ["Count", "One product visual per piece. A second one is a different piece"],
];

export const uiRules: string[] = [
  "Show the Live visual, not a screenshot. It is drawn from the same states as the product, in the brand, with sample data that is labelled as sample when it is not from a real run.",
  "Show only features that exist. Every control, state and name in a piece must be in the current build or on the published roadmap; check with the product team before publishing.",
  "Faces are agents, never people. A face next to a human name, a testimonial or a team page is wrong.",
  "Keep the state vocabulary: Working, Needs you, Idle, Paused, Failed. A face carries its state mark; colour alone never says it.",
  "The terminal is the before picture. It appears only as the five-terminals problem in a story, never as the image of Compozy.",
];

export const shapeRules: string[] = [
  "Every shape comes from the visual system: the point, the line, the bar and the cell. Nothing else is drawn by hand.",
  "One pitch: shapes on a page share a grid step (8, 16 or 32 px), so a dot field, a bar chart and a panel read as one system.",
  "Colour is signal, not decoration: most shapes are grey (#262626 to #5e5e5e) and at most one in ten is lit in cyan or the blue gradient.",
  "Panels and pills are the interface shapes; they carry content, never decoration.",
  "Hairlines are 1 px at every size. They organise; they never frame a picture.",
];

export const cropRules: string[] = [
  "Move the focal point with x and y (0 to 1), never by scaling the art past its box: a stretched visual loses its perspective.",
  "Keep the lit part of the visual (the stream, the ridge, the active session) inside the frame. A crop that loses it leaves only texture.",
  "Crop at one edge at a time: push the visual to the right, to the bottom or to the left, never into a corner that hides it.",
  "The crop sits on the side away from the headline; the quiet area takes the text, with a scrim when the two meet.",
];

export const diagramSpec: [string, string][] = [
  ["Node", "Rounded panel, radius 10 px at 1×, 1 px #383838 border, fill #131313"],
  [
    "Label",
    "Geist Mono 500 12 px, sentence case or real identifiers; a 1-line gloss in Figtree 13 px muted",
  ],
  [
    "Edge",
    "1 px #5e5e5e, straight or one right-angle turn, 4 px arrowhead; never curves with more than one bend",
  ],
  [
    "Highlight",
    "One node or one path per diagram in the gradient (stroke 1.5 px); it is the point of the diagram",
  ],
  ["State", "Status glyph + word at the node corner. Colour only where the state is the subject"],
  ["Grid", "8 px snap; nodes align on columns, gaps of 32 px minimum between nodes"],
];

export const diagramRules: string[] = [
  "Draw how it runs, not what it means. Boxes are real things: processes, sessions, folders, files.",
  "Read left to right or top to bottom. One direction per diagram.",
  "At most 9 nodes. Past that, split it into two diagrams with a shared node.",
  "Agents are drawn as their face (44 px in the 960 px drawing) with the harness named in the gloss. Never a bare box or a robot icon for an agent.",
  "No other icons inside nodes except the product icon set, at 16 px, left of the label.",
];

export const iconSpecs: [string, string][] = [
  ["Grid", "24 × 24 px, 2 px padding, 20 px live area"],
  ["Stroke", "1.5 px at 24; scales with the icon (1 px at 16, 2 px at 32)"],
  ["Ends", "Round caps, round joins, no fills except 1 px dots"],
  ["Corners", "Outer radius 3 px on panels, the same family as UI radius"],
  ["Colour", "One colour, currentColor. Fg on ink, ink on paper, white on volt"],
  ["Flat", "No tilt, no shadow, no gradient: perspective belongs to the visuals"],
];

export const iconSizes = [16, 20, 24, 32];

export const marks: { sections: [n: string, file: string, title: string][]; files: string[] } = {
  sections: [
    ["04", "shapes.svg", "Graphic system"],
    ["05", "status.ts", "Product"],
    ["09", "tokens.json", "Motion"],
  ],
  files: [
    "compozy.toml",
    "session/review-pr.log",
    "loops/flaky-auth.yaml",
    "skills/release-notes/SKILL.md",
    "~/.compozy/vault",
  ],
};

export const densities: { name: string; cover: string; use: string; rule: string }[] = [
  {
    name: "Quiet",
    cover: "≤ 15%",
    use: "Docs, forms, product empty states, long reads",
    rule: "Hairline grid only, or one small dot field in a corner.",
  },
  {
    name: "Present",
    cover: "15–40%",
    use: "Social posts, link images, slides, section heads",
    rule: "One visual, one quiet area for type on the opposite side.",
  },
  {
    name: "Full",
    cover: "40–70%",
    use: "Covers, wallpaper, event backdrops, merch",
    rule: "A full-bleed Atmosphere or a Live visual at scale; type is a single line or absent.",
  },
];

/**
 * Off-brand colours, used only to draw the counter-examples ("Don't" panels): a mascot cloud,
 * rainbow arrows and a matrix terminal. They exist to show what the system never uses.
 */
export const offBrand = {
  violet: "#7C3AED",
  orange: "#F97316",
  green: "#22C55E",
  pink: "#EC4899",
  matrix: "#00FF66",
  trafficRed: "#FF5F57",
  trafficAmber: "#FEBC2E",
  trafficGreen: "#28C840",
};
