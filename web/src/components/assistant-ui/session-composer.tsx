import { ComposerPrimitive } from "@assistant-ui/react";
import { LexicalComposerInput } from "@assistant-ui/react-lexical";
import type { ReactNode } from "react";

import { InputGroup } from "@compozy/ui";

import { cn } from "@/lib/utils";
import {
  SessionQueueStrip,
  type QueuedPrompt,
  type SessionBusyInputMode,
  type SessionSendOutcome,
  type SessionSteerDelivery,
  type UnconfirmedSend,
} from "@/systems/session";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import { commandItemPresentation } from "./session-command-menu-model";
import { SessionCommandChip } from "./session-composer-chip";
import {
  SessionComposerCommandMenu,
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
import { SessionComposerActionRow } from "./session-composer-action-row";
import { SessionComposerFeedbackNote } from "./session-composer-feedback-note";
import {
  useSessionComposerActionsContext,
  useSessionComposerMetaContext,
  useSessionComposerStateContext,
} from "./hooks/use-session-composer-context";
import type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";
import type { SessionComposerState } from "./hooks/use-session-composer-state";
import { ThreadContentRail, type SessionThreadContentInset } from "./session-thread-content-rail";
import { SESSION_THREAD_CONTENT_INSET_DEFAULT } from "./session-thread-content-rail-constants";
import { SessionComposerProvider } from "./session-composer-provider";

export type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";

/**
 * Where the composer stands in a stop: `stopping` from the first activation
 * until the daemon confirms the stop landed (US-009.AC-1); `idle` otherwise.
 */
export type SessionComposerStopPhase = "idle" | "stopping";

export interface SessionComposerProps {
  commandCatalog?: SessionComposerCommandCatalog;
  commandCatalogStatus?: "loading" | "ready";
  onCommandCatalogOpen?: () => void;
  onCommandAction?: (token: string) => boolean;
  canPrompt: boolean;
  onCancelPrompt: () => void;
  onQueuePrompt?: SessionBusyInputHandler;
  onInterruptPrompt?: SessionBusyInputHandler;
  onSteerPrompt?: SessionBusyInputHandler;
  isBusyInputPending?: boolean;
  isSessionRunning?: boolean;
  /**
   * While `stopping`, the primary control reads "Stopping…" and drops a second
   * activation; steer and interrupt cannot be honored, queue still can.
   */
  stopPhase?: SessionComposerStopPhase;
  allowBusyInput?: boolean;
  /** The daemon-owned follow-up default Enter performs during a turn (ADR-002). */
  busyInputDefaultMode?: SessionBusyInputMode;
  /** How a steer lands on this agent, from the session resource; `null` when unknown. */
  busyInputSteerDelivery?: SessionSteerDelivery | null;
  queuedPrompts?: QueuedPrompt[];
  onRemoveQueuedPrompt?: (id: string) => void;
  onReplaceQueuedPrompt?: (prompt: QueuedPrompt, message: string) => Promise<unknown>;
  onSteerQueuedPrompt?: (prompt: QueuedPrompt) => void;
  /** Explicit clear-all (`DELETE …/prompt/queue`); the strip offers it only when present. */
  onClearQueue?: () => Promise<unknown>;
  /** The daemon's queue cap once a refusal named it; at cap the queue affordances are absent. */
  queueCap?: number | null;
  /** Client-local sends whose acknowledgment was lost; Retry replays the same identity. */
  unconfirmedSends?: UnconfirmedSend[];
  onRetryUnconfirmedSend?: (id: string) => Promise<SessionSendOutcome | void>;
  onDiscardUnconfirmedSend?: (id: string) => void;
  contentInset?: SessionThreadContentInset;
  inactivePlaceholder?: string;
  decisionDock?: ReactNode;
  runtimeControl?: ReactNode;
  environmentControl?: ReactNode;
  contextControl?: ReactNode;
  promptImageCapability?: SessionPromptCapability;
  promptEmbeddedContextCapability?: SessionPromptCapability;
  sessionId: string;
  quoteSlot?: ReactNode;
  /** Replaces the idle placeholder, for hosts without the command catalog. */
  placeholder?: string;
  /** `false` removes every way to attach files (button, drop, paste), for hosts that carry text only. */
  attachments?: boolean;
  /** `false` drops the pane fill around the composer, for hosts that sit on the desk wallpaper. */
  backdrop?: boolean;
}

export type { SessionComposerCommandCatalog } from "./session-composer-command-menu";

export function SessionComposer(
  props: SessionComposerProps & { composerState: SessionComposerState }
) {
  return (
    <SessionComposerProvider {...props}>
      <SessionComposerSurface />
    </SessionComposerProvider>
  );
}

function SessionComposerSurface() {
  const state = useSessionComposerStateContext();
  const meta = useSessionComposerMetaContext();
  return (
    <div className={meta.backdrop ? "bg-canvas" : undefined} data-testid="composer-shell">
      <ThreadContentRail
        inset={meta.contentInset ?? SESSION_THREAD_CONTENT_INSET_DEFAULT}
        className="pt-2 pb-5"
      >
        <div className="group/composer relative flex min-w-0 flex-col gap-2">
          {meta.decisionDock}
          {state.showQueuedStrip ? <SessionComposerQueue /> : null}
          <SessionComposerEditor />
        </div>
      </ThreadContentRail>
    </div>
  );
}

function SessionComposerQueue() {
  const actions = useSessionComposerActionsContext();
  const meta = useSessionComposerMetaContext();
  return (
    <SessionQueueStrip
      prompts={meta.queuedPrompts}
      unconfirmedSends={meta.unconfirmedSends}
      queueCap={meta.queueCap}
      onSteer={meta.onSteerQueuedPrompt!}
      onRemove={actions.handleRemoveQueuedPrompt}
      onSaveEdit={meta.onReplaceQueuedPrompt ? actions.handleSaveQueuedPromptEdit : undefined}
      onClear={meta.onClearQueue}
      onRetryUnconfirmed={
        meta.onRetryUnconfirmedSend ? actions.handleRetryUnconfirmedSend : undefined
      }
      onDiscardUnconfirmed={meta.onDiscardUnconfirmedSend}
      disabled={meta.isBusyInputPending}
    />
  );
}

function SessionComposerEditor() {
  const state = useSessionComposerStateContext();
  const meta = useSessionComposerMetaContext();
  return (
    <ComposerPrimitive.Unstable_TriggerPopoverRoot>
      <SessionComposerCommandMenu
        catalog={meta.commandCatalog}
        scope={state.commandScope}
        isCatalogLoading={meta.commandCatalogStatus === "loading"}
        onOpen={meta.onCommandCatalogOpen}
      />
      <SessionComposerDropRoot disabled={!meta.canPrompt || !meta.attachments}>
        <InputGroup variant="composer">
          <ComposerPrimitive.Root
            className="flex min-w-0 flex-col gap-transcript-inline-gap"
            data-testid="session-composer-stack"
          >
            {meta.quoteSlot}
            <SessionAttachmentStrip
              promptEmbeddedContextCapability={meta.promptEmbeddedContextCapability}
              promptImageCapability={meta.promptImageCapability}
            />
            <SessionComposerInput />
            {state.feedback ? <SessionComposerFeedbackNote feedback={state.feedback} /> : null}
            <SessionComposerControls />
          </ComposerPrimitive.Root>
        </InputGroup>
      </SessionComposerDropRoot>
    </ComposerPrimitive.Unstable_TriggerPopoverRoot>
  );
}

function SessionComposerInput() {
  const state = useSessionComposerStateContext();
  const actions = useSessionComposerActionsContext();
  const meta = useSessionComposerMetaContext();
  return (
    <LexicalComposerInput
      data-testid="composer-input"
      inert={!meta.canPrompt}
      placeholder={
        meta.canPrompt
          ? (meta.placeholder ?? "Send a message — type / for commands")
          : meta.inactivePlaceholder
      }
      submitMode="enter"
      formatter={state.commandFormatter}
      directiveChip={SessionCommandChip}
      directivePluginProps={{
        onDirectiveSelect: item => {
          const token = commandItemPresentation(item).token;
          if (!meta.onCommandAction?.(token)) return;
          actions.recordPendingCommandAction(token, state.composerText);
        },
      }}
      className={cn(
        "max-h-72 min-h-6 w-full text-body leading-relaxed text-fg",
        !meta.canPrompt ? "opacity-60" : null
      )}
    >
      <SessionComposerHandleBridge
        onHandle={actions.setComposerInputElement}
        editableAriaLabel="Session prompt"
      />
      <SessionBusyEnterPlugin active={state.busyEnterActive} onEnter={actions.handleEnterAction} />
      <SessionCommandScopePlugin setScope={actions.setCommandScope} />
      <SessionDirectiveBoundaryPlugin />
      {meta.attachments ? <SessionComposerPastePlugin /> : null}
    </LexicalComposerInput>
  );
}

function SessionComposerControls() {
  const state = useSessionComposerStateContext();
  const actions = useSessionComposerActionsContext();
  const meta = useSessionComposerMetaContext();
  return (
    <SessionComposerActionRow
      attachments={meta.attachments}
      hasStagedQuote={state.hasStagedQuote}
      sessionId={meta.sessionId}
      actionState={{
        prompt: meta.canPrompt ? "enabled" : "disabled",
        enterHint: state.enterHint,
        controls: state.showBusyControls
          ? {
              kind: "busy",
              stopping: state.stopping,
              submission: state.canSubmitBusyInput ? "enabled" : "disabled",
            }
          : { kind: "send" },
      }}
      busyInputSteerDelivery={meta.busyInputSteerDelivery}
      composerAttachmentCount={state.composerAttachmentCount}
      environmentControl={meta.environmentControl}
      contextControl={meta.contextControl}
      handleDisconnectedSend={
        state.transportDisconnected ? actions.handleDisconnectedSend : undefined
      }
      handleInterruptAction={actions.handleInterruptAction}
      handleQueueAction={actions.handleQueueAction}
      handleSteerAction={actions.handleSteerAction}
      onCancelPrompt={meta.onCancelPrompt}
      onInterruptPrompt={
        meta.allowBusyInput && !state.stopping ? meta.onInterruptPrompt : undefined
      }
      onQueuePrompt={meta.allowBusyInput && !state.queueFull ? meta.onQueuePrompt : undefined}
      onSteerPrompt={meta.allowBusyInput && !state.stopping ? meta.onSteerPrompt : undefined}
      promptEmbeddedContextCapability={meta.promptEmbeddedContextCapability}
      promptImageCapability={meta.promptImageCapability}
      runtimeControl={meta.runtimeControl}
    />
  );
}
