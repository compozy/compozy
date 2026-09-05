import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { sessionBusyInputLogic } from "./session-busy-input-store";
import type { SessionComposerSubmission } from "./use-session-composer-state";
import {
  oppositeSessionBusyInputMode,
  splitQuotedPrompt,
  stageChosenSessionTerminalQuote,
  type QueuedPrompt,
  type SessionBusyInputAction,
  type SessionBusyInputDraft,
  type SessionBusyInputHandler,
  type SessionBusyInputMode,
} from "@/systems/session";

export type { SessionBusyInputHandler } from "@/systems/session";

interface UseSessionBusyInputActionsOptions {
  busyInputDefaultMode: SessionBusyInputMode;
  canSubmitBusyInput: boolean;
  consumeSubmittedDraft: (submission: SessionComposerSubmission) => string;
  /** The raw composer text and attachment ids a send would take right now. */
  submission: SessionComposerSubmission;
  onInterruptPrompt?: SessionBusyInputHandler;
  onQueuePrompt?: SessionBusyInputHandler;
  onRemoveQueuedPrompt?: (id: string) => void;
  onReplaceQueuedPrompt?: (prompt: QueuedPrompt, message: string) => Promise<unknown>;
  onSteerPrompt?: SessionBusyInputHandler;
  queuedPrompts: QueuedPrompt[];
  sessionId: string;
  setComposerText: (text: string) => void;
  draft: SessionBusyInputDraft;
  /** Fires after a queued, steered, or interrupt draft is accepted. */
  onDraftConsumed?: () => void;
}

function describeComposerActionError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && "name" in error && error.name === "AbortError"
  );
}

export function useSessionBusyInputActions({
  busyInputDefaultMode,
  canSubmitBusyInput,
  consumeSubmittedDraft,
  submission,
  onInterruptPrompt,
  onQueuePrompt,
  onRemoveQueuedPrompt,
  onReplaceQueuedPrompt,
  onSteerPrompt,
  queuedPrompts,
  sessionId,
  setComposerText,
  draft,
  onDraftConsumed,
}: UseSessionBusyInputActionsOptions) {
  const store = useStore(sessionBusyInputLogic);
  const editingQueuedPromptId = useSelector(
    store,
    snapshot => snapshot.context.editingQueuedPromptId
  );
  const feedback = useSelector(store, snapshot => snapshot.context.feedback);

  const handlerFor = (action: SessionBusyInputAction): SessionBusyInputHandler | undefined => {
    switch (action) {
      case "queue":
        return onQueuePrompt;
      case "steer":
        return onSteerPrompt;
      case "interrupt":
        return onInterruptPrompt;
    }
  };

  const handleBusyInputAction = (
    action: SessionBusyInputAction,
    handler: SessionBusyInputHandler | undefined,
    options: { onFailure?: (error: unknown) => void; onSuccess?: () => void } = {}
  ) => {
    store.trigger.submissionRequested({
      action,
      canSubmit: canSubmitBusyInput,
      consumeSubmittedDraft,
      handler,
      draft: {
        attachments: draft.attachments.map(attachment => ({ ...attachment })),
        message: draft.message,
      },
      submission: {
        attachmentIds: [...submission.attachmentIds],
        composerText: submission.composerText,
      },
      onFailure: options.onFailure,
      onSuccess: () => {
        onDraftConsumed?.();
        options.onSuccess?.();
      },
    });
  };

  const submitVerb = (action: SessionBusyInputAction) => {
    handleBusyInputAction(action, handlerFor(action));
  };

  const handleQueueAction = () => {
    const editingQueuedPrompt =
      queuedPrompts.find(prompt => prompt.id === editingQueuedPromptId) ?? null;
    if (editingQueuedPrompt) {
      if (!onReplaceQueuedPrompt) {
        toast.error("Couldn't update queued prompt.");
        return;
      }
      handleBusyInputAction(
        "queue",
        async nextDraft => {
          await onReplaceQueuedPrompt(editingQueuedPrompt, nextDraft.message);
        },
        {
          onFailure: error => {
            if (!isAbortError(error)) {
              toast.error(describeComposerActionError(error, "Couldn't update queued prompt."));
            }
          },
          onSuccess: () => {
            store.trigger.editCompleted();
          },
        }
      );
      return;
    }
    submitVerb("queue");
  };

  const handleSteerAction = () => submitVerb("steer");
  const handleInterruptAction = () => submitVerb("interrupt");

  /**
   * Enter follows the daemon default; the modifier performs the opposite for
   * exactly one send (US-003.AC-3). An in-progress queued-prompt edit keeps
   * Enter on the edit path.
   */
  const handleEnterAction = (variant: "default" | "opposite") => {
    const mode =
      variant === "default"
        ? busyInputDefaultMode
        : oppositeSessionBusyInputMode(busyInputDefaultMode);
    if (mode === "queue" || editingQueuedPromptId !== null) {
      handleQueueAction();
      return;
    }
    handleSteerAction();
  };

  const handleEditQueuedPrompt = (prompt: QueuedPrompt) => {
    if (
      draft.attachments.length > 0 ||
      (draft.message.length > 0 && draft.message !== prompt.text.trim())
    ) {
      toast.warning("Send or clear the current draft before editing a queued prompt.");
      return;
    }
    const { annotation, quote } = splitQuotedPrompt(prompt.text);
    if (quote) {
      stageChosenSessionTerminalQuote(sessionId, quote);
    }
    setComposerText(annotation);
    store.trigger.editStarted({ id: prompt.id });
  };

  const handleRemoveQueuedPrompt = (id: string) => {
    store.trigger.promptRemoved({ id });
    onRemoveQueuedPrompt?.(id);
  };

  const dismissFeedback = () => {
    store.trigger.feedbackDismissed();
  };

  return {
    dismissFeedback,
    feedback,
    handleBusyInputAction,
    handleEditQueuedPrompt,
    handleEnterAction,
    handleInterruptAction,
    handleQueueAction,
    handleRemoveQueuedPrompt,
    handleSteerAction,
  };
}
