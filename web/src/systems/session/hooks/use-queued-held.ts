import { useAgentInstances } from "@/systems/agent";

import { queuedPromptHeld, type QueuedPrompt } from "../lib/queued-prompt";
import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";

/** Whether this row waits for the viewing stays-on agent to resume (S12). */
export function useQueuedHeld(prompt: QueuedPrompt): boolean {
  const sessionId = useSessionRuntimeRenderContext()?.sessionId ?? "";
  const paused = useAgentInstances().some(
    instance =>
      sessionId !== "" && instance.main_session_id === sessionId && instance.state === "paused"
  );
  return queuedPromptHeld(prompt, paused);
}
