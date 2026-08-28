import { queryOptions } from "@tanstack/react-query";

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
  aggregate?: false
): TerminalProfileQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate: true
): TerminalQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate: boolean
): TerminalQueryScope;
export function terminalScope(
  workspaceId: string,
  profile: string,
  aggregate = false
): TerminalQueryScope {
  return aggregate
    ? {
        key: { workspaceId, profileKey: TERMINAL_ALL_PROFILES_KEY },
        params: { all_profiles: true },
      }
    : { key: { workspaceId, profileKey: profile }, params: { profile } };
}

export interface TerminalQueryScope {
  key: TerminalScopeKey;
  params: TerminalScopeParams;
}

export interface TerminalProfileQueryScope extends TerminalQueryScope {
  params: TerminalProfileScopeParams;
}

export function terminalCatalogQuery(scope: TerminalQueryScope) {
  return queryOptions({
    queryKey: terminalKeys.catalog(scope.key),
    queryFn: ({ signal }) => fetchTerminals(scope.key.workspaceId, scope.params, signal),
  });
}
