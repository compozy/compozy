// The Compozy visual system: one registry for every visual a chapter can show.
// Atmosphere = approved generated key visuals (raster, landscape + portrait crops).
// Product shots = captures of real web/ Storybook stories in the new theme (ADR-005, slice S3).
// Diagrams = bars/lanes/stack rebuilt from @compozy/ui tokens (ADR-005, slice S3).
// Until S3 lands, product shots and diagrams render a marked placeholder slot in <Visual>.

export type AtmosphereName =
  | "swarm"
  | "converge"
  | "horizon"
  | "field"
  | "cone"
  | "block"
  | "terrain";
/** Captured from web/ Storybook in the new theme; ids pinned in product-shots.ts (S3). */
export type ProductShotName =
  | "desktop"
  | "sessions"
  | "board"
  | "agents"
  | "orchestrate"
  | "approval";
/** Not product screens: rebuilt as token diagrams in S3 (ADR-005 §3). */
export type DiagramName = "stack" | "lanes" | "bars";
export type VisualName = AtmosphereName | ProductShotName | DiagramName;

type VisualFamily = "atmosphere" | "product" | "diagram";

export interface VisualInfo {
  name: VisualName;
  family: VisualFamily;
  title: string;
  idea: string;
  /** atmosphere: has a portrait crop (public/brand/visuals/<name>-portrait.jpg) */
  portrait?: boolean;
  /** default focal point of the art (0..1), used for object-position */
  focus: [number, number];
}

export const visuals: VisualInfo[] = [
  {
    name: "swarm",
    family: "atmosphere",
    title: "Swarm",
    idea: "Hundreds of thousands of points that read as one controlled mass: many agents, one system.",
    portrait: true,
    focus: [0.62, 0.45],
  },
  {
    name: "converge",
    family: "atmosphere",
    title: "Converge",
    idea: "Scattered work entering on one side and leaving as a single precise stream: what the control plane does.",
    portrait: true,
    focus: [0.6, 0.5],
  },
  {
    name: "block",
    family: "atmosphere",
    title: "Block",
    idea: "The whole system as one solid made of points, dissolving at the edge where work comes in.",
    focus: [0.72, 0.42],
  },
  {
    name: "cone",
    family: "atmosphere",
    title: "Cone",
    idea: "Diffuse blue light through grain. Pure atmosphere for quiet pieces and backgrounds behind text.",
    portrait: true,
    focus: [0.3, 0.3],
  },
  {
    name: "horizon",
    family: "atmosphere",
    title: "Horizon",
    idea: "A band of light sweeping fine lines, like a scan passing. Calm, wide, precise.",
    focus: [0.5, 0.5],
  },
  {
    name: "terrain",
    family: "atmosphere",
    title: "Terrain",
    idea: "Contour lines with one lit ridge: the work surface, with the active path in signal colour.",
    portrait: true,
    focus: [0.62, 0.62],
  },
  {
    name: "field",
    family: "atmosphere",
    title: "Field",
    idea: "A large regular fleet of points with a few lit: the active units in a large system.",
    portrait: true,
    focus: [0.55, 0.6],
  },
  {
    name: "desktop",
    family: "product",
    title: "Desktop",
    idea: "The whole control plane in one frame: projects with their always-on faces, sessions and subagents on the left, the orchestrator session and its handoff messages on the right.",
    focus: [0.5, 0.5],
  },
  {
    name: "agents",
    family: "product",
    title: "Always-on agents",
    idea: "The team that checks in on its own. Each face shows its state: thinking, needs you, resting, paused.",
    focus: [0.62, 0.5],
  },
  {
    name: "board",
    family: "product",
    title: "Board",
    idea: "Every session across projects, in columns by who has the next step: needs you, working, waiting, in review, done.",
    focus: [0.55, 0.5],
  },
  {
    name: "orchestrate",
    family: "product",
    title: "Orchestrate",
    idea: "One session distributes the work; child sessions in other harnesses and worktrees deliver it as pull requests.",
    focus: [0.5, 0.5],
  },
  {
    name: "approval",
    family: "product",
    title: "Approval",
    idea: "The agent asks, you decide: the permission dock with allow once, always allow and deny.",
    focus: [0.62, 0.5],
  },
  {
    name: "sessions",
    family: "product",
    title: "Sessions",
    idea: "Real session cards (harness, worktree, state, log) in perspective, fading into black.",
    focus: [0.62, 0.5],
  },
  {
    name: "stack",
    family: "diagram",
    title: "Task stack",
    idea: "One task in three steps: the task, the diff, the checks, stepped down into black.",
    focus: [0.6, 0.5],
  },
  {
    name: "lanes",
    family: "diagram",
    title: "Run lanes",
    idea: "One lane per agent, one bar per run, the now line at the edge; runs that are live carry the gradient.",
    focus: [0.5, 0.5],
  },
  {
    name: "bars",
    family: "diagram",
    title: "Activity",
    idea: "Activity across a day in thin bars; the live window in gradient.",
    focus: [0.5, 0.6],
  },
];

export const visualByName = (n: VisualName): VisualInfo => {
  const v = visuals.find(x => x.name === n);
  if (!v) throw new Error(`unknown visual ${n}`);
  return v;
};

export const VISUALS_BASE = "/brand/visuals";

/**
 * Files published under public/brand/visuals/hi/ (the 4x tier). Hi-res masters live outside git
 * in ~/Dev/compozy/_rebranding (ADR-003), so the tier is omitted until a file is published here;
 * the public-assets test fails if a listed file is missing.
 */
export const HI_TIER: ReadonlySet<string> = new Set<string>();

type Crop = "land" | "port";
const WIDTHS: Record<Crop, [web: number, retina: number, hi: number]> = {
  land: [1280, 2560, 6688],
  port: [1024, 2049, 4488],
};

/** srcset for one Atmosphere file: 1280 + 2560 always, the hi tier only when published. */
export function atmosphereSrcSet(file: string, crop: Crop, hi: ReadonlySet<string> = HI_TIER) {
  const [web, retina, big] = WIDTHS[crop];
  const set = [
    `${VISUALS_BASE}/${file}-1280.jpg ${web}w`,
    `${VISUALS_BASE}/${file}.jpg ${retina}w`,
  ];
  if (hi.has(file)) set.push(`${VISUALS_BASE}/hi/${file}.jpg ${big}w`);
  return set.join(", ");
}
