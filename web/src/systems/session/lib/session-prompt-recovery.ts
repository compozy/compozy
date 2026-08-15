import type { UIMessage } from "ai";

export interface RejectedSessionPromptDraft {
  text: string;
}

export interface RestoredSessionPromptDraft {
  files: readonly File[];
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
    this.draft = { text: latestUserPromptText(messages) };
  }

  public acknowledge(): void {
    this.draft = null;
  }

  public recover(files: readonly File[]): void {
    const draft = this.draft;
    this.draft = null;
    if (!draft) return;
    const restored = { files, text: draft.text };
    for (const listener of this.listeners) {
      listener(restored);
    }
  }

  public subscribe(listener: RecoveryListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}
