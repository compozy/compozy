import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { toast } from "@compozy/ui";

import { setSessionPinned } from "../adapters/session-pin-api";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./session-lifecycle-types";
import { requireWorkspace } from "./session-action-workspace";
import { useAbortableMutationRequest } from "./use-abortable-mutation-request";

export interface SessionPinActions {
  onPin: (session: SessionPayload) => void;
  onUnpin: (session: SessionPayload) => void;
  /** The pin change in flight, until its catalogs reread: its row's verbs wait for it. */
  pending: { action: "pin" | "unpin"; sessionId: string } | null;
  /** The polite announcement of the last pin change: "Pinned <title>." */
  announcement: string;
}

/**
 * Pin to top and Unpin for a session list. The acting list rereads its catalogs at once rather
 * than waiting for the catalog stream, then moves focus to the session's row in its new place
 * (`focusSession`) and announces the change; other windows follow on the stream.
 */
export function useSessionPinActions(
  focusSession: (sessionId: string, pinned: boolean) => void
): SessionPinActions {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();
  const [announcement, setAnnouncement] = useState("");
  const mutation = useMutation({
    mutationFn: ({ session, pinned }: { session: SessionPayload; pinned: boolean }) =>
      request(signal =>
        setSessionPinned(
          requireWorkspace(session.workspace_id ?? ""),
          session.id,
          session.profile_name,
          pinned,
          signal
        )
      ),
    onSuccess: async (_data, { session, pinned }) => {
      const workspace = requireWorkspace(session.workspace_id ?? "");
      await invalidateSessionMutationQueries(queryClient, workspace, session.id);
      const title = getSessionDisplayTitle(session);
      setAnnouncement(pinned ? `Pinned ${title}.` : `Unpinned ${title}.`);
      focusSession(session.id, pinned);
    },
    onError: (_error, { session, pinned }) => {
      const title = getSessionDisplayTitle(session);
      toast.error(
        pinned ? `Couldn't pin ${title}. Try again.` : `Couldn't unpin ${title}. Try again.`
      );
    },
  });
  const pending =
    mutation.isPending && mutation.variables
      ? {
          action: mutation.variables.pinned ? ("pin" as const) : ("unpin" as const),
          sessionId: mutation.variables.session.id,
        }
      : null;
  return {
    onPin: session => mutation.mutate({ session, pinned: true }),
    onUnpin: session => mutation.mutate({ session, pinned: false }),
    pending,
    announcement,
  };
}

/**
 * A session list's host actions with Pin to top and Unpin added, the list's content ref the focus
 * move searches, and the polite announcement to render: after a pin change settles, focus lands
 * on the session's row (`<testIdPrefix>-session-<id>`) in its new place. A row its new place does
 * not show (a folded Pinned section or group) hands focus to the Pinned header after a pin, else
 * to the list's filter field, never to the page. While a change is in flight its row's verbs wait.
 */
export function useSessionListPin(
  hostActions: SessionLifecycleActionHandlers,
  testIdPrefix: string
) {
  const contentRef = useRef<HTMLDivElement>(null);
  const pin = useSessionPinActions((sessionId, pinned) => {
    requestAnimationFrame(() => {
      const list = contentRef.current;
      if (!list) return;
      const row = list.querySelector<HTMLElement>(
        `[data-testid="${testIdPrefix}-session-${sessionId}"]`
      );
      const header = pinned
        ? list.querySelector<HTMLElement>(`[data-testid="${testIdPrefix}-pinned-toggle"]`)
        : null;
      const filter = list.querySelector<HTMLElement>('input[aria-label="Filter sessions"]');
      (row ?? header ?? filter)?.focus();
    });
  });
  const pending = pin.pending;
  return {
    contentRef,
    sessionActions: {
      ...hostActions,
      onPin: pin.onPin,
      onUnpin: pin.onUnpin,
      pendingAction: hostActions.pendingAction ?? pending?.action ?? null,
      pendingSessionIds: pending
        ? [...hostActions.pendingSessionIds, pending.sessionId]
        : hostActions.pendingSessionIds,
    },
    announcement: pin.announcement,
  };
}
