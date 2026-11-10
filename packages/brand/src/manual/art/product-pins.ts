// What every product shot in the manual captures (ADR-005, _uiux.md S3). App pins are real web/
// routes in the running app against the demo seed (`go run ./scripts/demo-seed`), with the
// interaction steps that bring the screen to its state; `export:product-shots` resolves each
// pin's seeded workspace by name before any capture, so a seed or route drift fails loudly
// (US-004.EC-1). A story pin is the exception for a state the seed cannot hold.
import type { ProductShotName, VisualTheme } from "./visuals";

export type ShotTheme = VisualTheme;

/** One interaction before the capture; selectors are Playwright selectors. */
export type CaptureStep = { click: string } | { press: string } | { waitFor: string };

interface PinFrame {
  name: ProductShotName;
  /** CSS viewport of the capture (rendered at 2x) */
  viewport: { width: number; height: number };
  /** capture only this element; omitted = the whole viewport */
  selector?: string;
  /** CSS px of surrounding canvas kept around `selector` */
  pad?: number;
}

export interface AppShotPin extends PinFrame {
  source: "app";
  /** app route, e.g. `/session/<id>`; `?workspace=<id>` is appended when `workspace` is set */
  route: string;
  /** seeded workspace name the route is scoped to */
  workspace?: string;
  steps?: readonly CaptureStep[];
  /** element that proves the screen rendered its seeded content */
  ready: string;
}

export interface StoryShotPin extends PinFrame {
  source: "story";
  /** story file, relative to web/ */
  story: string;
  /** named export inside the story file */
  exportName: string;
}

export type ProductShotPin = AppShotPin | StoryShotPin;

export const SHOT_THEMES: readonly ShotTheme[] = ["dark", "light"];

/** The seed's launch workspace (scripts/demo-seed, Northstar Pay). */
export const SEED_WORKSPACE = "Northstar Pay Launch HQ";

const SCREEN = { width: 1280, height: 800 } as const;

export const PRODUCT_SHOT_PINS: readonly ProductShotPin[] = [
  {
    // The desktop shell: menu bar, dock, and a session window with its seeded conversation.
    name: "desktop",
    source: "app",
    route: "/session/sess_northstar_launch_decision",
    ready: '[data-testid="session-sidebar"]',
    viewport: SCREEN,
  },
  {
    // The sessions rail of that window: every seeded session with agent, provider and state.
    name: "sessions",
    source: "app",
    route: "/session/sess_northstar_launch_decision",
    ready: '[data-testid="session-sidebar"]',
    viewport: SCREEN,
    selector: '[data-testid="session-sidebar"]',
  },
  {
    // The Tasks board: seeded launch work across Pending, Blocked and Done. (The sessions board
    // has every seeded session in Done, so its lanes read empty.)
    name: "board",
    source: "app",
    route: "/tasks?mode=kanban",
    workspace: SEED_WORKSPACE,
    ready: "text=Pre-stage Brazil rollback monitor",
    viewport: { width: 1540, height: 800 },
  },
  {
    // A Loop run in flight: step 8 of 13, the review fan-out running.
    name: "orchestrate",
    source: "app",
    route: "/loop-runs/loopr_northstar_market_rollout",
    workspace: SEED_WORKSPACE,
    ready: "text=review_market",
    viewport: SCREEN,
  },
  {
    // The seed holds no pending permission request, so approval stays the PermissionDock story.
    name: "approval",
    source: "story",
    story: "src/systems/session/components/stories/permission-dock.stories.tsx",
    exportName: "AllFourDecisions",
    viewport: { width: 1100, height: 700 },
    selector: ".max-w-xl",
    pad: 40,
  },
  {
    // The team: the always-on orchestrator above every seeded on-demand agent.
    name: "agents",
    source: "app",
    route: "/agents",
    ready: "text=On-demand",
    viewport: SCREEN,
  },
];

const PRODUCT_BASE = "/brand/product";

/** Public path of one capture: public/brand/product/<name>-<theme>.webp */
export const productShotFile = (name: ProductShotName, theme: ShotTheme) => `${name}-${theme}.webp`;
export const productShotSrc = (name: ProductShotName, theme: ShotTheme) =>
  `${PRODUCT_BASE}/${productShotFile(name, theme)}`;

interface StoryIndexEntry {
  id: string;
  type?: string;
  importPath?: string;
  exportName?: string;
}

/** Storybook index.json → story id per story pin; `missing` names every pin with no story. */
export function resolveStoryPins(
  index: { entries?: Record<string, StoryIndexEntry> },
  pins: readonly ProductShotPin[] = PRODUCT_SHOT_PINS
): { resolved: { pin: StoryShotPin; id: string }[]; missing: string[] } {
  const entries = Object.values(index.entries ?? {}).filter(e => e.type !== "docs");
  const resolved: { pin: StoryShotPin; id: string }[] = [];
  const missing: string[] = [];
  for (const pin of pins) {
    if (pin.source !== "story") continue;
    const hit = entries.find(
      e => e.importPath?.replace(/^\.\//, "") === pin.story && e.exportName === pin.exportName
    );
    if (hit) resolved.push({ pin, id: hit.id });
    else missing.push(`${pin.name}: web/${pin.story} → ${pin.exportName}`);
  }
  return { resolved, missing };
}

/** Seeded workspaces → the URL path of every app pin; `missing` names pins whose workspace is absent. */
export function resolveAppPins(
  workspaces: readonly { id: string; name: string }[],
  pins: readonly ProductShotPin[] = PRODUCT_SHOT_PINS
): { resolved: { pin: AppShotPin; path: string }[]; missing: string[] } {
  const resolved: { pin: AppShotPin; path: string }[] = [];
  const missing: string[] = [];
  const byName = new Map(workspaces.map(w => [w.name, w]));
  for (const pin of pins) {
    if (pin.source !== "app") continue;
    if (!pin.workspace) {
      resolved.push({ pin, path: pin.route });
      continue;
    }
    const ws = byName.get(pin.workspace);
    if (!ws) {
      missing.push(`${pin.name}: ${pin.route} needs seeded workspace "${pin.workspace}"`);
      continue;
    }
    const join = pin.route.includes("?") ? "&" : "?";
    resolved.push({ pin, path: `${pin.route}${join}workspace=${encodeURIComponent(ws.id)}` });
  }
  return { resolved, missing };
}
