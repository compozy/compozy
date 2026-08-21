import { useQuery } from "@tanstack/react-query";

import { cmdPaletteRankSignalsOptions } from "../lib/cmd-palette-query-options";
import type { CmdPaletteRankSignals } from "../lib/cmd-palette-types";

export interface CmdPaletteRankSignalsState {
  readonly data: CmdPaletteRankSignals | null;
  readonly loading: boolean;
  readonly failed: boolean;
}

/**
 * The scorer snapshot lives only in TanStack Query's in-memory cache. Unlike
 * the structural catalog, it never enters the IndexedDB catalog record.
 */
export function useCmdPaletteRankSignals(
  workspaceId: string | null,
  enabled: boolean
): CmdPaletteRankSignalsState {
  const query = useQuery(cmdPaletteRankSignalsOptions(workspaceId, enabled));
  return {
    data: query.data ?? null,
    loading: query.isPending && enabled,
    failed: query.isError,
  };
}
