import { useRef } from "react";

import { SessionApiError } from "../adapters/session-api";
import { SessionBusyInputRefusalError } from "../lib/session-busy-input-refusal";
import type {
  ClarificationPending,
  SessionBusyInputDraft,
  SessionBusyInputHandler,
} from "../types";
import { isClarificationTerminalGone } from "./use-clarification-dock";
import {
  useAnswerSessionClarification,
  useSessionClarifications,
} from "./use-session-clarifications";

export interface UseClarificationComposerAnswerOptions {
  enabled?: boolean;
  /** A read-only surface renders no composer, so it reads nothing. */
  readOnly?: boolean;
  /** The composer takes busy sends at all; without them nothing is routed here. */
  allowBusyInput?: boolean;
  /** A stopping turn cancels its question, so a send then is the operator's follow-up. */
  stopPhase?: string;
  sessionId: string;
  /** Absent for hosts without a workspace: nothing is read and sends pass through. */
  workspaceId?: string;
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && "name" in error && error.name === "AbortError"
  );
}

function refusal(message: string, draft: SessionBusyInputDraft): SessionBusyInputRefusalError {
  return new SessionBusyInputRefusalError({
    attachmentCount: draft.attachments.length,
    code: "not_delivered",
    message,
  });
}

/**
 * Routes a main-composer send to the session's pending clarification (#1821).
 *
 * While the agent waits on a question, its turn cannot reach the next step that
 * would take a steer or end so a queued prompt can run. So a steer or queue
 * send becomes the question's free-text answer instead, through the same
 * answer endpoint as the dock, and the turn resumes at once. A question that
 * settled first hands the send to the next pending question, or, when none
 * remains, to the original steer or queue. Interrupt keeps its own meaning: it
 * cancels the turn, which cancels the question.
 */
export function useClarificationComposerAnswer({
  enabled = true,
  readOnly = false,
  allowBusyInput = true,
  stopPhase = "idle",
  sessionId,
  workspaceId = "",
}: UseClarificationComposerAnswerOptions) {
  const clarifications = useSessionClarifications(workspaceId, sessionId, {
    enabled: enabled && !readOnly,
  });
  const answerMutation = useAnswerSessionClarification(workspaceId, sessionId);
  const routable = enabled && !readOnly && allowBusyInput && stopPhase !== "stopping";
  // Answers whose acknowledgment was lost, by request id. A later send of the same text that
  // finds the question settled is that answer landing, never a second prompt.
  const unconfirmedAnswers = useRef(new Map<string, string>());
  const pending: ClarificationPending | null =
    routable && !clarifications.error ? (clarifications.data?.[0] ?? null) : null;

  /** The live question after a re-read; `null` when none waits any more. */
  const rereadPending = async (): Promise<ClarificationPending | null> => {
    const reread = await clarifications.refetch({ throwOnError: true });
    return reread.data?.[0] ?? null;
  };

  /** Answers `requestId`; `false` when that question settled before the answer landed. */
  const answer = async (
    requestId: string,
    text: string,
    draft: SessionBusyInputDraft
  ): Promise<boolean> => {
    try {
      await answerMutation.mutateAsync({ requestId, body: { text } });
      unconfirmedAnswers.current.delete(requestId);
      return true;
    } catch (error) {
      if (isClarificationTerminalGone(error)) {
        const landed = unconfirmedAnswers.current.get(requestId) === text;
        unconfirmedAnswers.current.delete(requestId);
        return landed;
      }
      if (isAbortError(error)) throw error;
      if (error instanceof SessionApiError && error.status === 400) {
        throw refusal("the agent's question only accepts a listed choice. Pick one above.", draft);
      }
      // Admission is unknown (transport, 5xx): the live projection says whether it landed.
      const live = await rereadPending().catch(() => undefined);
      if (live === undefined) {
        unconfirmedAnswers.current.set(requestId, text);
        throw refusal(
          "couldn't confirm your answer reached the agent's question. Send it again; it never answers twice.",
          draft
        );
      }
      if (live?.request_id !== requestId) return true;
      throw refusal("couldn't send your answer to the agent's question. Try again.", draft);
    }
  };

  const answering = (
    handler: SessionBusyInputHandler | undefined
  ): SessionBusyInputHandler | undefined => {
    if (!handler || !pending) return handler;
    return async draft => {
      if (draft.attachments.length > 0) {
        throw refusal(
          "an answer to the agent's question can't carry files. Remove them to answer.",
          draft
        );
      }
      const text = draft.message.trim();
      if (text.length === 0) return handler(draft);
      if (await answer(pending.request_id, text, draft)) return;
      // The docked question settled first (answered in the dock, timed out): the next one in
      // line still blocks the turn, so it takes this answer; with none left the send goes out.
      const next = await rereadPending().catch(() => null);
      if (next && next.request_id !== pending.request_id) {
        if (await answer(next.request_id, text, draft)) return;
      }
      return handler(draft);
    };
  };

  return {
    pending,
    answering,
    placeholder: pending ? "Type here to answer the agent's question" : undefined,
  };
}
