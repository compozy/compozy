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
  private draft: RejectedSessionPromptDraft | null = null;
  private readonly listeners = new Set<RecoveryListener>();

  public stage({ messages }: { messages: readonly UIMessage[] }): void {
    const { annotation, quote } = splitQuotedPrompt(latestUserPromptText(messages));
    this.draft = { quote, text: annotation };
  }

  public acknowledge(): void {
    this.draft = null;
  }

  public recover(files: readonly File[]): void {
    const draft = this.draft;
    this.draft = null;
    if (!draft) return;
    const restored = { files, quote: draft.quote, text: draft.text };
    for (const listener of this.listeners) {
      listener(restored);
    }
  }

  public subscribe(listener: RecoveryListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}
