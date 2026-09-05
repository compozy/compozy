import { useAuiState } from "@assistant-ui/react";
import { useEffect, useState } from "react";

import {
  attachmentsFromPromptMessageParts,
  composeSessionPromptWithTerminalQuote,
  DEFAULT_SESSION_BUSY_INPUT_MODE,
  discardSessionTerminalQuote,
  oppositeSessionBusyInputMode,
  useSessionTerminalQuote,
  type QueuedPrompt,
  type SessionBusyInputMode,
} from "@/systems/session";

import type { SessionComposerProps } from "../session-composer";
import { createSessionCommandFormatter } from "../session-command-formatter";
import type { CommandCatalogScope } from "../session-composer-command-menu";
import { useSessionBusyInputActions } from "./use-session-busy-input-actions";
import { usePendingCommandAction } from "./use-pending-command-action";
import type { SessionComposerState } from "./use-session-composer-state";
import { sessionComposerSendBlocker } from "./use-session-composer-send-gate";

const EMPTY_QUEUED_PROMPTS: QueuedPrompt[] = [];

/** A disposition or refusal note stays until the operator types again or this elapses. */
export const SESSION_COMPOSER_FEEDBACK_TTL_MS = 6_000;

/** What Enter does right now, plus the one-shot opposite the modifier applies. */
export interface SessionComposerEnterHint {
  enter: "send" | SessionBusyInputMode;
  modifier: SessionBusyInputMode | null;
}

