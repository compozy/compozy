import { infiniteQueryOptions } from "@tanstack/react-query";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { fetchSessionTranscript } from "../adapters/session-api";
import { sessionKeys } from "./query-keys";
import { isSessionStreamLive } from "./session-stream-liveness";
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
import { shareTranscriptData } from "./session-transcript-sharing";

/**
 * The transcript as an infinite query: the head page is the latest window, older
 * pages continue by `before_sequence` in `pageParam` and never change. Once
 * loaded, the head is owned by the live stream: a refetch (a mutation's reread,
 * a focus refetch) reconciles with what the stream applied meanwhile instead of
 * replacing it, because the stream will not carry those entries again.
 */
export function sessionTranscriptOptions(workspace: string, id: string, computerId?: ComputerId) {
  const queryKey = sessionKeys.forComputer(computerId).transcript(workspace, id);
  // A caught-up stream owns this page set. Refocusing an idle live session must
  // not walk its retained history again; explicit invalidations still reread it.
  const shouldRefetch = (query: { state: { isInvalidated: boolean } }) =>
    query.state.isInvalidated || !isSessionStreamLive(id, computerId);
  return infiniteQueryOptions({
    queryKey,
    queryFn: async ({ client, pageParam, signal }) => {
      const request = transcriptPageRequest(pageParam);
      const response = isRemoteComputerId(computerId)
        ? await fetchSessionTranscript(workspace, id, request, signal, computerId)
        : await fetchSessionTranscript(workspace, id, request, signal);
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
    // Live entries re-bound the pages; share unchanged entries by sequence, not position.
    structuralSharing: shareTranscriptData,
    staleTime: SESSION_TRANSCRIPT_STALE_TIME_MS,
    refetchOnMount: shouldRefetch,
    refetchOnWindowFocus: shouldRefetch,
    ...SESSION_WARM_CACHE_POLICY,
    enabled: !!id,
  });
}
