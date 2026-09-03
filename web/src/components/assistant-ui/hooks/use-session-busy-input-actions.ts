import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { sessionBusyInputLogic } from "./session-busy-input-store";
import {
  splitQuotedPrompt,
  stageChosenSessionTerminalQuote,
  type QueuedPrompt,
  type SessionBusyInputDraft,
  type SessionBusyInputHandler,
} from "@/systems/session";

export type { SessionBusyInputHandler } from "@/systems/session";

interface UseSessionBusyInputActionsOptions {
  canSubmitBusyInput: boolean;
  clearComposer: (options?: { retainAttachments?: boolean }) => void;
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
  canSubmitBusyInput,
  clearComposer,
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

  const handleBusyInputAction = (
    handler: SessionBusyInputHandler | undefined,
    failureMessage: string,
    onSuccess?: () => void
  ) => {
    store.trigger.submissionRequested({
      canSubmit: canSubmitBusyInput,
      clearComposer,
      handler,
      draft: {
        attachments: draft.attachments.map(attachment => ({ ...attachment })),
        message: draft.message,
      },
      onFailure: error => {
        if (!isAbortError(error)) {
          toast.error(describeComposerActionError(error, failureMessage));
        }
      },
      onSuccess: () => {
        onDraftConsumed?.();
        onSuccess?.();
      },
    });
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
        nextDraft => onReplaceQueuedPrompt(editingQueuedPrompt, nextDraft.message),
        "Couldn't update queued prompt.",
        () => {
          store.trigger.editCompleted();
        }
      );
      return;
    }
    handleBusyInputAction(onQueuePrompt, "Couldn't queue prompt.");
  };

  const handleSteerAction = () => {
    if (draft.attachments.length > 0) {
      toast.error("Remove attachments before steering the active turn.");
      return;
    }
    handleBusyInputAction(onSteerPrompt, "Couldn't steer prompt.");
  };

  const handleInterruptAction = () => {
    handleBusyInputAction(onInterruptPrompt, "Couldn't interrupt prompt.");
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

  return {
    handleBusyInputAction,
    handleEditQueuedPrompt,
    handleInterruptAction,
    handleQueueAction,
    handleRemoveQueuedPrompt,
    handleSteerAction,
  };
}
