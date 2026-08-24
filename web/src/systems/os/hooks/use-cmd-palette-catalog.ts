import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { readCatalogRecord, writeCatalogRecord } from "../lib/cmd-palette-catalog-cache";
import { toCatalogRecord, type CmdPaletteCatalogRecord } from "../lib/cmd-palette-catalog-record";
import { cmdPaletteCatalogOptions } from "../lib/cmd-palette-query-options";
import { cmdPaletteKeys } from "../lib/cmd-palette-query-keys";
import type {
  CmdPaletteCatalogResponse,
  CmdPaletteStructuralCatalog,
} from "../lib/cmd-palette-types";
import { useProfileReadScope } from "@/systems/profiles";

export interface CmdPaletteCatalogState {
  /** Structure only; availability is the evaluator's job, never the cache's. */
  readonly catalog: CmdPaletteStructuralCatalog | null;
  /** The daemon's view of the targeted client's context, when it answered. */
  readonly contextRevision: string | null;
  /** True while the last-known catalog stands in for an unconfirmed daemon. */
  readonly stale: boolean;
  readonly daemonReachable: boolean;
  /** True until either the cache or the daemon has produced a first answer. */
  readonly hydrating: boolean;
}

export interface UseCmdPaletteCatalogOptions {
  readonly workspaceId: string | null;
  readonly clientId: string | null;
  readonly enabled?: boolean;
}

/**
 * Stale-while-revalidate hydration of the daemon catalog (BR-19).
 *
 * The last-known structural catalog renders the instant the palette opens; the
 * fetch then replaces it wholesale — never merged field by field, so a render is
 * always one consistent revision (Safety Invariant 3). A cold or reconnecting
 * daemon keeps the cached structure and reports itself unreachable, which is
 * what turns action rows into disabled-with-reason instead of a blank palette.
 */
export function useCmdPaletteCatalog({
  workspaceId,
  clientId,
  enabled = true,
}: UseCmdPaletteCatalogOptions): CmdPaletteCatalogState {
  const workspace = workspaceId?.trim() ?? "";
  // The catalog answers differently per profile — an extension enabled in one is
  // absent in another — so the lens keys the query and the offline record alike.
  const { key: profileKey } = useProfileReadScope();
  const queryClient = useQueryClient();
  const query = useQuery(cmdPaletteCatalogOptions(workspace, profileKey, clientId, enabled));
  const [cached, setCached] = useState<{
    workspaceId: string;
    record: CmdPaletteCatalogRecord | null;
    profileKey: string;
  } | null>(null);

  // IndexedDB is an external store, so reading it is effect work. Keyed by the
  // workspace and the profile: switching either must never show the previous
  // one's rows.
  useEffect(() => {
    if (!enabled || workspace === "") return undefined;
    let active = true;
    void readCatalogRecord(workspace, profileKey).then(record => {
      if (active) setCached({ workspaceId: workspace, profileKey, record });
    });
    return () => {
      active = false;
    };
  }, [enabled, profileKey, workspace]);

  const served = query.data;
  const workspaceCatalog =
    clientId !== null && query.isPending
      ? queryClient.getQueryData<CmdPaletteCatalogResponse>(
          cmdPaletteKeys.catalog(workspace, profileKey, null)
        )
      : undefined;
  const activeCatalog = served ?? workspaceCatalog;
  useEffect(() => {
    if (workspace === "" || served === undefined) return;
    void writeCatalogRecord(toCatalogRecord(workspace, profileKey, served));
  }, [profileKey, served, workspace]);

  const cacheHydrated =
    enabled && cached?.workspaceId === workspace && cached.profileKey === profileKey;
  const cachedRecord = cacheHydrated ? cached.record : null;
  const catalog: CmdPaletteStructuralCatalog | null =
    activeCatalog !== undefined
      ? toCatalogRecord(workspace, profileKey, activeCatalog)
      : cachedRecord;
  // A daemon that goes away mid-session leaves its last answer in the query
  // cache. Structure is still the best thing to render, but the *reachability*
  // it was resolved under is gone — reporting otherwise would leave action rows
  // enabled against a runtime that cannot serve them (US-001.EC-1).
  const daemonReachable =
    query.failureCount === 0 &&
    !query.isError &&
    (served !== undefined || workspaceCatalog !== undefined);
  return {
    catalog,
    contextRevision: activeCatalog?.context_revision?.trim() || null,
    stale: catalog !== null && !daemonReachable,
    daemonReachable,
    hydrating: activeCatalog === undefined && !cacheHydrated && enabled && workspace !== "",
  };
}
