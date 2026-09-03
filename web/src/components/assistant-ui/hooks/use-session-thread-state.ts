import { useAui } from "@assistant-ui/react";

import {
  useSessionFirstPrompt,
  type SessionFailurePayload,
  type SessionState,
} from "@/systems/session";

import { useSessionComposerState } from "./use-session-composer-state";
import { usePrefersReducedMotion } from "./use-prefers-reduced-motion";
import { useSessionPromptDispatch } from "./use-session-prompt-dispatch";

interface UseSessionThreadStateOptions {
  acpSessionId?: string;
  canPrompt: boolean;
  isSessionRunning: boolean;
  onCancelPrompt?: () => void;
  readOnly: boolean;
  sessionFailure?: SessionFailurePayload | null;
  sessionId: string;
  sessionState?: SessionState;
}

export function useSessionThreadState({
  acpSessionId,
  canPrompt,
  isSessionRunning,
  onCancelPrompt,
  readOnly,
  sessionFailure,
  sessionId,
  sessionState,
}: UseSessionThreadStateOptions) {
  const aui = useAui();
  const reducedMotion = usePrefersReducedMotion();
  const composer = useSessionComposerState(sessionId);
  const promptDispatch = useSessionPromptDispatch();
  const renderedComposer = promptDispatch.canceled ? { ...composer, isRunning: false } : composer;
  const runtimeRunning = isSessionRunning || renderedComposer.isRunning || promptDispatch.pending;
  const startupFailed =
    sessionState === "stopped" && Boolean(sessionFailure) && !acpSessionId?.trim().length;
  const lifecycleCanPrompt =
    !readOnly && canPrompt && sessionState !== "starting" && !startupFailed;
  useSessionFirstPrompt({ canPrompt: lifecycleCanPrompt, sessionId });

  return {
    composer,
    handleCancelPrompt: () => {
      aui.thread.cancelRun();
      promptDispatch.cancelPending();
      onCancelPrompt?.();
    },
    lifecycleCanPrompt,
    reducedMotion,
    renderedComposer,
    runtimeRunning,
    startupFailed,
  };
}
