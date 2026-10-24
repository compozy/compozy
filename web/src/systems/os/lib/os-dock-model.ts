import type { OsAttentionBadges } from "./attention-model";
import type { OsAppDescriptor } from "./app-catalog";
import type { OsAppId } from "./os-types";

const DOCK_ICON_IDS = [
  "sessions",
  "dashboard",
  "terminal",
  "agents",
  "tasks",
  "loops",
  "automations",
  "marketplace",
  "vault",
] as const;

export type DockIconId = (typeof DOCK_ICON_IDS)[number];

const DOCK_ICON_BY_APP = {
  session: "sessions",
  dashboard: "dashboard",
  terminal: "terminal",
  agents: "agents",
  tasks: "tasks",
  loops: "loops",
  automations: "automations",
  marketplace: "marketplace",
  vault: "vault",
} as const satisfies Record<Exclude<OsAppId, "new-tab" | "settings" | "usage">, DockIconId>;

export function dockIconForApp(app: Pick<OsAppDescriptor, "id">): DockIconId {
  if (app.id === "new-tab" || app.id === "settings" || app.id === "usage") {
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
  const count = item.badge ?? 0;
  if (count === 1) return `${item.name} — 1 needs you`;
  if (count > 1) return `${item.name} — ${count} need you`;
  return item.name;
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
  /** The focused window belongs to this app (selected plate in the rail). */
  active?: boolean;
  /** Window is minimized into its icon (hollow indicator, dimmed glyph). */
  minimized?: boolean;
  /** Attention count from a runtime projection; 0/undefined renders nothing. */
  badge?: number;
}
