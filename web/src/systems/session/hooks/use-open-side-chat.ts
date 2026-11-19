import { useMutation, useQueryClient } from "@tanstack/react-query";

import { createClientId } from "@/lib/client-id";

import { SessionApiError } from "../adapters/session-api-errors";
import { SessionDeriveCommittedError } from "../adapters/session-derive-api";
import { openSessionSide, type OpenSessionSideResult } from "../adapters/session-side-api";
import type { SessionWindowDrawerStore } from "../contexts/session-window-drawer-store";
import { sessionKeys } from "../lib/query-keys";
import { SIDE_CHAT_COPY } from "../lib/session-side";
import { isSessionSidesQuery } from "../lib/session-side-query-options";

/**
 * How an open settled: `opened` (the panel shows the side chat), `failed` (the panel keeps the
 * question with Retry), or `refused` (nothing was created; the reason reaches the operator and the
 * question stays where it was typed).
 */
export type SideChatOpenOutcome = "opened" | "failed" | "refused";

export function newSideChatKey(): string {
  return `web-side-${createClientId()}`;
}

/** A daemon refusal that created nothing: empty, archived or unsupported source, nested side. */
function isRefusal(error: unknown): error is SessionApiError {
  return (
    error instanceof SessionApiError &&
    !(error instanceof SessionDeriveCommittedError) &&
    error.status >= 400 &&
    error.status < 500
  );
}

function failureMessage(error: unknown): string {
  return error instanceof SessionApiError && error.code !== null && error.message.trim() !== ""
    ? error.message
    : SIDE_CHAT_COPY.openFailed;
}

function sideSessionId(result: OpenSessionSideResult): string {
  return result.session?.id ?? result.derived.child_session_id;
}

/**
 * The one open mutation behind `/side <question>` on both submit paths, the empty panel's first
 * question, and Retry (`_uiux.md` Component plan). The composer submission id is the idempotency
 * key: Retry re-sends the same key, so the same side chat receives the question once.
 */
export function useOpenSideChat({
  workspaceId,
  sessionId,
  store,
  onRefused,
}: {
  workspaceId: string;
  sessionId: string;
  store: SessionWindowDrawerStore;
  /** A refusal's reason, for the host to show where the question was typed. */
  onRefused: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const mutation = useMutation<OpenSessionSideResult, Error, { message: string; key: string }>({
    mutationFn: ({ message, key }) =>
      openSessionSide(workspaceId, sessionId, { message, idempotency_key: key }),
    onSuccess: result => {
      const side = result.session;
      if (side) queryClient.setQueryData(sessionKeys.detail(workspaceId, side.id), side);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({
        predicate: query => isSessionSidesQuery(query.queryKey, sessionId),
      });
    },
  });

  const open = async (
    message: string,
    key: string = newSideChatKey()
  ): Promise<SideChatOpenOutcome> => {
    const view = store.getSnapshot().context.side.view;
    // A second press of the same submission while it is on its way is the same request (EC-4).
    if (view?.kind === "opening" && view.error === null && view.message === message) {
      return "opened";
    }
    store.trigger.sideRequested({ message, key });
    try {
      const result = await mutation.mutateAsync({ message, key });
      store.trigger.sideSettled({ key, id: sideSessionId(result) });
      return "opened";
    } catch (error) {
      if (isRefusal(error)) {
        store.trigger.sideRefused({ key });
        onRefused(failureMessage(error));
        return "refused";
      }
      store.trigger.sideFailed({ key, error: failureMessage(error) });
      return "failed";
    }
  };

  /** Re-sends the failed question with its original key. */
  const retry = () => {
    const view = store.getSnapshot().context.side.view;
    if (view?.kind !== "opening" || view.error === null) return;
    void open(view.message, view.key);
  };

  return { open, retry, pending: mutation.isPending };
}
