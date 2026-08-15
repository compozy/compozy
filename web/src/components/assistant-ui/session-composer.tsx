import { ComposerPrimitive, useAui, useAuiState } from "@assistant-ui/react";
import { LexicalComposerInput } from "@assistant-ui/react-lexical";
import { useState, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import { attachmentsFromPromptMessageParts, type QueuedPrompt } from "@/systems/session";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import { createSessionCommandFormatter } from "./session-command-formatter";
import { commandItemPresentation } from "./session-command-menu-model";
import { SessionCommandChip } from "./session-composer-chip";
import {
  SessionComposerCommandMenu,
  type CommandCatalogScope,
  type SessionComposerCommandCatalog,
} from "./session-composer-command-menu";
import {
  SessionBusyEnterPlugin,
  SessionCommandScopePlugin,
  SessionComposerHandleBridge,
  SessionComposerPastePlugin,
  SessionDirectiveBoundaryPlugin,
} from "./session-composer-lexical-plugins";
import { SessionAttachmentStrip } from "./session-attachment-strip";
import { SessionComposerDropRoot } from "./session-attachment-drop-overlay";
import { SessionComposerQueuedPrompts } from "./session-composer-queued-prompts";
import { SessionComposerActionRow } from "./session-composer-action-row";
import {
  useSessionBusyInputActions,
  type SessionBusyInputHandler,
} from "./hooks/use-session-busy-input-actions";
import type { SessionComposerState } from "./hooks/use-session-composer-state";
import { sessionComposerSendBlocker } from "./hooks/use-session-composer-send-gate";
import {
  SESSION_THREAD_CONTENT_INSET_DEFAULT,
  ThreadContentRail,
  type SessionThreadContentInset,
} from "./session-thread-content-rail";

export type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";

const EMPTY_QUEUED_PROMPTS: QueuedPrompt[] = [];

function removeSelectedActionToken(beforeSelection: string, text: string, token: string): string {
  let changedStart = 0;
  while (
    changedStart < beforeSelection.length &&
    changedStart < text.length &&
    beforeSelection[changedStart] === text[changedStart]
  ) {
    changedStart += 1;
  }

  let unchangedSuffix = 0;
  while (
    unchangedSuffix < beforeSelection.length - changedStart &&
    unchangedSuffix < text.length - changedStart &&
    beforeSelection[beforeSelection.length - unchangedSuffix - 1] ===
      text[text.length - unchangedSuffix - 1]
  ) {
    unchangedSuffix += 1;
  }

  const changedEnd = text.length - unchangedSuffix;
  const candidates: number[] = [];
  let searchFrom = 0;
  while (searchFrom <= text.length - token.length) {
    const candidate = text.indexOf(token, searchFrom);
    if (candidate < 0) break;
    if (candidate < changedEnd && candidate + token.length > changedStart) {
      candidates.push(candidate);
    }
    searchFrom = candidate + token.length;
  }
  const at = candidates.sort(
    (left, right) => Math.abs(left - changedStart) - Math.abs(right - changedStart)
  )[0];
  if (at === undefined) return text;
  const before = text.slice(0, at);
  const after = text.slice(at + token.length);
  return `${before}${before.endsWith(" ") && after.startsWith(" ") ? after.slice(1) : after}`;
}

export interface SessionComposerProps {
  /** Daemon-owned commands projected by the session page into the composer. */
  commandCatalog?: SessionComposerCommandCatalog;
  /** "loading" while the command catalog query has not resolved yet. */
  commandCatalogStatus?: "loading" | "ready";
  /** Notifies session orchestration when the native command catalog opens. */
  onCommandCatalogOpen?: () => void;
  /** Executes a standalone catalog action without inserting it into the prompt. */
  onCommandAction?: (token: string) => boolean;
  canPrompt: boolean;
  onCancelPrompt: () => void;
  onQueuePrompt?: SessionBusyInputHandler;
  onInterruptPrompt?: SessionBusyInputHandler;
  onSteerPrompt?: SessionBusyInputHandler;
  isBusyInputPending?: boolean;
  isSessionRunning?: boolean;
  allowBusyInput?: boolean;
  busyInputFenceAvailable?: boolean;
  queuedPrompts?: QueuedPrompt[];
  onRemoveQueuedPrompt?: (id: string) => void;
  onReplaceQueuedPrompt?: (prompt: QueuedPrompt, message: string) => Promise<unknown>;
  onSteerQueuedPrompt?: (prompt: QueuedPrompt) => void;
  contentInset?: SessionThreadContentInset;
  inactivePlaceholder?: string;
  /** Pending decision panel fused onto the composer top (permission/clarification dock). */
  decisionDock?: ReactNode;
  /** Runtime picker that applies to the next submitted prompt. */
  runtimeControl?: ReactNode;
  /** States where the session runs. Sits beside the runtime chip in the control row. */
  environmentControl?: ReactNode;
  /** Agent input support for the targeted runtime binding. */
  promptImageCapability?: SessionPromptCapability;
  /** Agent embedded-context support for the targeted runtime binding. */
  promptEmbeddedContextCapability?: SessionPromptCapability;
}

export type { SessionComposerCommandCatalog } from "./session-composer-command-menu";

/**
 * The session prompt composer. Idle: an accent Send disc submits to the runtime.
 * While a turn runs the phase changes coherently — Enter queues the draft (with a
 * visible hint), the primary disc becomes a danger Stop, and Queue/Interrupt stay
 * available. A direct Steer action replaces the fenced active turn with the current draft;
 * queued follow-ups keep their separate steer/edit/remove actions.
 */
export function SessionComposer({
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
}: SessionComposerProps & { composerState: SessionComposerState }) {
  const aui = useAui();
  const { clearComposer, setComposerInputElement, setComposerText, composerText, isRunning } =
    composerState;
  const [commandScope, setCommandScope] = useState<CommandCatalogScope>("inline");
  const commandFormatter = createSessionCommandFormatter(
    commandCatalog ?? { standaloneSections: [], inlineSkills: [] }
  );
  const trimmedComposerText = composerText.trim();
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
    (trimmedComposerText.length > 0 || promptAttachments.length > 0) &&
    attachmentBlocker === null &&
    !isBusyInputPending;
  const showBusyControls = runtimeRunning || isBusyInputPending;
  const canQueueFromInput = allowBusyInput && Boolean(onQueuePrompt);
  const hasQueuedPrompts = queuedPrompts.length > 0;
  const showQueuedStrip = hasQueuedPrompts && Boolean(onRemoveQueuedPrompt && onSteerQueuedPrompt);
  const {
    handleEditQueuedPrompt,
    handleInterruptAction,
    handleQueueAction,
    handleRemoveQueuedPrompt,
    handleSteerAction,
  } = useSessionBusyInputActions({
    canSubmitBusyInput,
    clearComposer,
    onInterruptPrompt,
    onQueuePrompt,
    onRemoveQueuedPrompt,
    onReplaceQueuedPrompt,
    onSteerPrompt,
    queuedPrompts,
    setComposerText,
    draft: { attachments: promptAttachments, message: trimmedComposerText },
  });

  return (
    // The composer zone continues the canvas — no top border; the transcript's
    // scroll edge does the separation.
    <div className="bg-canvas" data-testid="composer-shell">
      <ThreadContentRail
        inset={contentInset ?? SESSION_THREAD_CONTENT_INSET_DEFAULT}
        className="pt-1.5 pb-4"
      >
        <div className="group/composer relative flex min-w-0 flex-col">
          {decisionDock}
          {showQueuedStrip ? (
            <SessionComposerQueuedPrompts
              prompts={queuedPrompts}
              onSteer={onSteerQueuedPrompt!}
              onEdit={handleEditQueuedPrompt}
              onRemove={handleRemoveQueuedPrompt}
              disabled={isBusyInputPending}
              editDisabled={!onReplaceQueuedPrompt}
              steerDisabled={!busyInputFenceAvailable}
            />
          ) : null}
          <ComposerPrimitive.Unstable_TriggerPopoverRoot>
            <SessionComposerCommandMenu
              catalog={commandCatalog}
              scope={commandScope}
              isCatalogLoading={commandCatalogStatus === "loading"}
              onOpen={onCommandCatalogOpen}
            />
            <SessionComposerDropRoot disabled={!canPrompt}>
              <ComposerPrimitive.Root
                className={cn(
                  "flex flex-col gap-[7px] rounded-lg border border-line bg-elevated shadow-highlight",
                  "pt-[11px] pr-2.5 pb-2 pl-3.5",
                  "transition-colors duration-base ease-out",
                  "hover:border-line-strong focus-within:border-accent-dim",
                  "group-data-[dragging=true]/drop:border-accent-dim",
                  "group-has-[[data-slot=dock]]/composer:rounded-t-none",
                  showQueuedStrip ? "rounded-t-none" : null
                )}
              >
                <SessionAttachmentStrip
                  promptEmbeddedContextCapability={promptEmbeddedContextCapability}
                  promptImageCapability={promptImageCapability}
                />
                <LexicalComposerInput
                  data-testid="composer-input"
                  inert={!canPrompt}
                  placeholder={canPrompt ? "Send a message…" : inactivePlaceholder}
                  submitMode="enter"
                  formatter={commandFormatter}
                  directiveChip={SessionCommandChip}
                  directivePluginProps={{
                    onDirectiveSelect: item => {
                      const token = commandItemPresentation(item).token;
                      if (!onCommandAction?.(token)) return;
                      const beforeSelection = composerText;
                      window.setTimeout(() => {
                        setComposerText(
                          removeSelectedActionToken(
                            beforeSelection,
                            aui.composer.getState().text,
                            token
                          )
                        );
                      }, 0);
                    },
                  }}
                  className={cn(
                    "max-h-72 min-h-6 w-full text-small-body leading-relaxed text-fg",
                    !canPrompt ? "opacity-60" : null
                  )}
                >
                  <SessionComposerHandleBridge
                    onHandle={setComposerInputElement}
                    editableAriaLabel="Session prompt"
                  />
                  <SessionBusyEnterPlugin
                    queueActive={runtimeRunning && canQueueFromInput}
                    steerActive={
                      runtimeRunning &&
                      allowBusyInput &&
                      Boolean(onSteerPrompt) &&
                      busyInputFenceAvailable
                    }
                    onQueue={handleQueueAction}
                    onSteer={handleSteerAction}
                  />
                  <SessionCommandScopePlugin setScope={setCommandScope} />
                  <SessionDirectiveBoundaryPlugin />
                  <SessionComposerPastePlugin />
                </LexicalComposerInput>
                <SessionComposerActionRow
                  actionState={{
                    prompt: canPrompt ? "enabled" : "disabled",
                    enterHint: runtimeRunning && canQueueFromInput ? "queue" : "send",
                    controls: showBusyControls
                      ? {
                          kind: "busy",
                          submission: canSubmitBusyInput ? "enabled" : "disabled",
                          fence: busyInputFenceAvailable ? "available" : "unavailable",
                        }
                      : { kind: "send" },
                  }}
                  composerAttachmentCount={composerAttachments.length}
                  environmentControl={environmentControl}
                  handleInterruptAction={handleInterruptAction}
                  handleQueueAction={handleQueueAction}
                  handleSteerAction={handleSteerAction}
                  onCancelPrompt={onCancelPrompt}
                  onInterruptPrompt={allowBusyInput ? onInterruptPrompt : undefined}
                  onQueuePrompt={allowBusyInput ? onQueuePrompt : undefined}
                  onSteerPrompt={allowBusyInput ? onSteerPrompt : undefined}
                  promptEmbeddedContextCapability={promptEmbeddedContextCapability}
                  promptImageCapability={promptImageCapability}
                  runtimeControl={runtimeControl}
                />
              </ComposerPrimitive.Root>
            </SessionComposerDropRoot>
          </ComposerPrimitive.Unstable_TriggerPopoverRoot>
        </div>
      </ThreadContentRail>
    </div>
  );
}
