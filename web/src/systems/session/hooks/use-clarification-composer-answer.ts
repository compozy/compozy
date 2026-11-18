import { SessionBusyInputRefusalError } from "../lib/session-busy-input-refusal";
import type { ClarificationPending, SessionBusyInputHandler } from "../types";
import { isClarificationTerminalGone } from "./use-clarification-dock";
import {
  useAnswerSessionClarification,
  useSessionClarifications,
} from "./use-session-clarifications";

export interface UseClarificationComposerAnswerOptions {
  enabled?: boolean;
  sessionId: string;
  workspaceId: string;
}

/**
 * Routes a main-composer send to the session's pending clarification (#1821).
 *
 * While the agent waits on a question, its turn cannot reach the next step that
 * would take a steer or end so a queued prompt can run. So a steer or queue
 * send becomes the question's free-text answer instead, through the same
 * answer endpoint as the dock, and the turn resumes at once. When the question
 * settled first (answered elsewhere, timed out, canceled), the send goes out as
 * it would have. Interrupt keeps its own meaning: it cancels the turn, which
 * cancels the question.
 */
export function useClarificationComposerAnswer({
  enabled = true,
  sessionId,
  workspaceId,
}: UseClarificationComposerAnswerOptions) {
  const clarifications = useSessionClarifications(workspaceId, sessionId, { enabled });
  const answerMutation = useAnswerSessionClarification(workspaceId, sessionId);
  const pending: ClarificationPending | null = clarifications.error
    ? null
    : (clarifications.data?.[0] ?? null);

  const answering = (
    handler: SessionBusyInputHandler | undefined
  ): SessionBusyInputHandler | undefined => {
    if (!handler || !pending) return handler;
    return async draft => {
      const text = draft.message.trim();
      if (text.length === 0) return handler(draft);
      if (draft.attachments.length > 0) {
        throw new SessionBusyInputRefusalError({
          attachmentCount: draft.attachments.length,
          code: "not_delivered",
          message: "an answer to the agent's question can't carry files. Remove them to answer.",
        });
      }
      try {
        await answerMutation.mutateAsync({ requestId: pending.request_id, body: { text } });
      } catch (error) {
        if (isClarificationTerminalGone(error)) return handler(draft);
        throw new SessionBusyInputRefusalError({
          code: "not_delivered",
          message: "couldn't send your answer to the agent's question. Try again.",
        });
      }
    };
  };

  return { pending, answering };
}
