import { infiniteQueryOptions } from "@tanstack/react-query";

import { fetchSessionTranscript } from "../adapters/session-api";
import { sessionKeys } from "./query-keys";
import {
  SESSION_TRANSCRIPT_STALE_TIME_MS,
  SESSION_WARM_CACHE_POLICY,
} from "./session-query-policy";
import {
  nextTranscriptPageParam,
  reconcileRefetchedTranscriptHead,
  transcriptPageFromResponse,
  transcriptPageMatchesFence,
  transcriptPageRequest,
  type SessionTranscriptData,
  type SessionTranscriptPageParam,
} from "./session-transcript-query";

/**
 * The transcript as an infinite query: the head page is the latest window, older
 * pages continue by `before_sequence` in `pageParam` and never change. Once
 * loaded, the head is owned by the live stream: a refetch (a mutation's reread,
 * a focus refetch) reconciles with what the stream applied meanwhile instead of
 * replacing it, because the stream will not carry those entries again.
 */
export function sessionTranscriptOptions(workspace: string, id: string) {
  const queryKey = sessionKeys.transcript(workspace, id);
  return infiniteQueryOptions({
    queryKey,
    queryFn: async ({ client, pageParam, signal }) => {
      const response = await fetchSessionTranscript(
        workspace,
        id,
        transcriptPageRequest(pageParam),
        signal
      );
      if (!transcriptPageMatchesFence(response, pageParam)) {
        throw new Error("Session transcript changed while loading older messages");
      }
      const page = transcriptPageFromResponse(response);
      return pageParam === undefined
        ? reconcileRefetchedTranscriptHead(
            client.getQueryData<SessionTranscriptData>(queryKey),
            page
          )
        : page;
    },
    initialPageParam: undefined as SessionTranscriptPageParam,
    getNextPageParam: nextTranscriptPageParam,
    staleTime: SESSION_TRANSCRIPT_STALE_TIME_MS,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!workspace && !!id,
  });
}