export function useSessionComposerController({
  composerState,
  contentInset,
  canPrompt,
  onCancelPrompt,
  onQueuePrompt,
  onInterruptPrompt,
  onSteerPrompt,
  isBusyInputPending = false,
  isSessionRunning = false,
  stopPhase = "idle",
  allowBusyInput = true,
  busyInputDefaultMode = DEFAULT_SESSION_BUSY_INPUT_MODE,
  busyInputSteerDelivery = null,
  queuedPrompts = EMPTY_QUEUED_PROMPTS,
  onRemoveQueuedPrompt,
  onReplaceQueuedPrompt,
  onSteerQueuedPrompt,
  inactivePlaceholder = "Session is not active",
  decisionDock,
  runtimeControl,
  environmentControl,
  commandCatalog,
  commandCatalogStatus,
  onCommandCatalogOpen,
  onCommandAction,
  promptImageCapability = "unknown",
  promptEmbeddedContextCapability = "unknown",
  sessionId,
  quoteSlot,
}: SessionComposerProps & { composerState: SessionComposerState }) {
  const {
    consumeSubmittedDraft,
    setComposerInputElement,
    setComposerText,
    composerText,
    isRunning,
  } = composerState;
  const pendingCommandActionRef = usePendingCommandAction(composerText);
  const [commandScope, setCommandScope] = useState<CommandCatalogScope>("inline");
  const commandFormatter = createSessionCommandFormatter(
    commandCatalog ?? { standaloneSections: [], inlineSkills: [] }
  );
  const stagedQuote = useSessionTerminalQuote(sessionId);
  const trimmedComposerText = composerText.trim();
  const promptText = composeSessionPromptWithTerminalQuote(sessionId, trimmedComposerText);
  const composerAttachments = useAuiState(state => state.composer.attachments);
  const promptAttachments = composerAttachments.flatMap(attachment =>
    attachmentsFromPromptMessageParts(attachment.content)
  );
  const attachmentBlocker = sessionComposerSendBlocker({
    attachments: composerAttachments,
    promptEmbeddedContextCapability,
    promptImageCapability,
  });
  const runtimeRunning = isRunning || isSessionRunning;
  const isStopping = stopPhase === "stopping";
  // While a stop is landing the turn is ending: guidance can't be honored, but
  // queued work survives a stop (ADR-003), so Enter queues and only Queue stays.
  const busyControlsActive = runtimeRunning || isStopping;
  const effectiveBusyInputMode: SessionBusyInputMode = isStopping ? "queue" : busyInputDefaultMode;
  const canSubmitBusyInput =
    busyControlsActive &&
    canPrompt &&
    allowBusyInput &&
    (trimmedComposerText.length > 0 || promptAttachments.length > 0 || Boolean(stagedQuote)) &&
    attachmentBlocker === null &&
    !isBusyInputPending;
  const showBusyControls = busyControlsActive || isBusyInputPending;
  const busyEnterActive =
    busyControlsActive &&
    allowBusyInput &&
    Boolean(onQueuePrompt || (onSteerPrompt && !isStopping));
  const showQueuedStrip =
    queuedPrompts.length > 0 && Boolean(onRemoveQueuedPrompt && onSteerQueuedPrompt);
  const busyActions = useSessionBusyInputActions({
    busyInputDefaultMode: effectiveBusyInputMode,
    canSubmitBusyInput,
    consumeSubmittedDraft,
    submission: {
      attachmentIds: composerAttachments.map(attachment => attachment.id),
      composerText,
    },
    onInterruptPrompt: isStopping ? undefined : onInterruptPrompt,
    onQueuePrompt,
    onRemoveQueuedPrompt,
    onReplaceQueuedPrompt,
    onSteerPrompt: isStopping ? undefined : onSteerPrompt,
    queuedPrompts,
    sessionId,
    setComposerText,
    draft: { attachments: promptAttachments, message: promptText },
    onDraftConsumed: () => discardSessionTerminalQuote(sessionId),
  });
  const { dismissFeedback, feedback } = busyActions;
  // The note belongs to the raw field text it answered: typing again hides it (derived, no sync).
  const visibleFeedback = feedback && feedback.draftText === composerText ? feedback : null;

  useEffect(() => {
    if (!visibleFeedback) return;
    const timer = window.setTimeout(dismissFeedback, SESSION_COMPOSER_FEEDBACK_TTL_MS);
    return () => window.clearTimeout(timer);
  }, [dismissFeedback, visibleFeedback]);

  const enterHint: SessionComposerEnterHint = !busyEnterActive
    ? { enter: "send", modifier: null }
    : isStopping
      ? { enter: "queue", modifier: null }
      : {
          enter: busyInputDefaultMode,
          modifier: oppositeSessionBusyInputMode(busyInputDefaultMode),
        };

  return {
    state: {
      busyEnterActive,
      canSubmitBusyInput,
      commandFormatter,
      commandScope,
      composerAttachmentCount: composerAttachments.length,
      composerText,
      enterHint,
      feedback: visibleFeedback,
      hasStagedQuote: Boolean(stagedQuote),
      runtimeRunning,
      showBusyControls,
      showQueuedStrip,
      stopping: isStopping,
    },
    actions: {
      ...busyActions,
      // The modifier has no opposite to perform while stopping: both variants queue.
      handleEnterAction: (variant: "default" | "opposite") =>
        busyActions.handleEnterAction(isStopping ? "default" : variant),
      recordPendingCommandAction: (token: string, beforeSelection: string) => {
        pendingCommandActionRef.current = { beforeSelection, token };
      },
      setCommandScope,
      setComposerInputElement,
    },
    meta: {
      allowBusyInput,
      busyInputSteerDelivery,
      canPrompt,
      commandCatalog,
      commandCatalogStatus,
      contentInset,
      decisionDock,
      environmentControl,
      inactivePlaceholder,
      isBusyInputPending,
      onCancelPrompt,
      onCommandAction,
      onCommandCatalogOpen,
      onInterruptPrompt,
      onQueuePrompt,
      onReplaceQueuedPrompt,
      onSteerPrompt,
      onSteerQueuedPrompt,
      promptEmbeddedContextCapability,
      promptImageCapability,
      queuedPrompts,
      quoteSlot,
      runtimeControl,
      sessionId,
    },
  };
}

export type SessionComposerController = ReturnType<typeof useSessionComposerController>;
