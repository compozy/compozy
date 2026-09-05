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
  const busy = deriveComposerBusyState({
    allowBusyInput,
    attachmentBlocked: attachmentBlocker !== null,
    attachmentCount: promptAttachments.length,
    busyInputDefaultMode,
    canPrompt,
    draftTextLength: trimmedComposerText.length,
    hasQueueHandler: Boolean(onQueuePrompt),
    hasQueuedStrip:
      queuedPrompts.length > 0 && Boolean(onRemoveQueuedPrompt && onSteerQueuedPrompt),
    hasStagedQuote: Boolean(stagedQuote),
    hasSteerHandler: Boolean(onSteerPrompt),
    isBusyInputPending,
    isRunning,
    isSessionRunning,
    stopPhase,
  });
  const {
    busyEnterActive,
    canSubmitBusyInput,
    effectiveBusyInputMode,
    isStopping,
    runtimeRunning,
    showBusyControls,
    showQueuedStrip,
  } = busy;
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
  const visibleFeedback = visibleComposerFeedback(feedback, composerText);

  useEffect(() => {
    if (!visibleFeedback) return;
    const timer = window.setTimeout(dismissFeedback, SESSION_COMPOSER_FEEDBACK_TTL_MS);
    return () => window.clearTimeout(timer);
  }, [dismissFeedback, visibleFeedback]);

  const enterHint = composerEnterHint(busyEnterActive, isStopping, busyInputDefaultMode);

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

interface ComposerBusyInput {
  allowBusyInput: boolean;
  attachmentBlocked: boolean;
  attachmentCount: number;
  busyInputDefaultMode: SessionBusyInputMode;
  canPrompt: boolean;
  draftTextLength: number;
  hasQueueHandler: boolean;
  hasQueuedStrip: boolean;
  hasStagedQuote: boolean;
  hasSteerHandler: boolean;
  isBusyInputPending: boolean;
  isRunning: boolean;
  isSessionRunning: boolean;
  stopPhase: "idle" | "stopping";
}

/**
 * The busy-turn read model the composer renders from. While a stop is landing
 * the turn is ending: guidance can't be honored, but queued work survives a
 * stop (ADR-003), so Enter queues and only Queue stays.
 */
function deriveComposerBusyState(input: ComposerBusyInput) {
  const runtimeRunning = input.isRunning || input.isSessionRunning;
  const isStopping = input.stopPhase === "stopping";
  const busyControlsActive = runtimeRunning || isStopping;
  const effectiveBusyInputMode: SessionBusyInputMode = isStopping
    ? "queue"
    : input.busyInputDefaultMode;
  const hasDraft = input.draftTextLength > 0 || input.attachmentCount > 0 || input.hasStagedQuote;
  const canSubmitBusyInput =
    busyControlsActive &&
    input.canPrompt &&
    input.allowBusyInput &&
    hasDraft &&
    !input.attachmentBlocked &&
    !input.isBusyInputPending;
  const busyEnterActive =
    busyControlsActive &&
    input.allowBusyInput &&
    (input.hasQueueHandler || (input.hasSteerHandler && !isStopping));
  return {
    busyEnterActive,
    canSubmitBusyInput,
    effectiveBusyInputMode,
    isStopping,
    runtimeRunning,
    showBusyControls: busyControlsActive || input.isBusyInputPending,
    showQueuedStrip: input.hasQueuedStrip,
  };
}

function composerEnterHint(
  busyEnterActive: boolean,
  isStopping: boolean,
  busyInputDefaultMode: SessionBusyInputMode
): SessionComposerEnterHint {
  if (!busyEnterActive) return { enter: "send", modifier: null };
  if (isStopping) return { enter: "queue", modifier: null };
  return {
    enter: busyInputDefaultMode,
    modifier: oppositeSessionBusyInputMode(busyInputDefaultMode),
  };
}

/** The note belongs to the raw field text it answered: typing again hides it (derived, no sync). */
function visibleComposerFeedback<T extends { draftText: string }>(
  feedback: T | null,
  composerText: string
): T | null {
  return feedback && feedback.draftText === composerText ? feedback : null;
}
