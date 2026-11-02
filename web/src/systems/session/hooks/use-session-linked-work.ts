import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  addSessionWorkLink,
  refreshSessionWorkLink,
  removeSessionWorkLink,
} from "../adapters/session-linked-work-api";
import { sessionKeys } from "../lib/query-keys";
import {
  leadLink,
  linkKey,
  linkRequest,
  linkShortName,
  linkUnlinkedNote,
  moreLabel,
  summaryMoreLabel,
  type SessionLinkedWorkItem,
  type SessionLinkedWorkResponse,
  type SessionWorkLinkRequest,
} from "../lib/session-linked-work";
import { sessionLinkedWorkOptions } from "../lib/session-linked-work-options";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { PULL_REQUEST_REREAD_INTERVAL_MS } from "../lib/session-pull-request";
import type { SessionPayload } from "../types";

/**
 * The two list writes: the response replaces the list, and the catalog rereads its summary. An
 * unlink says how it went from here, not from the caller: `mutate` keeps only the latest call's
 * callbacks, and a burst must not lose an earlier row's toast and Undo.
 */
export function useSessionLinkMutations(workspaceId: string, sessionId: string, profile: string) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.linkedWork(workspaceId, sessionId);
  const reread = () => invalidateSessionMutationQueries(queryClient, workspaceId, sessionId);
  const settle = (response: SessionLinkedWorkResponse) => {
    queryClient.setQueryData(queryKey, response);
    return reread();
  };
  const add = useMutation({
    mutationFn: (request: SessionWorkLinkRequest) =>
      addSessionWorkLink(workspaceId, sessionId, profile, request),
    onSuccess: settle,
  });
  const refresh = useMutation({
    mutationFn: (item: SessionLinkedWorkItem) =>
      refreshSessionWorkLink(
        workspaceId,
        sessionId,
        profile,
        linkRequest({ repository: item.repository, branch: item.branch })
      ),
    onError: (_error, item) => {
      toast.error(`Couldn't refresh ${linkShortName(item)}.`);
    },
    onSuccess: settle,
  });
  const remove = useMutation({
    mutationFn: (item: SessionLinkedWorkItem) =>
      removeSessionWorkLink(
        workspaceId,
        sessionId,
        profile,
        linkRequest({ repository: item.repository, branch: item.branch })
      ),
    onMutate: async item => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<SessionLinkedWorkResponse>(queryKey);
      if (previous) {
        const key = linkKey(item);
        queryClient.setQueryData<SessionLinkedWorkResponse>(queryKey, {
          ...previous,
          items: previous.items.filter(current => linkKey(current) !== key),
        });
      }
      return { previous };
    },
    onError: (_error, item, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
      toast.error(`Couldn't unlink ${linkShortName(item)}.`);
    },
    onSuccess: (response, item) => {
      queryClient.setQueryData(queryKey, response);
      toast.message(`Unlinked ${linkShortName(item)}`, {
        description: linkUnlinkedNote(item),
        action: {
          label: "Undo",
          // A toast outlives a session switch, so Undo stays on the session it came from.
          onClick: () => {
            addSessionWorkLink(workspaceId, sessionId, profile, linkRequest(item))
              .then(settle)
              .catch(() => {
                toast.error(`Couldn't link ${linkShortName(item)} again.`);
              });
          },
        },
      });
    },
    // A snapshot predates the unlinks after it, so only the daemon's list can settle a burst.
    onSettled: reread,
  });
  return { add, remove, refresh };
}

export interface SessionLinkedWorkModel {
  items: readonly SessionLinkedWorkItem[];
  lead: SessionLinkedWorkItem | null;
  more: string | null;
  refreshing: boolean;
  /** Whether the header's status zone has clipped the chip, moving the list to the actions zone. */
  clipped: boolean;
  setClipped: (clipped: boolean) => void;
  refresh: (item: SessionLinkedWorkItem) => void;
  unlink: (item: SessionLinkedWorkItem) => void;
}

interface UseSessionLinkedWorkInput {
  session: SessionPayload;
  workspaceId: string;
  enabled: boolean;
}

/** The session window's list: the catalog summary answers first, then the full read takes over. */
export function useSessionLinkedWork({
  session,
  workspaceId,
  enabled,
}: UseSessionLinkedWorkInput): SessionLinkedWorkModel {
  const [clipped, setClipped] = useState(false);
  const list = useQuery({
    ...sessionLinkedWorkOptions(workspaceId, session.id),
    // A window that isn't live still shows the whole list; it just stops rereading it.
    enabled: !!session.id,
    refetchInterval: enabled ? PULL_REQUEST_REREAD_INTERVAL_MS : false,
  });
  const { remove, refresh } = useSessionLinkMutations(
    workspaceId,
    session.id,
    session.profile_name
  );
  const summary = session.linked_work;
  const items = list.data?.items ?? (summary?.lead ? [summary.lead] : []);

  return {
    items,
    lead: list.data ? leadLink(list.data.items) : (summary?.lead ?? null),
    more: list.data ? moreLabel(list.data.items) : summary ? summaryMoreLabel(summary) : null,
    // Only what the person asked for: background rereads must not spin the chip every minute.
    refreshing: refresh.isPending,
    clipped,
    setClipped,
    refresh: item => refresh.mutate(item),
    unlink: item => remove.mutate(item),
  };
}
