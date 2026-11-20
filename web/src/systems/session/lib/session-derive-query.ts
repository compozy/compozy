import { queryOptions } from "@tanstack/react-query";

import { fetchSessionDerivePreview } from "../adapters/session-derive-api";
import {
  SESSION_DERIVE_PREVIEW_TIMEOUT_MS,
  sessionDeriveDeadline,
} from "./session-derive-deadline";
import { sessionKeys } from "./query-keys";

/**
 * The derive preview is measured when a dialog opens, never reused from an
 * earlier open: the size on screen must describe the transcript as it is now.
 * The wait is bounded; a failed or timed-out measurement surfaces at once with
 * the line's own Try again, so it is never retried behind the operator's back.
 */
export function sessionDerivePreviewOptions(
  workspaceId: string,
  sessionId: string,
  messageId?: string
) {
  return queryOptions({
    queryKey: sessionKeys.derivePreview(workspaceId, sessionId, messageId),
    queryFn: ({ signal }) =>
      fetchSessionDerivePreview(
        workspaceId,
        sessionId,
        { messageId },
        sessionDeriveDeadline(SESSION_DERIVE_PREVIEW_TIMEOUT_MS, signal)
      ),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    enabled: workspaceId.trim() !== "" && sessionId.trim() !== "",
  });
}
