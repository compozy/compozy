import type { UIMessage } from "ai";

import type { TerminalQuote } from "@/systems/terminal/parts";

import { splitQuotedPrompt } from "./session-terminal-quote-prompt";
import type { SessionSendEnvelope } from "./session-unconfirmed-send";

export interface RejectedSessionPromptDraft {
  quote: TerminalQuote | null;
  text: string;
}

export interface RestoredSessionPromptDraft {
  files: readonly File[];
  quote: TerminalQuote | null;
  text: string;
}

type RecoveryListener = (draft: RestoredSessionPromptDraft) => void;
type UnconfirmedListener = (envelope: SessionSendEnvelope) => void;

export interface SessionPromptRecoveryScope {
  workspaceId: string;
  sessionId: string;
}

function recoveryScopeKey(scope: SessionPromptRecoveryScope): string {
  return JSON.stringify([scope.workspaceId, scope.sessionId]);
}

function latestUserPromptText(messages: readonly UIMessage[]): string {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message?.role !== "user") continue;
    const text: string[] = [];
    for (const part of message.parts) {
      if (part.type === "text") text.push(part.text);
    }
    return text.join("\n");
  }
  return "";
}

function stagedDraftKey(scope: SessionPromptRecoveryScope, messageId: string): string {
  return JSON.stringify([scope.workspaceId, scope.sessionId, messageId]);
}

/**
 * Holds each in-flight composer draft, by prompt identity, until the daemon
 * either accepts that prompt or rejects it. The adapter supplies the original
 * Files on rejection, allowing the public composer API to restore previews and
 * editable text.
 *
 * Prompts of one session can overlap once a runtime closes mid-request and a
 * new one sends again, so every outcome settles only its own `message_id`. A
 * rejection can land after its composer is gone (navigation, a profile
 * switch), or the composer can go away before it finished restoring: the
 * restored draft stays owned here, in arrival order, until a composer of that
 * session settles it. One composer receives it at a time, so a session open
 * twice never restores the same files into both.
 */
export class SessionPromptRecovery {
  private readonly drafts = new Map<string, RejectedSessionPromptDraft>();
  private readonly restored = new Map<string, RestoredSessionPromptDraft[]>();
  private readonly listeners = new Map<string, Set<RecoveryListener>>();
  private readonly unconfirmed = new Map<string, SessionSendEnvelope[]>();
  private readonly unconfirmedListeners = new Map<string, Set<UnconfirmedListener>>();

  /** Forgets staged and pending drafts; suites sharing the app-scoped owner start clean. */
  public reset(): void {
    this.drafts.clear();
    this.restored.clear();
    this.unconfirmed.clear();
  }

  /**
   * Keeps the identity of a send whose acknowledgment was lost until a session
   * page takes it. A detached send can lose it after its page is gone; held
   * here, the next page of that session still offers Retry with the same
   * identity instead of a fresh send that could run twice. Memory only: a
   * reload drops it, and daemon replay protection still holds.
   */
  public retainUnconfirmed(scope: SessionPromptRecoveryScope, envelope: SessionSendEnvelope): void {
    const key = recoveryScopeKey(scope);
    const listeners = this.unconfirmedListeners.get(key);
    if (listeners && listeners.size > 0) {
      for (const listener of listeners) listener(envelope);
      return;
    }
    this.unconfirmed.set(key, [...(this.unconfirmed.get(key) ?? []), envelope]);
  }

  /** A session page receives its retained unconfirmed sends, then each new one as it is lost. */
  public subscribeUnconfirmed(
    scope: SessionPromptRecoveryScope,
    listener: UnconfirmedListener
  ): () => void {
    const key = recoveryScopeKey(scope);
    const listeners = this.unconfirmedListeners.get(key) ?? new Set<UnconfirmedListener>();
    listeners.add(listener);
    this.unconfirmedListeners.set(key, listeners);
    const pending = this.unconfirmed.get(key) ?? [];
    this.unconfirmed.delete(key);
    for (const envelope of pending) listener(envelope);
    return () => {
      listeners.delete(listener);
      if (listeners.size === 0) this.unconfirmedListeners.delete(key);
    };
  }

  public stage(
    scope: SessionPromptRecoveryScope,
    messageId: string,
    { messages }: { messages: readonly UIMessage[] }
  ): void {
    const { annotation, quote } = splitQuotedPrompt(latestUserPromptText(messages));
    this.drafts.set(stagedDraftKey(scope, messageId), { quote, text: annotation });
  }

  public acknowledge(scope: SessionPromptRecoveryScope, messageId: string): void {
    this.drafts.delete(stagedDraftKey(scope, messageId));
  }

  public recover(
    scope: SessionPromptRecoveryScope,
    messageId: string,
    files: readonly File[]
  ): void {
    const key = stagedDraftKey(scope, messageId);
    const draft = this.drafts.get(key);
    this.drafts.delete(key);
    if (!draft) return;
    this.deliver(recoveryScopeKey(scope), { files, quote: draft.quote, text: draft.text });
  }

  public subscribe(scope: SessionPromptRecoveryScope, listener: RecoveryListener): () => void {
    const key = recoveryScopeKey(scope);
    const listeners = this.listeners.get(key) ?? new Set<RecoveryListener>();
    const owner = listeners.size === 0;
    listeners.add(listener);
    this.listeners.set(key, listeners);
    if (owner) this.handOver(key, listener);
    return () => {
      const wasOwner = this.owner(key) === listener;
      listeners.delete(listener);
      if (listeners.size === 0) {
        this.listeners.delete(key);
        return;
      }
      // Drafts the departing composer never settled go to the next one.
      const next = this.owner(key);
      if (wasOwner && next) this.handOver(key, next);
    };
  }

  /** A composer confirms it restored this draft; only then does recovery let go of it. */
  public settle(scope: SessionPromptRecoveryScope, draft: RestoredSessionPromptDraft): void {
    const key = recoveryScopeKey(scope);
    const pending = (this.restored.get(key) ?? []).filter(candidate => candidate !== draft);
    if (pending.length === 0) this.restored.delete(key);
    else this.restored.set(key, pending);
  }

  private deliver(key: string, draft: RestoredSessionPromptDraft): void {
    this.restored.set(key, [...(this.restored.get(key) ?? []), draft]);
    this.owner(key)?.(draft);
  }

  private owner(key: string): RecoveryListener | undefined {
    const [listener] = this.listeners.get(key) ?? [];
    return listener;
  }

  private handOver(key: string, listener: RecoveryListener): void {
    for (const draft of this.restored.get(key) ?? []) listener(draft);
  }
}

/**
 * The browser's one recovery owner. A session runtime is rebuilt whenever its
 * workspace or session identity changes, but the outcome of a prompt already on
 * the wire must still reach that session's composer.
 */
export const sessionPromptRecovery = new SessionPromptRecovery();

/** Clears the app-scoped recovery owner; test suites outside the session system use this seam. */
export function resetSessionPromptRecovery(): void {
  sessionPromptRecovery.reset();
}
