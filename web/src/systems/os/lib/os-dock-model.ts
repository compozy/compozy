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
export function dockItemAccessibleName(
  item: Pick<OsDockItemData, "name" | "badge" | "hint">
): string {
  const count = item.badge ?? 0;
  let name = item.name;
  if (count === 1) name = `${item.name} — 1 needs you`;
  else if (count > 1) name = `${item.name} — ${count} need you`;
  return item.hint ? `${name}. ${item.hint}` : name;
}

/** Tooltip text: the app title, plus why the launcher does something else right now. */
export function dockItemTip(item: Pick<OsDockItemData, "name" | "hint">): string {
  return item.hint ? `${item.name} — ${item.hint}` : item.name;
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
  /** Why activation leads somewhere else right now (e.g. pick a project first). */
  hint?: string;
}
