/**
 * Every cached palette read is qualified by three axes, because the same command
 * id answers differently on each of them: the canonical workspace id, the profile
 * lens the daemon resolved the projection under, and — for the catalog — the
 * attached client whose context resolved it (Safety Invariant 17, ADR-016).
 *
 * The profile lens sits between workspace and client so the existing prefix walk
 * from `workspaceCatalogs` still sweeps every client of every profile in one
 * invalidation, while no two lenses ever share an entry.
 */
export const CMD_PALETTE_NO_CLIENT_KEY = "__unattached__";

/** The reserved aggregate identity; a real profile uses its own name. */
export const CMD_PALETTE_AGGREGATE_LENS_KEY = "@all";

export function cmdPaletteClientKey(clientId?: string | null): string {
  const normalized = typeof clientId === "string" ? clientId.trim() : "";
  return normalized === "" ? CMD_PALETTE_NO_CLIENT_KEY : normalized;
}

export function cmdPaletteProfileKey(profileKey: string): string {
  const normalized = profileKey.trim();
  return normalized === "" ? CMD_PALETTE_AGGREGATE_LENS_KEY : normalized;
}

export const cmdPaletteKeys = {
  all: ["cmd-palette"] as const,
  catalogs: () => [...cmdPaletteKeys.all, "catalog"] as const,
  workspaceCatalogs: (workspaceId: string) =>
    [...cmdPaletteKeys.catalogs(), workspaceId.trim()] as const,
  profileCatalogs: (workspaceId: string, profileKey: string) =>
    [...cmdPaletteKeys.workspaceCatalogs(workspaceId), cmdPaletteProfileKey(profileKey)] as const,
  catalog: (workspaceId: string, profileKey: string, clientId?: string | null) =>
    [
      ...cmdPaletteKeys.profileCatalogs(workspaceId, profileKey),
      cmdPaletteClientKey(clientId),
    ] as const,
  rankSignals: (workspaceId: string, profileKey: string) =>
    [
      ...cmdPaletteKeys.all,
      "rank-signals",
      workspaceId.trim(),
      cmdPaletteProfileKey(profileKey),
    ] as const,
  views: () => [...cmdPaletteKeys.all, "views"] as const,
  view: (workspaceId: string, profileKey: string, viewId: string) =>
    [
      ...cmdPaletteKeys.views(),
      workspaceId.trim(),
      cmdPaletteProfileKey(profileKey),
      viewId.trim(),
    ] as const,
};
