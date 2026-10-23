import { useAuiState } from "@assistant-ui/react";

import { useSteerProvenance } from "@/systems/session/hooks/use-steer-provenance";

import {
  assistantMessagePlacement,
  type AssistantMessagePlacement,
} from "../session-assistant-message.logic";

/**
 * Whether the message draws, what it draws, and how it sits in its turn: a
 * steer marker whose row draws nothing mounts no shell, and one turn split
 * across messages (a steer delivered mid-turn) joins back into one run.
 */
export function useAssistantMessagePlacement(): AssistantMessagePlacement {
  const provenance = useSteerProvenance();
  return useAuiState(state =>
    assistantMessagePlacement(state.thread.messages, state.message.id, provenance)
  );
}
