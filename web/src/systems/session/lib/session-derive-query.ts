import { queryOptions } from "@tanstack/react-query";

import { fetchSessionDerivePreview } from "../adapters/session-derive-api";
import { sessionKeys } from "./query-keys";

/**
 * The derive preview is measured when a dialog opens, never reused from an
 * earlier open: the size on screen must describe the transcript as it is now.
 * A failed measurement disables the primary, so it surfaces without retries.
 */
export function sessionDerivePreviewOptions(
  workspaceId: string,
  sessionId: string,
  messageId?: string
) {
  return queryOptions({
    queryKey: sessionKeys.derivePreview(workspaceId, sessionId, messageId),
    queryFn: ({ signal }) =>
      fetchSessionDerivePreview(workspaceId, sessionId, { messageId }, signal),
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    enabled: workspaceId.trim() !== "" && sessionId.trim() !== "",
  });
}
