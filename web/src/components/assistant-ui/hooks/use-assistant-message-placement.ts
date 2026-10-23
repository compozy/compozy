import { useAuiState } from "@assistant-ui/react";

import { useSteerProvenance } from "@/systems/session/hooks/use-steer-provenance";
import { assistantMessageHasRenderableContent } from "@/systems/session/lib/session-thinking-state";

import {
  assistantMessageRenders,
  assistantTurnContinuity,
  visibleAssistantContent,
} from "../session-assistant-message.logic";
import { queueClearTraceCount } from "../session-queue-trace.logic";

/**
 * Whether the message draws, what it draws, and how it sits in its turn: a
 * steer marker whose row draws nothing mounts no shell, and one turn split
 * across messages (a steer delivered mid-turn) joins back into one run.
 */
export function useAssistantMessagePlacement() {
  const provenance = useSteerProvenance();
  const traceCount = useAuiState(state =>
    queueClearTraceCount(state.thread.messages, state.message.id)
  );
  const renders = useAuiState(state =>
    assistantMessageRenders(state.thread.messages, state.message, provenance)
  );
  const hasRenderableContent = useAuiState(state =>
    assistantMessageHasRenderableContent(visibleAssistantContent(state.message.content, provenance))
  );
  const joinsPrevious = useAuiState(
    state =>
      assistantTurnContinuity(state.thread.messages, state.message.id, provenance).joinsPrevious
  );
  const joinsNext = useAuiState(
    state => assistantTurnContinuity(state.thread.messages, state.message.id, provenance).joinsNext
  );
  return { traceCount, renders, hasRenderableContent, joinsPrevious, joinsNext };
}
