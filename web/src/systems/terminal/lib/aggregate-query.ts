import { queryOptions } from "@tanstack/react-query";

import { fetchTerminalsAggregate } from "../adapters/terminal-api";
import type { TerminalScopeParams } from "../types";
import { TERMINAL_ALL_PROFILES_KEY, terminalKeys } from "./query-keys";

/**
 * One All projects terminal selection: the normalized Project filter plus the
 * profile lens. The selection is the cache key and the stream cursor's
 * fingerprint, so any change starts a fresh list and a fresh subscription.
 */
export interface TerminalAggregateScope {
  workspaceIds: readonly string[];
  /** `workspaceIds` joined in order; `""` is every project and User. */
  selection: string;
  profileKey: string;
  params: TerminalScopeParams;
}

export function terminalAggregateScope(
  workspaceIds: readonly string[],
  profile: string,
  aggregate: boolean
): TerminalAggregateScope {
  const unique = new Set<string>();
  for (const value of workspaceIds) {
    const id = value.trim();
    if (id !== "") unique.add(id);
  }
  const ids = [...unique].sort();
  return {
    workspaceIds: ids,
    selection: ids.join(","),
    profileKey: aggregate ? TERMINAL_ALL_PROFILES_KEY : profile,
    params: aggregate ? { all_profiles: true } : { profile },
  };
}

export function terminalAggregateQuery(scope: TerminalAggregateScope) {
  return queryOptions({
    queryKey: terminalKeys.aggregate(scope.selection, scope.profileKey),
    queryFn: ({ signal }) => fetchTerminalsAggregate(scope.workspaceIds, scope.params, signal),
  });
}

/** The aggregate catalog stream; its cursor is bound to this selection by the daemon. */
export function terminalAggregateStreamPath(scope: TerminalAggregateScope): string {
  const query = new URLSearchParams({ all_workspaces: "true" });
  for (const id of scope.workspaceIds) query.append("workspace_ids", id);
  if (scope.params.all_profiles) query.set("all_profiles", "true");
  else if (scope.params.profile) query.set("profile", scope.params.profile);
  return `/api/terminals/catalog-stream?${query.toString()}`;
}
