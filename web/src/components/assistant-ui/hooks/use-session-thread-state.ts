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
    // Cancel settles the live transport by aborting the tracked prompt fetch:
    // the AI SDK reads the abort as a clean cancel (status ready, no error), so
    // no runtime remount is needed. `thread.cancelRun()` is deliberately not
    // used — assistant-ui treats a cancelled run as an unsent message, deleting
    // the trailing user message and re-drafting its text, while the daemon
    // already accepted and recorded that prompt (US-009.EC-4, US-019.EC-1).
    handleCancelPrompt: () => {
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
