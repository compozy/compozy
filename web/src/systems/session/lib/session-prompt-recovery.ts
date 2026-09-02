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

/**
 * Holds only the in-flight composer draft until the daemon either accepts the
 * prompt or rejects it. The adapter supplies the original Files on rejection,
 * allowing the public composer API to restore previews and editable text.
 */
export class SessionPromptRecovery {
  private readonly drafts = new Map<string, RejectedSessionPromptDraft>();
  private readonly listeners = new Map<string, Set<RecoveryListener>>();

  public stage(
    scope: SessionPromptRecoveryScope,
    { messages }: { messages: readonly UIMessage[] }
  ): void {
    const { annotation, quote } = splitQuotedPrompt(latestUserPromptText(messages));
    this.drafts.set(recoveryScopeKey(scope), { quote, text: annotation });
  }

  public acknowledge(scope: SessionPromptRecoveryScope): void {
    this.drafts.delete(recoveryScopeKey(scope));
  }

  public recover(scope: SessionPromptRecoveryScope, files: readonly File[]): void {
    const key = recoveryScopeKey(scope);
    const draft = this.drafts.get(key);
    this.drafts.delete(key);
    if (!draft) return;
    const restored = { files, quote: draft.quote, text: draft.text };
    for (const listener of this.listeners.get(key) ?? []) {
      listener(restored);
    }
  }

  public subscribe(scope: SessionPromptRecoveryScope, listener: RecoveryListener): () => void {
    const key = recoveryScopeKey(scope);
    const listeners = this.listeners.get(key) ?? new Set<RecoveryListener>();
    listeners.add(listener);
    this.listeners.set(key, listeners);
    return () => {
      listeners.delete(listener);
      if (listeners.size === 0) this.listeners.delete(key);
    };
  }
}
