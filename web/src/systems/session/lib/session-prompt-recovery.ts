import type { UIMessage } from "ai";

import type { TerminalQuote } from "@/systems/terminal/parts";

import { splitQuotedPrompt } from "./session-terminal-quote-prompt";

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
 * switch): the restored draft then waits, in arrival order, for the next
 * composer of that session instead of being dropped. One composer receives
 * it, so a session open twice never restores the same files into both.
 */
export class SessionPromptRecovery {
  private readonly drafts = new Map<string, RejectedSessionPromptDraft>();
  private readonly restored = new Map<string, RestoredSessionPromptDraft[]>();
  private readonly listeners = new Map<string, Set<RecoveryListener>>();

  /** Forgets staged and pending drafts; suites sharing the app-scoped owner start clean. */
  public reset(): void {
    this.drafts.clear();
    this.restored.clear();
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
    listeners.add(listener);
    this.listeners.set(key, listeners);
    const pending = this.restored.get(key) ?? [];
    this.restored.delete(key);
    for (const draft of pending) listener(draft);
    return () => {
      listeners.delete(listener);
      if (listeners.size === 0) this.listeners.delete(key);
    };
  }

  private deliver(key: string, draft: RestoredSessionPromptDraft): void {
    const [listener] = this.listeners.get(key) ?? [];
    if (!listener) {
      this.restored.set(key, [...(this.restored.get(key) ?? []), draft]);
      return;
    }
    listener(draft);
  }
}

/**
 * The browser's one recovery owner. A session runtime is rebuilt whenever its
 * workspace or session identity changes, but the outcome of a prompt already on
 * the wire must still reach that session's composer.
 */
export const sessionPromptRecovery = new SessionPromptRecovery();
