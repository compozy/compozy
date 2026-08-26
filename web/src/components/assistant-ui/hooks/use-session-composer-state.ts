import { useEffect, useLayoutEffect, useRef } from "react";
import { useAui, useAuiEvent, useAuiState } from "@assistant-ui/react";
import { useStore } from "@xstate/store-react";
import { toast } from "sonner";

import type { SessionComposerInputHandle } from "../session-composer-lexical-plugins";
import { sessionComposerSyncLogic } from "./session-composer-sync-store";
import {
  retainSubmittedComposerAttachments,
  discardSessionTerminalQuote,
  discardSessionTerminalQuoteIfMissing,
  sessionStore,
  useSessionComposerDraft,
} from "@/systems/session";

export interface SessionComposerState {
  clearComposer: (options?: { retainAttachments?: boolean }) => void;
  persistComposerText: (text: string) => void;
  setComposerText: (text: string) => void;
  setComposerInputElement: (handle: SessionComposerInputHandle | null) => void;
  prefillComposer: (text: string) => void;
  composerText: string;
  isRunning: boolean;
}

export function useSessionComposerState(sessionId: string): SessionComposerState {
  const aui = useAui();
  const draftText = useSessionComposerDraft(sessionId);
  const composerText = useAuiState(state => state.composer.text);
  const isRunning = useAuiState(state => state.thread.isRunning);
  const syncStore = useStore(sessionComposerSyncLogic);
  const composerInputHandleRef = useRef<SessionComposerInputHandle | null>(null);

  const setComposerInputElement = (handle: SessionComposerInputHandle | null) => {
    composerInputHandleRef.current = handle;
  };

  const clearDraftForSession = () => sessionStore.trigger.composerDraftDiscarded({ sessionId });

  const clearComposer = ({ retainAttachments = false }: { retainAttachments?: boolean } = {}) => {
    if (retainAttachments) {
      retainSubmittedComposerAttachments(aui.composer.getState().attachments);
    }
    sessionStore.trigger.composerDraftDiscarded({ sessionId });
    aui.composer.setText("");
    composerInputHandleRef.current?.clear();
    void aui.composer.clearAttachments();
  };

  const persistComposerText = (text: string) => {
    sessionStore.trigger.composerDraftChanged({ sessionId, text });
  };

  const setComposerText = (text: string) => {
    persistComposerText(text);
  };

  const prefillComposer = (text: string) => {
    const currentText = aui.composer.getState().text;
    if (currentText.trim().length > 0 && currentText !== text) {
      toast.warning("Send or discard the current draft before prefilling a Goal command.");
      composerInputHandleRef.current?.focus();
      return;
    }
    setComposerText(text);
    composerInputHandleRef.current?.focus();
  };

  useLayoutEffect(() => {
    syncStore.trigger.draftObserved({
      composerText: aui.composer.getState().text,
      draftText,
      hydrate: text => aui.composer.setText(text),
      sessionId,
    });
  }, [aui, draftText, sessionId, syncStore]);

  useEffect(() => {
    syncStore.trigger.composerObserved({
      composerText,
      draftText,
      persist: text => sessionStore.trigger.composerDraftChanged({ sessionId, text }),
    });
    discardSessionTerminalQuoteIfMissing(sessionId, composerText);
  }, [composerText, draftText, sessionId, syncStore]);

  useAuiEvent("composer.send", () => {
    clearDraftForSession();
    discardSessionTerminalQuote(sessionId);
  });

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
