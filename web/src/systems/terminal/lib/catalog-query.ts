import { queryOptions } from "@tanstack/react-query";

import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

import { fetchTerminals } from "../adapters/terminal-api";
import type { TerminalProfileScopeParams, TerminalScopeKey, TerminalScopeParams } from "../types";
import { TERMINAL_ALL_PROFILES_KEY, terminalKeys } from "./query-keys";

/**
 * Turns a read scope into both halves the cache and the query need.
 *
 * Session transcript blocks always pass a concrete profile. The aggregate
 * form is the Terminal app's all-profiles view — never a session subscribe.
 */
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate?: false,
  computerId?: ComputerId
): TerminalProfileQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate: true,
  computerId?: ComputerId
): TerminalQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate: boolean,
  computerId?: ComputerId
): TerminalQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate = false,
  computerId?: ComputerId
): TerminalQueryScope {
  const normalized = normalizeComputerId(computerId);
  return aggregate
    ? {
        key: { workspaceId, profileKey: TERMINAL_ALL_PROFILES_KEY },
        params: { all_profiles: true },
        computerId: normalized,
      }
    : {
        key: { workspaceId, profileKey: profile },
        params: { profile },
        computerId: normalized,
      };
}

export interface TerminalQueryScope {
  key: TerminalScopeKey;
  params: TerminalScopeParams;
  computerId?: ComputerId;
}

export interface TerminalProfileQueryScope extends TerminalQueryScope {
  params: TerminalProfileScopeParams;
}

export function terminalCatalogQuery(scope: TerminalQueryScope) {
  const keys = terminalKeys.forComputer(scope.computerId);
  return queryOptions({
    queryKey: keys.catalog(scope.key),
    queryFn: ({ signal }) =>
      fetchTerminals(scope.key.workspaceId, scope.params, signal, scope.computerId),
  });
}
