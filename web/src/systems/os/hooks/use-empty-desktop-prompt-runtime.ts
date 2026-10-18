import {
  useExternalStoreRuntime,
  type AppendMessage,
  type AssistantRuntime,
  type ThreadComposerRuntime,
  type ThreadMessage,
} from "@assistant-ui/react";
import { useEffect, useRef } from "react";

import { sessionStore } from "@/systems/session";

/**
 * Draft slot for the prompt typed before any session exists, one per project:
 * words typed on one project's empty desktop never show up on another's. It
 * lives beside real session drafts, so an unsent prompt survives a reload the
 * same way. Global scope, which cannot start a session, gets its own slot.
 */
export function emptyDesktopDraftId(workspaceId: string | null): string {
  return `desktop:new-session:${workspaceId ?? "global"}`;
}

const NO_MESSAGES: readonly ThreadMessage[] = [];

function promptTextOf(message: AppendMessage): string {
  return message.content
    .flatMap(part => (part.type === "text" ? [part.text] : []))
    .join("\n")
    .trim();
}

export interface EmptyDesktopPromptRuntimeOptions {
  /** The composer's draft slot ({@link emptyDesktopDraftId}). */
  draftId: string;
  /** The composer accepts a send right now. */
  canSend: boolean;
  /**
   * Receives the prompt the operator sent. The words stay in the composer —
   * inert while the start is in flight — until `release` says a session owns
   * them; a start that fails leaves them there to edit or send again.
   */
  onSubmit(prompt: string, release: () => void): Promise<void> | void;
  /**
   * Runs when a session took the prompt, after its draft slot is emptied. The
   * host remounts the composer on a fresh runtime: clearing the live composer
   * in place races the draft sync, which writes back the last text it rendered.
   */
  onRelease(): void;
}

/**
 * A thread runtime with no transcript: it exists only so the real session
 * composer can take a first message before there is a session to send it to.
 */
export function useEmptyDesktopPromptRuntime({
  draftId,
  canSend,
  onSubmit,
  onRelease,
}: EmptyDesktopPromptRuntimeOptions): AssistantRuntime {
  const composerRef = useRef<ThreadComposerRuntime | null>(null);
  const runtime = useExternalStoreRuntime({
    messages: NO_MESSAGES,
    isRunning: false,
    isSendDisabled: !canSend,
    onNew: async message => {
      const prompt = promptTextOf(message);
      if (prompt === "") return;
      // Sending empties the composer; put the words straight back so they stay
      // visible until a session owns them.
      composerRef.current?.setText(prompt);
      await onSubmit(prompt, () => {
        sessionStore.trigger.composerDraftDiscarded({ sessionId: draftId });
        onRelease();
      });
    },
  });
  useEffect(() => {
    composerRef.current = runtime.thread.composer;
  }, [runtime]);
  return runtime;
}
