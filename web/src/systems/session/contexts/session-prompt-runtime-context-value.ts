import { createContext } from "react";

import type { ReasoningEffort, RuntimeSpeed } from "@/lib/api-contract";
import type { RuntimeACPOptionSelection } from "@/systems/runtime";
import type { SessionPromptRuntimeStore } from "../stores/session-prompt-runtime-store";
import type { SessionRuntimePayload } from "../types";

/** Runtime intent captured when a user dispatches one session prompt. */
export interface SessionPromptRuntimeSnapshot {
  provider: string;
  model?: string;
  reasoning_effort?: ReasoningEffort;
  speed?: RuntimeSpeed;
  acp_options?: RuntimeACPOptionSelection[];
}

export const SessionPromptRuntimeContext = createContext<SessionPromptRuntimeStore | null>(null);
/** Server-owned capabilities stay outside the prompt intent store. */
export const SessionPromptRuntimeCapabilitiesContext = createContext<
  SessionRuntimePayload | undefined
>(undefined);
