/**
 * Every cached palette read is qualified by the canonical workspace id, and the
 * catalog additionally by the attached client whose context resolved it: the
 * same workspace answers differently for two attachments, so one projection can
 * never be replayed for another client (Safety Invariant 17).
 */
export const CMD_PALETTE_NO_CLIENT_KEY = "__unattached__";

export function cmdPaletteClientKey(clientId?: string | null): string {
  const normalized = typeof clientId === "string" ? clientId.trim() : "";
  return normalized === "" ? CMD_PALETTE_NO_CLIENT_KEY : normalized;
}

export const cmdPaletteKeys = {
  all: ["cmd-palette"] as const,
  catalogs: () => [...cmdPaletteKeys.all, "catalog"] as const,
  workspaceCatalogs: (workspaceId: string) =>
    [...cmdPaletteKeys.catalogs(), workspaceId.trim()] as const,
  catalog: (workspaceId: string, clientId?: string | null) =>
    [...cmdPaletteKeys.workspaceCatalogs(workspaceId), cmdPaletteClientKey(clientId)] as const,
  rankSignals: (workspaceId: string) =>
    [...cmdPaletteKeys.all, "rank-signals", workspaceId.trim()] as const,
  views: () => [...cmdPaletteKeys.all, "views"] as const,
  view: (workspaceId: string, viewId: string) =>
    [...cmdPaletteKeys.views(), workspaceId.trim(), viewId.trim()] as const,
};
