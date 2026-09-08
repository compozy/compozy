import { useSelector, useStore } from "@xstate/store-react";
import { useAui } from "@assistant-ui/react";
import { toast } from "sonner";

import { sessionBusyInputLogic } from "./session-busy-input-store";
import type { SessionComposerSubmission } from "./use-session-composer-state";
import {
  composeQuotedPrompt,
  findUnconfirmedSend,
  oppositeSessionBusyInputMode,
  sessionBusyInputRefusalFromError,
  splitQuotedPrompt,
  type QueuedPrompt,
  type QueuedPromptEditOutcome,
  type SessionBusyInputAction,
  type SessionBusyInputDraft,
  type SessionBusyInputHandler,
  type SessionBusyInputMode,
  type SessionSendOutcome,
  type UnconfirmedSend,
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
  onRetryUnconfirmedSend?: (id: string) => Promise<SessionSendOutcome | void>;
  onSteerPrompt?: SessionBusyInputHandler;
  setComposerText: (text: string) => void;
  /** The live stream is down: a send fails at once and keeps the draft (US-018.AC-3). */
  transportDisconnected: boolean;
  unconfirmedSends: UnconfirmedSend[];
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
  onRetryUnconfirmedSend,
  onSteerPrompt,
  setComposerText,
  transportDisconnected,
  unconfirmedSends,
  draft,
  onDraftConsumed,
}: UseSessionBusyInputActionsOptions) {
  const aui = useAui();
  const store = useStore(sessionBusyInputLogic);
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

  /**
   * Sending while disconnected fails now and keeps the draft (US-018.AC-3):
   * one note, nothing consumed, no request leaves. A second press repeats the
   * note — never a silent no-op.
   */
  const refuseDisconnectedSend = (action: SessionBusyInputAction) => {
    store.trigger.submissionRefused({
      action,
      composerText: submission.composerText,
      refusal: {
        attachmentCount: draft.attachments.length,
        code: "disconnected",
        currentTurnId: null,
        message: null,
        queueCap: null,
      },
    });
  };

  const handleBusyInputAction = (
    action: SessionBusyInputAction,
    handler: SessionBusyInputHandler | undefined,
    options: { onFailure?: (error: unknown) => void; onSuccess?: () => void } = {}
  ) => {
    if (transportDisconnected) {
      refuseDisconnectedSend(action);
      return;
    }
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

  const handleQueueAction = () => submitVerb("queue");

  /**
   * Saves an in-row edit as one atomic replacement (US-005.AC-3). The editor
   * held the annotation only; the terminal-context envelope, when any, rides
   * back unchanged. When the entry had already started dispatching the daemon
   * refuses with `entry_dispatching`: its word lands in the composer note and
   * the edited text becomes a fresh draft there, after anything the operator
   * was already typing — nothing typed is lost (US-005.EC-2).
   */
  const handleSaveQueuedPromptEdit = async (
    prompt: QueuedPrompt,
    annotation: string
  ): Promise<QueuedPromptEditOutcome> => {
    if (!onReplaceQueuedPrompt) {
      toast.error("Couldn't update queued prompt.");
      return "failed";
    }
    const { quote } = splitQuotedPrompt(prompt.text);
    const text = annotation.trim();
    try {
      await onReplaceQueuedPrompt(prompt, composeQuotedPrompt(text, quote));
      return "replaced";
    } catch (error) {
      const refusal = sessionBusyInputRefusalFromError(error);
      if (refusal?.code === "entry_dispatching") {
        const current = aui.composer.getState().text;
        const handedOff = current.trim().length > 0 ? `${current}\n\n${text}` : text;
        setComposerText(handedOff);
        store.trigger.submissionRefused({ action: "queue", composerText: handedOff, refusal });
        return "handed_off";
      }
      if (!isAbortError(error)) {
        toast.error(describeComposerActionError(error, "Couldn't update queued prompt."));
      }
      return "failed";
    }
  };

  /**
   * Retry replays a retained identity through the same submission gate, so the
   * daemon's answer lands in the composer note like any other busy send —
   * `replayed` when it had arrived, a fresh disposition when it had not. The
   * field is untouched: nothing was taken from it and nothing is consumed.
   */
  const handleRetryUnconfirmedSend = (id: string) => {
    const send = findUnconfirmedSend(unconfirmedSends, id);
    if (!send || !onRetryUnconfirmedSend) return;
    store.trigger.submissionRequested({
      action: send.action,
      canSubmit: true,
      consumeSubmittedDraft: submitted => submitted.composerText,
      handler: () => onRetryUnconfirmedSend(id),
      draft: {
        attachments: send.attachments.map(attachment => ({ ...attachment })),
        message: send.text,
      },
      submission: { attachmentIds: [], composerText: submission.composerText },
    });
  };

  const handleSteerAction = () => submitVerb("steer");
  const handleInterruptAction = () => submitVerb("interrupt");

  /**
   * Enter follows the daemon default; the modifier performs the opposite for
   * exactly one send (US-003.AC-3).
   */
  const handleEnterAction = (variant: "default" | "opposite") => {
    const mode =
      variant === "default"
        ? busyInputDefaultMode
        : oppositeSessionBusyInputMode(busyInputDefaultMode);
    if (mode === "queue") {
      handleQueueAction();
      return;
    }
    handleSteerAction();
  };

  const handleRemoveQueuedPrompt = (id: string) => {
    onRemoveQueuedPrompt?.(id);
  };

  const dismissFeedback = () => {
    store.trigger.feedbackDismissed();
  };

  return {
    dismissFeedback,
    feedback,
    handleBusyInputAction,
    /** The idle Send control's answer while disconnected: the same note, the draft untouched. */
    handleDisconnectedSend: () => refuseDisconnectedSend("steer"),
    handleEnterAction,
    handleInterruptAction,
    handleQueueAction,
    handleRemoveQueuedPrompt,
    handleRetryUnconfirmedSend,
    handleSaveQueuedPromptEdit,
    handleSteerAction,
  };
}
