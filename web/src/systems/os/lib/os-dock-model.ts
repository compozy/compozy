import type { OsAttentionBadges } from "./attention-model";
import type { OsAppDescriptor } from "./app-catalog";
import type { OsAppId } from "./os-types";

const DOCK_ICON_IDS = [
  "sessions",
  "dashboard",
  "terminal",
  "agents",
  "network",
  "tasks",
  "loops",
  "jobs",
  "triggers",
  "marketplace",
  "bridges",
  "knowledge",
  "sandbox",
  "vault",
] as const;

export type DockIconId = (typeof DOCK_ICON_IDS)[number];

const DOCK_ICON_BY_APP = {
  session: "sessions",
  dashboard: "dashboard",
  terminal: "terminal",
  agents: "agents",
  network: "network",
  tasks: "tasks",
  loops: "loops",
  jobs: "jobs",
  triggers: "triggers",
  marketplace: "marketplace",
  bridges: "bridges",
  knowledge: "knowledge",
  sandbox: "sandbox",
  vault: "vault",
} as const satisfies Record<Exclude<OsAppId, "new-tab" | "settings">, DockIconId>;

export function dockIconForApp(app: Pick<OsAppDescriptor, "id">): DockIconId {
  if (app.id === "new-tab" || app.id === "settings") {
    throw new Error(`App ${app.id} is not available in the dock`);
  }
  return DOCK_ICON_BY_APP[app.id];
}

export function dockBadgeFor(
  app: Pick<OsAppDescriptor, "badge">,
  badges: OsAttentionBadges
): number | undefined {
  return app.badge ? badges[app.badge] : undefined;
}

/** OpenDesign dock name: app title, plus the exact needs-you count when present. */
export function dockItemAccessibleName(item: Pick<OsDockItemData, "name" | "badge">): string {
  const count = item.badge;
  if (count === undefined || count <= 0) return item.name;
  return count === 1 ? `${item.name} — 1 needs you` : `${item.name} — ${count} need you`;
}

export interface OsDockItemData {
  /** Stable app key. */
  id: string;
  /** App title used for the tooltip; the button name may append needs-you. */
  name: string;
  /** Presentational glyph identifier resolved by the dock component layer. */
  icon: DockIconId;
  /** Window is open. */
  running?: boolean;
  /** Window is minimized into its icon (hollow indicator, dimmed glyph). */
  minimized?: boolean;
  /** Attention count from a runtime projection; 0/undefined renders nothing. */
  badge?: number;
}

/** Group break matching OpenDesign `dock-sep` (sidebar group seams). */
export type OsDockSeparator = { id: string; sep: true };

export type OsDockEntry = OsDockItemData | OsDockSeparator;

export function isOsDockSeparator(entry: OsDockEntry): entry is OsDockSeparator {
  return "sep" in entry && entry.sep === true;
}
