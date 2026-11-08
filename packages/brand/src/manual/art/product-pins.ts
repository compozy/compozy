// Pinned web/ Storybook stories behind every product shot in the manual (ADR-005, _uiux.md S3).
// `export:product-shots` resolves each pin to a story id through the built Storybook index.json
// (importPath + exportName), so a renamed or deleted story fails the capture loudly (US-004.EC-1).
import type { ProductShotName, VisualTheme } from "./visuals";

export type ShotTheme = VisualTheme;

export interface ProductShotPin {
  name: ProductShotName;
  /** story file, relative to web/ */
  story: string;
  /** named export inside the story file */
  exportName: string;
  /** CSS viewport of the capture (rendered at 2x) */
  viewport: { width: number; height: number };
  /** capture only this element (centered stories); omitted = the whole viewport */
  selector?: string;
  /** CSS px of surrounding story canvas kept around `selector` */
  pad?: number;
}

export const SHOT_THEMES: readonly ShotTheme[] = ["dark", "light"];

export const PRODUCT_SHOT_PINS: readonly ProductShotPin[] = [
  {
    name: "desktop",
    story: "src/systems/os/components/stories/os-window-deck.stories.tsx",
    exportName: "VC06PinnedMixedSurfaces",
    viewport: { width: 1280, height: 800 },
  },
  {
    name: "sessions",
    story: "src/systems/session/components/stories/session-sidebar.stories.tsx",
    exportName: "Folders",
    viewport: { width: 1280, height: 800 },
    // The rail itself; the story's empty right pane is decorator padding, not product.
    selector: ".w-66",
    pad: 28,
  },
  {
    name: "board",
    story: "src/systems/session/components/stories/sessions-board.stories.tsx",
    exportName: "Populated",
    viewport: { width: 1540, height: 680 },
  },
  {
    name: "orchestrate",
    story:
      "src/systems/session/components/assistant-ui/stories/session-thread-subagents.stories.tsx",
    exportName: "AgentsRunning",
    viewport: { width: 1100, height: 760 },
    selector: '[data-testid="subagent-story-window"]',
  },
  {
    name: "approval",
    story: "src/systems/session/components/stories/permission-dock.stories.tsx",
    exportName: "AllFourDecisions",
    viewport: { width: 1100, height: 700 },
    selector: ".max-w-xl",
    pad: 40,
  },
  {
    // First populated export of the team page (pinned in S3; _uiux.md S3 table).
    name: "agents",
    story: "src/systems/agent/components/stories/agent-team-page.stories.tsx",
    exportName: "Cards",
    viewport: { width: 820, height: 620 },
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

/** Storybook index.json → story id per pin; `missing` names every pin with no matching story. */
export function resolvePins(
  index: { entries?: Record<string, StoryIndexEntry> },
  pins: readonly ProductShotPin[] = PRODUCT_SHOT_PINS
): { resolved: { pin: ProductShotPin; id: string }[]; missing: string[] } {
  const entries = Object.values(index.entries ?? {}).filter(e => e.type !== "docs");
  const resolved: { pin: ProductShotPin; id: string }[] = [];
  const missing: string[] = [];
  for (const pin of pins) {
    const hit = entries.find(
      e => e.importPath?.replace(/^\.\//, "") === pin.story && e.exportName === pin.exportName
    );
    if (hit) resolved.push({ pin, id: hit.id });
    else missing.push(`${pin.name}: web/${pin.story} → ${pin.exportName}`);
  }
  return { resolved, missing };
}
