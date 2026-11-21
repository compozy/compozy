import { infiniteQueryOptions } from "@tanstack/react-query";

import { fetchSessionInheritedTranscript } from "../adapters/session-api";
import { SessionApiError } from "../adapters/session-api-errors";
import type { SessionPayload } from "../types";
import { sessionKeys } from "./query-keys";
import { sessionOriginKind } from "./session-origin";
import {
  nextTranscriptPageParam,
  transcriptPageFromResponse,
  transcriptPageMatchesFence,
  transcriptPageRequest,
  type SessionTranscriptPageParam,
} from "./session-transcript-query";

/** Inherited entries per page: the tail first, older pages on demand. */
export const INHERITED_TRANSCRIPT_PAGE_LIMIT = 50;

/**
 * What a derived child shows of the conversation it inherited (#1885 C). The
 * displayed rows are read from the source through the derive cut; what the
 * agent received is the bounded replay (or the native clone), so the two are
 * worded apart and never inferred from each other.
 */
export interface SessionInheritedView {
  /** The derived child: the inherited read is addressed through it. */
  sessionId: string;
  workspaceId: string;
  /** Collapsed sentence: `Earlier conversation · 42 messages`. */
  label: string;
  /** What the agent got: `The agent gets the last 30 of 42 messages.`; `null` when unrecorded. */
  agentLine: string | null;
  /** The agent holds a native copy of the source, not the replayed carry. */
  nativeClone: boolean;
}

function messages(count: number): string {
  return count === 1 ? "1 message" : `${count.toLocaleString()} messages`;
}

function nonNegative(value: number | null | undefined): number | null {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : null;
}

function isNativeClone(derivation: NonNullable<SessionPayload["derivation"]>): boolean {
  return derivation.seed === "native_fork" && derivation.native_state !== "failed";
}

function agentLine(derivation: NonNullable<SessionPayload["derivation"]>): string | null {
  if (isNativeClone(derivation))
    return "The agent continues from its own copy of this conversation.";
  const received = nonNegative(derivation.replay_message_count);
  if (received === null) return null;
  const total = received + (nonNegative(derivation.replay_omitted_count) ?? 0);
  if (received < total) {
    return `The agent gets the last ${received.toLocaleString()} of ${messages(total)}.`;
  }
  return received === 1
    ? "The agent gets this message."
    : `The agent gets all ${messages(received)}.`;
}

/**
 * The inherited block of a continued or forked child, or `null` for every other
 * session and for a derive that carried nothing (a fork cut before any message).
 * A derive recorded before the count existed still shows the block, uncounted.
 */
export function sessionInheritedView(
  session: SessionPayload,
  workspaceId: string
): SessionInheritedView | null {
  const derivation = session.derivation;
  if (sessionOriginKind(session) === null || !derivation) return null;
  const count = nonNegative(derivation.inherited_message_count);
  if (count === 0) return null;
  return {
    sessionId: session.id,
    workspaceId,
    label: count === null ? "Earlier conversation" : `Earlier conversation · ${messages(count)}`,
    agentLine: agentLine(derivation),
    nativeClone: isNativeClone(derivation),
  };
}

/**
 * The source is gone for this reader: deleted, archived, moved, rewound or
 * cleared past the cut, or outside the reader's scope. The daemon answers 404
 * (`inherited_transcript_unavailable`); a refused scope answers 403.
 */
export function isInheritedTranscriptUnavailable(error: unknown): boolean {
  return error instanceof SessionApiError && (error.status === 404 || error.status === 403);
}

/**
 * The inherited range as an infinite query: the head page is the newest
 * entries before the cut, older pages continue by `before_sequence`. The range
 * never grows, so nothing refetches it on focus; a source that moved under an
 * older page fails that page instead of mixing two histories.
 */
export function sessionInheritedTranscriptOptions(workspaceId: string, sessionId: string) {
  return infiniteQueryOptions({
    queryKey: sessionKeys.inheritedTranscript(workspaceId, sessionId),
    queryFn: async ({ pageParam, signal }) => {
      const response = await fetchSessionInheritedTranscript(
        workspaceId,
        sessionId,
        { ...transcriptPageRequest(pageParam), limit: INHERITED_TRANSCRIPT_PAGE_LIMIT },
        signal
      );
      if (!transcriptPageMatchesFence(response, pageParam)) {
        throw new Error("The source conversation changed while loading earlier messages");
      }
      return transcriptPageFromResponse(response);
    },
    initialPageParam: undefined as SessionTranscriptPageParam,
    getNextPageParam: nextTranscriptPageParam,
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
    enabled: sessionId !== "",
  });
}
