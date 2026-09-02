import { useAuiState } from "@assistant-ui/react";
import { useState } from "react";

import {
  attachmentsFromPromptMessageParts,
  composeSessionPromptWithTerminalQuote,
  discardSessionTerminalQuote,
  useSessionTerminalQuote,
  type QueuedPrompt,
} from "@/systems/session";

import type { SessionComposerProps } from "../session-composer";
import { createSessionCommandFormatter } from "../session-command-formatter";
import type { CommandCatalogScope } from "../session-composer-command-menu";
import { useSessionBusyInputActions } from "./use-session-busy-input-actions";
import { usePendingCommandAction } from "./use-pending-command-action";
import type { SessionComposerState } from "./use-session-composer-state";
import { sessionComposerSendBlocker } from "./use-session-composer-send-gate";

const EMPTY_QUEUED_PROMPTS: QueuedPrompt[] = [];

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
  allowBusyInput = true,
  busyInputFenceAvailable = true,
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
  const { clearComposer, setComposerInputElement, setComposerText, composerText, isRunning } =
    composerState;
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
  const canSubmitBusyInput =
    runtimeRunning &&
    canPrompt &&
    allowBusyInput &&
    (trimmedComposerText.length > 0 || promptAttachments.length > 0 || Boolean(stagedQuote)) &&
    attachmentBlocker === null &&
    !isBusyInputPending;
  const showBusyControls = runtimeRunning || isBusyInputPending;
  const canQueueFromInput = allowBusyInput && Boolean(onQueuePrompt);
  const showQueuedStrip =
    queuedPrompts.length > 0 && Boolean(onRemoveQueuedPrompt && onSteerQueuedPrompt);
  const busyActions = useSessionBusyInputActions({
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
    draft: { attachments: promptAttachments, message: promptText },
    onDraftConsumed: () => discardSessionTerminalQuote(sessionId),
  });

  return {
    state: {
      canQueueFromInput,
      canSubmitBusyInput,
      commandFormatter,
      commandScope,
      composerAttachmentCount: composerAttachments.length,
      composerText,
      hasStagedQuote: Boolean(stagedQuote),
      runtimeRunning,
      showBusyControls,
      showQueuedStrip,
    },
    actions: {
      ...busyActions,
      recordPendingCommandAction: (token: string, beforeSelection: string) => {
        pendingCommandActionRef.current = { beforeSelection, token };
      },
      setCommandScope,
      setComposerInputElement,
    },
    meta: {
      allowBusyInput,
      busyInputFenceAvailable,
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
