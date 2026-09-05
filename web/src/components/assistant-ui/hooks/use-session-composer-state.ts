import { useEffect, useLayoutEffect, useRef } from "react";
import { useAui, useAuiEvent, useAuiState } from "@assistant-ui/react";
import { useStore } from "@xstate/store-react";
import { toast } from "sonner";

import type { SessionComposerInputHandle } from "../session-composer-lexical-plugins";
import { sessionComposerSyncLogic } from "./session-composer-sync-store";
import {
  retainSubmittedComposerAttachments,
  sessionStore,
  useSessionComposerDraft,
} from "@/systems/session";

/** What one busy send took from the composer, captured at submission time. */
export interface SessionComposerSubmission {
  /** Raw composer text (not the wire message) when the send was requested. */
  composerText: string;
  /** Composer attachment ids that rode along with the send. */
  attachmentIds: readonly string[];
}

export interface SessionComposerState {
  clearComposer: (options?: { retainAttachments?: boolean }) => void;
  /**
   * Removes exactly what an accepted busy send took, and returns the text left
   * in the field. The editor stays writable while a send is in flight, so
   * anything typed or attached after submission belongs to the operator.
   */
  consumeSubmittedDraft: (submission: SessionComposerSubmission) => string;
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

  const consumeSubmittedDraft = ({
    composerText: submittedText,
    attachmentIds,
  }: SessionComposerSubmission) => {
    const state = aui.composer.getState();
    const submittedIds = new Set(attachmentIds);
    const submittedAttachments = state.attachments.filter(attachment =>
      submittedIds.has(attachment.id)
    );
    retainSubmittedComposerAttachments(submittedAttachments);
    for (const attachment of submittedAttachments) {
      void aui.composer.attachment({ id: attachment.id }).remove();
    }
    const currentText = state.text;
    if (!currentText.startsWith(submittedText)) {
      // The operator rewrote the field while the send was in flight: it is theirs.
      return currentText;
    }
    // Exact remainder: whatever the operator authored after the sent prefix,
    // leading whitespace and indentation included (an attachment-only send has
    // an empty prefix, so the whole field is theirs).
    const remainder = currentText.slice(submittedText.length);
    if (remainder.length === 0) {
      sessionStore.trigger.composerDraftDiscarded({ sessionId });
      aui.composer.setText("");
      composerInputHandleRef.current?.clear();
      return "";
    }
    sessionStore.trigger.composerDraftChanged({ sessionId, text: remainder });
    aui.composer.setText(remainder);
    return remainder;
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
  }, [composerText, draftText, sessionId, syncStore]);

  useAuiEvent("composer.send", () => {
    clearDraftForSession();
  });

  return {
    clearComposer,
    composerText,
    consumeSubmittedDraft,
    isRunning,
    persistComposerText,
    prefillComposer,
    setComposerInputElement,
    setComposerText,
  };
}
