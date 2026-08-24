import { queryOptions } from "@tanstack/react-query";

import {
  getCmdPaletteRankSignals,
  getCmdPaletteView,
  listCmdPaletteCommands,
} from "../adapters/cmd-palette-api";
import { cmdPaletteKeys } from "./cmd-palette-query-keys";

/**
 * The catalog is invalidated by `cmd_palette.catalog.changed`, not by polling:
 * a zero stale time would refetch on every mount of every surface reading the
 * projection, and the stream already converges the revision.
 */
const CATALOG_STALE_TIME = 60_000;

export const cmdPaletteCatalogOptions = (
  workspaceId: string | null,
  profileKey: string,
  clientId: string | null,
  enabled = true
) => {
  const workspace = workspaceId?.trim() ?? "";
  const client = clientId?.trim() || null;
  return queryOptions({
    queryKey: cmdPaletteKeys.catalog(workspace, profileKey, client),
    queryFn: ({ signal }) => listCmdPaletteCommands(workspace, profileKey, client, signal),
    enabled: enabled && workspace !== "",
    staleTime: CATALOG_STALE_TIME,
  });
};

export const cmdPaletteViewOptions = (
  workspaceId: string | null,
  profileKey: string,
  viewId: string,
  enabled = true
) => {
  const workspace = workspaceId?.trim() ?? "";
  const view = viewId.trim();
  return queryOptions({
    queryKey: cmdPaletteKeys.view(workspace, profileKey, view),
    queryFn: ({ signal }) => getCmdPaletteView(workspace, profileKey, view, signal),
    enabled: enabled && workspace !== "" && view !== "",
    staleTime: 30_000,
  });
};

export const cmdPaletteRankSignalsOptions = (
  workspaceId: string | null,
  profileKey: string,
  enabled = true
) => {
  const workspace = workspaceId?.trim() ?? "";
  return queryOptions({
    queryKey: cmdPaletteKeys.rankSignals(workspace, profileKey),
    queryFn: ({ signal }) => getCmdPaletteRankSignals(workspace, profileKey, signal),
    enabled: enabled && workspace !== "",
    staleTime: Infinity,
  });
};
