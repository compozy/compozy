import { useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { sessionBusyInputLogic } from "./session-busy-input-store";
import type { QueuedPrompt } from "@/systems/session";

export type SessionBusyInputHandler = (message: string) => void | Promise<unknown>;

interface UseSessionBusyInputActionsOptions {
  canSubmitBusyInput: boolean;
  clearComposer: () => void;
  onInterruptPrompt?: SessionBusyInputHandler;
  onQueuePrompt?: SessionBusyInputHandler;
  onRemoveQueuedPrompt?: (id: string) => void;
  onReplaceQueuedPrompt?: (prompt: QueuedPrompt, message: string) => Promise<unknown>;
  onSteerPrompt?: SessionBusyInputHandler;
  queuedPrompts: QueuedPrompt[];
  setComposerText: (text: string) => void;
  trimmedComposerText: string;
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
  setComposerText,
  trimmedComposerText,
}: UseSessionBusyInputActionsOptions) {
  const store = useStore(sessionBusyInputLogic);

  const handleBusyInputAction = (
    handler: SessionBusyInputHandler | undefined,
    failureMessage: string,
    onSuccess?: () => void
  ) => {
    if (!handler || !canSubmitBusyInput || store.getSnapshot().context.phase === "submitting") {
      return;
    }
    store.trigger.submissionStarted();

    void Promise.resolve()
      .then(() => handler(trimmedComposerText))
      .then(() => {
        clearComposer();
        onSuccess?.();
        store.trigger.submissionFinished();
      })
      .catch(error => {
        if (!isAbortError(error)) {
          toast.error(describeComposerActionError(error, failureMessage));
        }
        store.trigger.submissionFinished();
      });
  };

  const handleQueueAction = () => {
    const editingQueuedPrompt =
      queuedPrompts.find(
        prompt => prompt.id === store.getSnapshot().context.editingQueuedPromptId
      ) ?? null;
    if (editingQueuedPrompt) {
      if (!onReplaceQueuedPrompt) {
        toast.error("Couldn't update queued prompt.");
        return;
      }
      handleBusyInputAction(
        message => onReplaceQueuedPrompt(editingQueuedPrompt, message),
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
    handleBusyInputAction(onSteerPrompt, "Couldn't steer prompt.");
  };

  const handleInterruptAction = () => {
    handleBusyInputAction(onInterruptPrompt, "Couldn't interrupt prompt.");
  };

  const handleEditQueuedPrompt = (prompt: QueuedPrompt) => {
    if (trimmedComposerText.length > 0 && trimmedComposerText !== prompt.text.trim()) {
      toast.warning("Send or clear the current draft before editing a queued prompt.");
      return;
    }
    setComposerText(prompt.text);
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
