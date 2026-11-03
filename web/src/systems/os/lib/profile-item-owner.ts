import type { QueryClient } from "@tanstack/react-query";

import { agentDetailOptions, isAgentNotFound } from "@/systems/agent";
import {
  AutomationApiError,
  automationJobDetailOptions,
  automationTriggerDetailOptions,
} from "@/systems/automation";
import { ownerFromRow } from "@/systems/profiles";
import { SessionNotFoundError, sessionAcrossProfilesOptions } from "@/systems/session";
import {
  terminalAggregateQuery,
  terminalAggregateScope,
  terminalCatalogQuery,
  terminalScope,
} from "@/systems/terminal";
import { workspaceSelectorOrAll, type DataScope } from "@/systems/workspace";

import type { ProfileOwnerRef, ShownProfileItem } from "./foreign-profile-windows";

const ALL_PROFILES = { all_profiles: true } as const;

/** The item is gone: no owner to read, and nothing to retry. */
function isGone(error: unknown): boolean {
  return (
    error instanceof SessionNotFoundError ||
    isAgentNotFound(error) ||
    (error instanceof AutomationApiError && error.status === 404)
  );
}

/**
 * Reads who owns the item a window shows, through each kind's own read, widened across profiles
 * where the daemon offers it. Every read reuses the owning system's query factory, so the sweep
 * shares the cache a window would read. A gone item answers `null`; any other failed read throws,
 * so the sweep can tell an unknown owner from a missing item.
 */
export function profileItemOwnerLookup(queryClient: QueryClient) {
  return async (item: ShownProfileItem): Promise<ProfileOwnerRef | null> => {
    try {
      return await readOwner(queryClient, item);
    } catch (error) {
      if (isGone(error)) return null;
      throw error;
    }
  };
}

async function readOwner(
  queryClient: QueryClient,
  item: ShownProfileItem
): Promise<ProfileOwnerRef | null> {
  switch (item.kind) {
    case "session":
      return {
        id: ownerFromRow(await queryClient.fetchQuery(sessionAcrossProfilesOptions(item.sessionId)))
          .id,
      };
    case "terminal":
      return terminalOwner(queryClient, item.terminalId, item.scope);
    case "agent":
      // The window reads its definition only in its pinned profile; finding it there is the
      // owner read. A missing definition is gone and the window stays.
      await queryClient.fetchQuery(
        agentDetailOptions(item.name, workspaceSelectorOrAll(item.scope), item.profile)
      );
      return { name: item.profile };
    case "automation-job":
      return {
        id: (await queryClient.fetchQuery(automationJobDetailOptions(item.jobId, ALL_PROFILES)))
          .profile_id,
      };
    case "automation-trigger":
      return {
        id: (
          await queryClient.fetchQuery(automationTriggerDetailOptions(item.triggerId, ALL_PROFILES))
        ).profile_id,
      };
  }
}

/**
 * A terminal is read one profile at a time, so its owner comes from the catalog of the scope the
 * window reads, across profiles: the window's project or User, or every project for an unbound
 * window on the All projects desk.
 */
async function terminalOwner(
  queryClient: QueryClient,
  terminalId: string,
  scope: DataScope
): Promise<ProfileOwnerRef | null> {
  const selector = workspaceSelectorOrAll(scope);
  const terminals =
    selector === null
      ? await queryClient.fetchQuery(terminalAggregateQuery(terminalAggregateScope([], "", true)))
      : await queryClient.fetchQuery(terminalCatalogQuery(terminalScope(selector, "", true)));
  const terminal = terminals.find(row => row.id === terminalId);
  return terminal === undefined ? null : { id: terminal.profile_id };
}
