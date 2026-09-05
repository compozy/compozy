import { createStoreLogic } from "@xstate/store";

import type { SessionComposerSubmission } from "./use-session-composer-state";
import {
  sessionBusyInputRefusalFromError,
  type SessionBusyInputAction,
  type SessionBusyInputDraft,
  type SessionBusyInputHandler,
  type SessionBusyInputRefusal,
  type SessionSendOutcome,
} from "@/systems/session";

/**
 * What the composer says about the last busy send, inside the card
 * (US-004.AC-1/AC-3). `draftText` is the raw field text the note belongs to
 * (what the field read when the send was refused, or what it was left with
 * after acceptance): the note is shown while the field still reads exactly
 * that text and hides the moment the operator types again.
 */
export type SessionComposerFeedback =
  | {
      kind: "disposition";
      action: SessionBusyInputAction;
      outcome: SessionSendOutcome;
      draftText: string;
    }
  | {
      kind: "refusal";
      action: SessionBusyInputAction;
      refusal: SessionBusyInputRefusal;
      draftText: string;
    };

interface SessionBusyInputState {
  editingQueuedPromptId: string | null;
  feedback: SessionComposerFeedback | null;
  phase: "idle" | "submitting";
}

type SessionBusyInputEvents = {
  editCompleted: Record<never, never>;
  editStarted: { id: string };
  feedbackDismissed: Record<never, never>;
  promptRemoved: { id: string };
  submissionRefused: {
    action: SessionBusyInputAction;
    /** Raw field text when the send was refused — it stays exactly as typed. */
    composerText: string;
    refusal: SessionBusyInputRefusal;
  };
  submissionFinished: Record<never, never>;
  submissionRequested: {
    action: SessionBusyInputAction;
    canSubmit: boolean;
    consumeSubmittedDraft: (submission: SessionComposerSubmission) => string;
    handler: SessionBusyInputHandler | undefined;
    draft: SessionBusyInputDraft;
    submission: SessionComposerSubmission;
    onFailure?: (error: unknown) => void;
    onSuccess?: () => void;
  };
  submissionSucceeded: {
    action: SessionBusyInputAction;
    outcome: SessionSendOutcome | null;
    /** Text left in the field after the accepted send was consumed. */
    remainingText: string;
  };
};

export const sessionBusyInputLogic = createStoreLogic<
  SessionBusyInputState,
  SessionBusyInputEvents
>({
  context: { editingQueuedPromptId: null, feedback: null, phase: "idle" },
  on: {
    editStarted: (context, event) => ({ ...context, editingQueuedPromptId: event.id }),
    editCompleted: context => ({ ...context, editingQueuedPromptId: null }),
    feedbackDismissed: context =>
      context.feedback === null ? undefined : { ...context, feedback: null },
    promptRemoved: (context, event) =>
      context.editingQueuedPromptId === event.id
        ? { ...context, editingQueuedPromptId: null }
        : undefined,
    submissionRefused: (context, event) => ({
      ...context,
      feedback: {
        action: event.action,
        draftText: event.composerText,
        kind: "refusal",
        refusal: event.refusal,
      },
    }),
    submissionRequested: (context, event, enqueue) => {
      const { handler } = event;
      const hasDraft = event.draft.message.trim().length > 0 || event.draft.attachments.length > 0;
      if (context.phase === "submitting" && hasDraft) {
        // A second send while one is in flight is a gate, and gates say why (US-004.AC-3).
        return {
          ...context,
          feedback: {
            action: event.action,
            draftText: event.submission.composerText,
            kind: "refusal",
            refusal: {
              attachmentCount: event.draft.attachments.length,
              code: "send_in_flight",
              currentTurnId: null,
              message: null,
            },
          },
        };
      }
      if (!event.canSubmit || !handler || context.phase === "submitting") return;
      enqueue.effect(async ({ trigger }) => {
        try {
          const outcome = await handler(event.draft);
          // Acceptance consumes exactly what was sent — never text or files the
          // operator added while the send was in flight; a refusal leaves the
          // field exactly where it was (US-004.EC-3).
          const remainingText = event.consumeSubmittedDraft(event.submission);
          event.onSuccess?.();
          trigger.submissionSucceeded({
            action: event.action,
            outcome: outcome ?? null,
            remainingText,
          });
        } catch (error) {
          const refusal = sessionBusyInputRefusalFromError(error, {
            attachmentCount: event.draft.attachments.length,
          });
          if (refusal) {
            trigger.submissionRefused({
              action: event.action,
              composerText: event.submission.composerText,
              refusal,
            });
          }
          event.onFailure?.(error);
        } finally {
          trigger.submissionFinished();
        }
      });
      return { ...context, feedback: null, phase: "submitting" };
    },
    submissionSucceeded: (context, event) => ({
      ...context,
      feedback: event.outcome
        ? {
            action: event.action,
            draftText: event.remainingText,
            kind: "disposition",
            outcome: event.outcome,
          }
        : null,
    }),
    submissionFinished: context =>
      context.phase === "idle" ? undefined : { ...context, phase: "idle" },
  },
});
