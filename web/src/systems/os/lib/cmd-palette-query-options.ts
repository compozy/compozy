import { queryOptions } from "@tanstack/react-query";

import { listCmdPaletteClients, listCmdPaletteCommands } from "../adapters/cmd-palette-api";
import { cmdPaletteKeys } from "./cmd-palette-query-keys";

/**
 * The catalog is invalidated by `cmd_palette.catalog.changed`, not by polling:
 * a zero stale time would refetch on every mount of every surface reading the
 * projection, and the stream already converges the revision.
 */
const CATALOG_STALE_TIME = 60_000;
const CLIENTS_STALE_TIME = 15_000;

export const cmdPaletteCatalogOptions = (
  workspaceId: string | null,
  clientId: string | null,
  enabled = true
) => {
  const workspace = workspaceId?.trim() ?? "";
  return queryOptions({
    queryKey: cmdPaletteKeys.catalog(workspace, clientId),
    queryFn: ({ signal }) => listCmdPaletteCommands(workspace, clientId, signal),
    enabled: enabled && workspace !== "",
    staleTime: CATALOG_STALE_TIME,
  });
};

export const cmdPaletteClientsOptions = (workspaceId: string | null, enabled = true) => {
  const workspace = workspaceId?.trim() ?? "";
  return queryOptions({
    queryKey: cmdPaletteKeys.clients(workspace),
    queryFn: ({ signal }) => listCmdPaletteClients(workspace, signal),
    enabled: enabled && workspace !== "",
    staleTime: CLIENTS_STALE_TIME,
  });
};
