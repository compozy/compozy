import { useRef, type ChangeEvent, type KeyboardEvent } from "react";
import { toast } from "sonner";

import type { QueuedPrompt } from "@/systems/session";

export type SessionBusyInputHandler = (message: string) => void | Promise<unknown>;

interface UseSessionBusyInputActionsOptions {
  canQueueFromInput: boolean;
  canSubmitBusyInput: boolean;
  clearComposer: () => void;
  persistComposerText: (text: string) => void;
  onInterruptPrompt?: SessionBusyInputHandler;
  onQueuePrompt?: SessionBusyInputHandler;
  onRemoveQueuedPrompt?: (id: string) => void;
  onReplaceQueuedPrompt?: (prompt: QueuedPrompt, message: string) => Promise<unknown>;
  onSteerPrompt?: SessionBusyInputHandler;
  queuedPrompts: QueuedPrompt[];
  runtimeRunning: boolean;
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
  canQueueFromInput,
  canSubmitBusyInput,
  clearComposer,
  persistComposerText,
  onInterruptPrompt,
  onQueuePrompt,
  onRemoveQueuedPrompt,
  onReplaceQueuedPrompt,
  onSteerPrompt,
  queuedPrompts,
  runtimeRunning,
  setComposerText,
  trimmedComposerText,
}: UseSessionBusyInputActionsOptions) {
  const editingQueuedPromptId = useRef<string | null>(null);

  const handleBusyInputAction = (
    handler: SessionBusyInputHandler | undefined,
    failureMessage: string,
    onSuccess?: () => void
  ) => {
    if (!handler || !canSubmitBusyInput) {
      return;
    }

    void Promise.resolve()
      .then(() => handler(trimmedComposerText))
      .then(() => {
        clearComposer();
        onSuccess?.();
      })
      .catch(error => {
        if (isAbortError(error)) return;
        toast.error(describeComposerActionError(error, failureMessage));
      });
  };

  const handleQueueAction = () => {
    const editingQueuedPrompt =
      queuedPrompts.find(prompt => prompt.id === editingQueuedPromptId.current) ?? null;
    if (editingQueuedPrompt) {
      if (!onReplaceQueuedPrompt) {
        toast.error("Couldn't update queued prompt.");
        return;
      }
      handleBusyInputAction(
        message => onReplaceQueuedPrompt(editingQueuedPrompt, message),
        "Couldn't update queued prompt.",
        () => {
          editingQueuedPromptId.current = null;
        }
      );
      return;
    }
    handleBusyInputAction(onQueuePrompt, "Couldn't queue prompt.");
  };

  const handleComposerChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    persistComposerText(event.currentTarget.value);
  };

  const handleSteerAction = () => {
    handleBusyInputAction(onSteerPrompt, "Couldn't steer prompt.");
  };

  const handleInterruptAction = () => {
    handleBusyInputAction(onInterruptPrompt, "Couldn't interrupt prompt.");
  };

  const handleInputKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (
      runtimeRunning &&
      canQueueFromInput &&
      event.key === "Enter" &&
      !event.shiftKey &&
      !event.ctrlKey &&
      !event.metaKey &&
      !event.nativeEvent.isComposing
    ) {
      event.preventDefault();
      handleQueueAction();
    }
  };

  const handleEditQueuedPrompt = (prompt: QueuedPrompt) => {
    if (trimmedComposerText.length > 0 && trimmedComposerText !== prompt.text.trim()) {
      toast.warning("Send or clear the current draft before editing a queued prompt.");
      return;
    }
    setComposerText(prompt.text);
    editingQueuedPromptId.current = prompt.id;
  };

  const handleRemoveQueuedPrompt = (id: string) => {
    if (editingQueuedPromptId.current === id) {
      editingQueuedPromptId.current = null;
    }
    onRemoveQueuedPrompt?.(id);
  };

  return {
    handleBusyInputAction,
    handleComposerChange,
    handleEditQueuedPrompt,
    handleInputKeyDown,
    handleInterruptAction,
    handleQueueAction,
    handleRemoveQueuedPrompt,
    handleSteerAction,
  };
}
