import { useState } from "react";

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

/** Re-reads after a settled question before the send stops chasing the agent's next one. */
const NEXT_QUESTION_ATTEMPTS = 3;

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
  // Answers whose acknowledgment was lost, by request id. A later send of the same text replays
  // that answer: when the question settled meanwhile, it was that answer landing, never a prompt.
  const [unconfirmedAnswers, setUnconfirmedAnswers] = useState<ReadonlyMap<string, string>>(
    () => new Map()
  );
  const recordUnconfirmed = (requestId: string, text: string | null) =>
    setUnconfirmedAnswers(previous => {
      const next = new Map(previous);
      if (text === null) next.delete(requestId);
      else next.set(requestId, text);
      return next;
    });
  // A failed re-read keeps the last known question: a send still takes the answer path, which
  // re-reads before it lets anything through, so a waiting question is never bypassed.
  const pending: ClarificationPending | null = routable ? (clarifications.data?.[0] ?? null) : null;

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
      recordUnconfirmed(requestId, null);
      return true;
    } catch (error) {
      if (isClarificationTerminalGone(error)) {
        recordUnconfirmed(requestId, null);
        return unconfirmedAnswers.get(requestId) === text;
      }
      if (isAbortError(error)) throw error;
      if (error instanceof SessionApiError && error.status === 400) {
        throw refusal("the agent's question only accepts a listed choice. Pick one above.", draft);
      }
      // Admission is unknown (transport, 5xx): the live projection says whether it landed.
      // A question still waiting proves the answer missed it; a gone one may have taken it or
      // settled another way, so the draft stays and a resend replays this answer.
      const live = await rereadPending().catch(() => undefined);
      if (live?.request_id === requestId) {
        throw refusal("couldn't send your answer to the agent's question. Try again.", draft);
      }
      recordUnconfirmed(requestId, text);
      throw refusal(
        "couldn't confirm your answer reached the agent's question. Send it again; it never answers twice.",
        draft
      );
    }
  };

  const answering = (
    handler: SessionBusyInputHandler | undefined
  ): SessionBusyInputHandler | undefined => {
    if (!handler || !routable || (!pending && unconfirmedAnswers.size === 0)) return handler;
    return async draft => {
      const resent = [...unconfirmedAnswers].find(
        ([, text]) => text === draft.message.trim() && draft.attachments.length === 0
      );
      if (resent && (await answer(resent[0], resent[1], draft))) return;
      if (!pending) return handler(draft);
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
      // line still blocks the turn, so it takes this answer. The send goes out as steer or queue
      // only once a read proves no question waits.
      let settled = pending.request_id;
      for (let attempt = 0; attempt < NEXT_QUESTION_ATTEMPTS; attempt += 1) {
        const next = await rereadPending().catch(() => {
          throw refusal("couldn't check the agent's question. Try again.", draft);
        });
        if (!next) return handler(draft);
        if (next.request_id !== settled && (await answer(next.request_id, text, draft))) return;
        settled = next.request_id;
      }
      throw refusal("the agent's questions kept changing. Try again.", draft);
    };
  };

  return {
    pending,
    answering,
    placeholder: pending ? "Type here to answer the agent's question" : undefined,
  };
}
