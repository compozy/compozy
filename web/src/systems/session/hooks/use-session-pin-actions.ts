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
  /** The polite announcement of the last pin change: "Pinned <title>." */
  announcement: string;
}

/**
 * Pin to top and Unpin for a session list. The acting list rereads its catalogs at once rather
 * than waiting for the catalog stream, then moves focus to the session's row in its new place
 * (`focusSession`) and announces the change; other windows follow on the stream.
 */
export function useSessionPinActions(focusSession: (sessionId: string) => void): SessionPinActions {
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
      focusSession(session.id);
    },
    onError: (_error, { session, pinned }) => {
      const title = getSessionDisplayTitle(session);
      toast.error(
        pinned ? `Couldn't pin ${title}. Try again.` : `Couldn't unpin ${title}. Try again.`
      );
    },
  });
  return {
    onPin: session => mutation.mutate({ session, pinned: true }),
    onUnpin: session => mutation.mutate({ session, pinned: false }),
    announcement,
  };
}

/**
 * A session list's host actions with Pin to top and Unpin added, the list's content ref the focus
 * move searches, and the polite announcement to render: after a pin change settles, focus lands
 * on the session's row (`<testIdPrefix>-session-<id>`) in its new place.
 */
export function useSessionListPin(
  hostActions: SessionLifecycleActionHandlers,
  testIdPrefix: string
) {
  const contentRef = useRef<HTMLDivElement>(null);
  const pin = useSessionPinActions(sessionId => {
    requestAnimationFrame(() =>
      contentRef.current
        ?.querySelector<HTMLElement>(`[data-testid="${testIdPrefix}-session-${sessionId}"]`)
        ?.focus()
    );
  });
  return {
    contentRef,
    sessionActions: { ...hostActions, onPin: pin.onPin, onUnpin: pin.onUnpin },
    announcement: pin.announcement,
  };
}
