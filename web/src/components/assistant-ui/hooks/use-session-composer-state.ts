import { useEffect, useRef, type RefCallback } from "react";
import { useAui, useAuiEvent, useAuiState } from "@assistant-ui/react";
import { toast } from "sonner";

import { useSessionStore } from "@/systems/session/hooks/use-session-store";

export interface SessionComposerState {
  clearComposer: () => void;
  setComposerText: (text: string) => void;
  setComposerInputElement: RefCallback<HTMLTextAreaElement>;
  prefillComposer: (text: string) => void;
  composerText: string;
  isRunning: boolean;
}

export function useSessionComposerState(sessionId: string): SessionComposerState {
  const aui = useAui();
  const draftText = useSessionStore(state => state.drafts[sessionId]?.text ?? "");
  const setDraft = useSessionStore(state => state.setDraft);
  const clearDraft = useSessionStore(state => state.clearDraft);
  const composerText = useAuiState(state => state.composer.text);
  const isRunning = useAuiState(state => state.thread.isRunning);
  const composerInputElementRef = useRef<HTMLTextAreaElement>(null);
  const hasHydratedDraftRef = useRef(false);

  const setComposerInputElement: RefCallback<HTMLTextAreaElement> = element => {
    composerInputElementRef.current = element;
  };

  const clearDraftForSession = () => clearDraft(sessionId);

  const clearComposer = () => {
    aui.composer().setText("");
    clearDraft(sessionId);
  };

  const setComposerText = (text: string) => {
    aui.composer().setText(text);
    setDraft(sessionId, { text });
  };

  const prefillComposer = (text: string) => {
    const currentText = aui.composer().getState().text;
    if (currentText.trim().length > 0 && currentText !== text) {
      toast.warning("Send or discard the current draft before prefilling a Goal command.");
      composerInputElementRef.current?.focus();
      return;
    }
    setComposerText(text);
    composerInputElementRef.current?.focus();
  };

  useEffect(() => {
    if (hasHydratedDraftRef.current) {
      return;
    }

    hasHydratedDraftRef.current = true;

    if (!draftText) {
      return;
    }

    aui.composer().setText(draftText);
  }, [aui, draftText]);

  useEffect(() => {
    setDraft(sessionId, { text: composerText });
  }, [composerText, sessionId, setDraft]);

  useAuiEvent("composer.send", clearDraftForSession);

  return {
    clearComposer,
    composerText,
    isRunning,
    prefillComposer,
    setComposerInputElement,
    setComposerText,
  };
}
