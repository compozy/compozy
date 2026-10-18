import { AssistantRuntimeProvider } from "@assistant-ui/react";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { useSessionComposerState } from "@/components/assistant-ui/hooks/use-session-composer-state";
import { SessionComposer } from "@/components/assistant-ui/session-composer";

import {
  useEmptyDesktopPromptRuntime,
  type EmptyDesktopPromptRuntimeOptions,
} from "../hooks/use-empty-desktop-prompt-runtime";

/** Nothing runs before the session exists, so there is never a turn to stop. */
function ignoreCancel() {}

export interface OsEmptyDesktopComposerProps {
  /**
   * Where the unsent prompt is kept ({@link emptyDesktopDraftId}). Mount one
   * composer per slot (`key={draftId}`): the prompt runtime holds the text, so
   * a new slot needs a new runtime or the old words would follow it.
   */
  draftId: string;
  canPrompt: boolean;
  onSubmit: EmptyDesktopPromptRuntimeOptions["onSubmit"];
  placeholder: string;
  /** Shown in place of the placeholder while the composer cannot take a prompt. */
  inactivePlaceholder: string;
  /** Sits in the composer's tool row, where a session shows its agent and model. */
  control?: ReactNode;
  /**
   * Typing while nothing holds focus starts the prompt here. The composer does
   * not take focus on its own: an editable swallows the shell's chords (desktop
   * switching, ⌘E…), and an empty desktop must keep them.
   */
  typeToFocus?: boolean;
}

/** Focus rests on the page or the desk itself, not on any control. */
function focusIsFree(active: Element | null): boolean {
  return active === null || active === document.body || active.id === "app-content";
}

/**
 * The session composer, hosted before a session exists: same input, Enter
 * behavior, and send control as a session window, without the in-session
 * extras (commands, attachments, queue) that need one.
 */
export function OsEmptyDesktopComposer({
  draftId,
  canPrompt,
  onSubmit,
  placeholder,
  inactivePlaceholder,
  control,
  typeToFocus = false,
}: OsEmptyDesktopComposerProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  // Each released prompt starts the next one on a fresh composer.
  const [generation, setGeneration] = useState(0);

  useEffect(() => {
    if (!typeToFocus || !canPrompt) return;
    // Window bubble runs after the shell's document listener, so a chord it
    // claimed arrives here already prevented and stays the shell's.
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing || event.repeat) return;
      if (event.metaKey || event.ctrlKey || event.altKey || event.key.length !== 1) return;
      if (!focusIsFree(document.activeElement)) return;
      // Focus moves before the key's text is inserted, so the character lands in the prompt.
      rootRef.current?.querySelector<HTMLElement>('[contenteditable="true"]')?.focus();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [canPrompt, typeToFocus]);

  return (
    <div ref={rootRef} className="w-full" data-testid="os-desk-composer">
      <EmptyDesktopPromptSurface
        key={generation}
        draftId={draftId}
        canPrompt={canPrompt}
        onSubmit={onSubmit}
        onRelease={() => setGeneration(current => current + 1)}
        placeholder={placeholder}
        inactivePlaceholder={inactivePlaceholder}
        control={control}
      />
    </div>
  );
}

function EmptyDesktopPromptSurface({
  onSubmit,
  onRelease,
  ...props
}: Omit<OsEmptyDesktopComposerProps, "typeToFocus"> & { onRelease: () => void }) {
  const runtime = useEmptyDesktopPromptRuntime({
    draftId: props.draftId,
    canSend: props.canPrompt,
    onSubmit,
    onRelease,
  });
  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <EmptyDesktopSessionComposer {...props} />
    </AssistantRuntimeProvider>
  );
}

function EmptyDesktopSessionComposer({
  draftId,
  canPrompt,
  placeholder,
  inactivePlaceholder,
  control,
}: Omit<OsEmptyDesktopComposerProps, "onSubmit" | "typeToFocus">) {
  const composerState = useSessionComposerState(draftId);
  return (
    <SessionComposer
      sessionId={draftId}
      composerState={composerState}
      canPrompt={canPrompt}
      onCancelPrompt={ignoreCancel}
      allowBusyInput={false}
      placeholder={placeholder}
      inactivePlaceholder={inactivePlaceholder}
      attachments={false}
      backdrop={false}
      runtimeControl={control}
    />
  );
}
