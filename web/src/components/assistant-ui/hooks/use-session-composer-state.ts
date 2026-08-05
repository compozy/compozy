import { useEffect, useLayoutEffect, useRef, type RefCallback } from "react";
import { useAui, useAuiEvent, useAuiState } from "@assistant-ui/react";
import { toast } from "sonner";

import { sessionStore, useSessionComposerDraft } from "@/systems/session";

export interface SessionComposerState {
  clearComposer: () => void;
  persistComposerText: (text: string) => void;
  setComposerText: (text: string) => void;
  setComposerInputElement: RefCallback<HTMLTextAreaElement>;
  prefillComposer: (text: string) => void;
  composerText: string;
  isRunning: boolean;
}

export function useSessionComposerState(sessionId: string): SessionComposerState {
  const aui = useAui();
  const draftText = useSessionComposerDraft(sessionId);
  const composerText = useAuiState(state => state.composer.text);
  const isRunning = useAuiState(state => state.thread.isRunning);
  const composerInputElementRef = useRef<HTMLTextAreaElement>(null);
  const hydratedSessionIdRef = useRef<string | null>(null);
  const hydratedDraftTextRef = useRef<string | null>(null);
  const skipNextComposerSyncRef = useRef(false);

  const setComposerInputElement: RefCallback<HTMLTextAreaElement> = element => {
    composerInputElementRef.current = element;
  };

  const clearDraftForSession = () => sessionStore.trigger.composerDraftDiscarded({ sessionId });

  const clearComposer = () => {
    aui.composer.setText("");
    sessionStore.trigger.composerDraftDiscarded({ sessionId });
  };

  const persistComposerText = (text: string) => {
    sessionStore.trigger.composerDraftChanged({ sessionId, text });
  };

  const setComposerText = (text: string) => {
    aui.composer.setText(text);
    persistComposerText(text);
  };

  const prefillComposer = (text: string) => {
    const currentText = aui.composer.getState().text;
    if (currentText.trim().length > 0 && currentText !== text) {
      toast.warning("Send or discard the current draft before prefilling a Goal command.");
      composerInputElementRef.current?.focus();
      return;
    }
    setComposerText(text);
    composerInputElementRef.current?.focus();
  };

  useLayoutEffect(() => {
    const sameSession = hydratedSessionIdRef.current === sessionId;
    const composerText = aui.composer.getState().text;
    if (sameSession && composerText !== hydratedDraftTextRef.current) {
      return;
    }
    hydratedSessionIdRef.current = sessionId;
    hydratedDraftTextRef.current = draftText;
    skipNextComposerSyncRef.current = true;
    aui.composer.setText(draftText);
  }, [aui, draftText, sessionId]);

  useEffect(() => {
    if (skipNextComposerSyncRef.current) {
      skipNextComposerSyncRef.current = false;
      return;
    }
    if (composerText === draftText) return;
    sessionStore.trigger.composerDraftChanged({ sessionId, text: composerText });
  }, [composerText, draftText, sessionId]);

  useAuiEvent("composer.send", clearDraftForSession);

  return {
    clearComposer,
    composerText,
    isRunning,
    persistComposerText,
    prefillComposer,
    setComposerInputElement,
    setComposerText,
  };
}
