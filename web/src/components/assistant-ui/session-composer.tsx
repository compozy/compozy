import { ComposerPrimitive, useAui } from "@assistant-ui/react";
import { LexicalComposerInput } from "@assistant-ui/react-lexical";
import { CornerDownRight, ListPlus, Scissors, Square } from "lucide-react";
import { useState, type ReactNode } from "react";

import { cn } from "@/lib/utils";
import type { QueuedPrompt } from "@/systems/session";
import { Button } from "@compozy/ui";
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
import { SessionAttachButton } from "./session-attach-button";
import { SessionAttachmentStrip } from "./session-attachment-strip";
import { SessionComposerDropRoot } from "./session-attachment-drop-overlay";
import { SessionComposerQueuedPrompts } from "./session-composer-queued-prompts";
import { SessionComposerSendButton } from "./session-composer-send-button";
import {
  useSessionBusyInputActions,
  type SessionBusyInputHandler,
} from "./hooks/use-session-busy-input-actions";
import type { SessionComposerState } from "./hooks/use-session-composer-state";
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
  /** Session ACP prompt_image cap. Absent/false refuses image tiles. */
  promptImage?: boolean;
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
  promptImage = false,
}: SessionComposerProps & { composerState: SessionComposerState }) {
  const aui = useAui();
  const { clearComposer, setComposerInputElement, setComposerText, composerText, isRunning } =
    composerState;
  const [commandScope, setCommandScope] = useState<CommandCatalogScope>("inline");
  const commandFormatter = createSessionCommandFormatter(
    commandCatalog ?? { standaloneSections: [], inlineSkills: [] }
  );
  const trimmedComposerText = composerText.trim();
  const runtimeRunning = isRunning || isSessionRunning;
  const canSubmitBusyInput =
    runtimeRunning &&
    canPrompt &&
    allowBusyInput &&
    trimmedComposerText.length > 0 &&
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
    trimmedComposerText,
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
              {({ isDragging }) => (
                <ComposerPrimitive.Root
                  className={cn(
                    "flex flex-col gap-[7px] rounded-lg border border-line bg-elevated shadow-highlight",
                    "pt-[11px] pr-2.5 pb-2 pl-3.5",
                    "transition-colors duration-base ease-out",
                    "hover:border-line-strong focus-within:border-accent-dim",
                    "group-has-[[data-slot=dock]]/composer:rounded-t-none",
                    showQueuedStrip ? "rounded-t-none" : null,
                    isDragging ? "border-accent-dim" : null
                  )}
                >
                  <SessionAttachmentStrip promptImage={promptImage} />
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
                  <div className="flex min-h-7 flex-wrap items-center gap-2">
                    {runtimeControl || environmentControl ? (
                      <div className="flex min-w-0 items-center gap-2">
                        {runtimeControl}
                        {environmentControl}
                      </div>
                    ) : null}
                    {canPrompt ? <SessionAttachButton /> : null}
                    {canPrompt ? (
                      <span
                        data-testid="composer-enter-hint"
                        className="inline-flex items-center gap-[5px] text-[10.5px] text-faint"
                      >
                        <kbd className="rounded-xs border border-line bg-canvas-soft px-1 py-px font-mono text-[9px] not-italic text-subtle">
                          ⏎
                        </kbd>
                        {runtimeRunning && canQueueFromInput ? "queue" : "send"}
                      </span>
                    ) : null}
                    <span className="flex-1" />

                    {showBusyControls ? (
                      <>
                        {allowBusyInput && onQueuePrompt ? (
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={handleQueueAction}
                            disabled={!canSubmitBusyInput}
                            data-testid="composer-queue-button"
                          >
                            <ListPlus className="size-3" />
                            Queue
                          </Button>
                        ) : null}
                        {allowBusyInput && onSteerPrompt ? (
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={handleSteerAction}
                            disabled={!canSubmitBusyInput || !busyInputFenceAvailable}
                            data-testid="composer-steer-button"
                          >
                            <CornerDownRight className="size-3" />
                            Steer
                          </Button>
                        ) : null}
                        {allowBusyInput && onInterruptPrompt ? (
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={handleInterruptAction}
                            disabled={!canSubmitBusyInput || !busyInputFenceAvailable}
                            data-testid="composer-interrupt-button"
                          >
                            <Scissors className="size-3" />
                            Interrupt
                          </Button>
                        ) : null}
                        <button
                          type="button"
                          onClick={onCancelPrompt}
                          aria-label="Stop generation"
                          data-testid="composer-stop-button"
                          className={cn(
                            "inline-flex size-7 items-center justify-center rounded-full",
                            "border border-line text-muted transition-colors duration-base ease-out",
                            "hover:border-transparent hover:bg-danger-tint hover:text-danger",
                            "focus-visible:shadow-focus-ring focus-visible:outline-none"
                          )}
                        >
                          <Square className="size-3 fill-current" />
                        </button>
                      </>
                    ) : (
                      <SessionComposerSendButton canPrompt={canPrompt} promptImage={promptImage} />
                    )}
                  </div>
                </ComposerPrimitive.Root>
              )}
            </SessionComposerDropRoot>
          </ComposerPrimitive.Unstable_TriggerPopoverRoot>
        </div>
      </ThreadContentRail>
    </div>
  );
}
