import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";

import { addSessionWorkLink, removeSessionWorkLink } from "../adapters/session-linked-work-api";
import { sessionKeys } from "../lib/query-keys";
import { repositoryForgeName } from "../lib/session-link-input";
import {
  leadLink,
  linkKey,
  linkNumber,
  linkRequest,
  linkShortName,
  moreLabel,
  summaryMoreLabel,
  type SessionLinkedWorkItem,
  type SessionLinkedWorkResponse,
  type SessionWorkLinkRequest,
} from "../lib/session-linked-work";
import { sessionLinkedWorkOptions } from "../lib/session-linked-work-options";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";

/** The two list writes: the response replaces the list, and the catalog rereads its summary. */
export function useSessionLinkMutations(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.linkedWork(workspaceId, sessionId);
  const settle = (response: SessionLinkedWorkResponse) => {
    queryClient.setQueryData(queryKey, response);
    return invalidateSessionMutationQueries(queryClient, workspaceId, sessionId);
  };
  const add = useMutation({
    mutationFn: (request: SessionWorkLinkRequest) =>
      addSessionWorkLink(workspaceId, sessionId, request),
    onSuccess: settle,
  });
  const remove = useMutation({
    mutationFn: (item: SessionLinkedWorkItem) =>
      removeSessionWorkLink(
        workspaceId,
        sessionId,
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
    onError: (_error, _item, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSuccess: settle,
  });
  return { add, remove };
}

/** Says what did not happen, because that is what a person worries about after unlinking. */
function unlinkedDescription(item: SessionLinkedWorkItem): string {
  return linkNumber(item) !== null
    ? `It's still on ${repositoryForgeName(item.repository)}. Only the link to this session is gone.`
    : "Its files are untouched. Only the link to this session is gone.";
}

export interface SessionLinkedWorkModel {
  items: readonly SessionLinkedWorkItem[];
  lead: SessionLinkedWorkItem | null;
  more: string | null;
  refreshing: boolean;
  /** Whether the header's status zone has clipped the chip, moving the list to the actions zone. */
  clipped: boolean;
  setClipped: (clipped: boolean) => void;
  refresh: () => void;
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
    enabled: enabled && !!session.id,
  });
  const { add, remove } = useSessionLinkMutations(workspaceId, session.id);
  const summary = session.linked_work;
  const items = list.data?.items ?? (summary?.lead ? [summary.lead] : []);

  const unlink = (item: SessionLinkedWorkItem) => {
    remove.mutate(item, {
      onSuccess: () => {
        toast.message(`Unlinked ${linkShortName(item)}`, {
          description: unlinkedDescription(item),
          action: { label: "Undo", onClick: () => add.mutate(linkRequest(item)) },
        });
      },
      onError: () => {
        toast.error(`Couldn't unlink ${linkShortName(item)}.`);
      },
    });
  };

  return {
    items,
    lead: list.data ? leadLink(list.data.items) : (summary?.lead ?? null),
    more: list.data ? moreLabel(list.data.items) : summary ? summaryMoreLabel(summary) : null,
    refreshing: list.isFetching,
    clipped,
    setClipped,
    refresh: () => {
      void list.refetch();
    },
    unlink,
  };
}
