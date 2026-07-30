import { createContext } from "react";

import type { ReasoningEffort, RuntimeSpeed } from "@/lib/api-contract";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime";

/** Runtime intent captured when a user dispatches one session prompt. */
export interface SessionPromptRuntimeSnapshot {
  provider: string;
  model?: string;
  reasoning_effort?: ReasoningEffort;
  speed?: RuntimeSpeed;
}

export interface SessionPromptRuntimeContextValue {
  catalog: {
    error: string | null;
    loaded: boolean;
    loading: boolean;
    models: RuntimeModelOption[];
    providers: RuntimeProviderOption[];
    refresh: () => void;
    refreshError: string | null;
    refreshing: boolean;
    stale: boolean;
  };
  canSelectRuntime: boolean;
  getRuntimeSnapshot: () => SessionPromptRuntimeSnapshot | null;
  onRuntimeChange: (next: RuntimeSelectorValue) => void;
  onSpeedChange: (next: RuntimeSpeed) => void;
  speed: RuntimeSpeed;
  value: RuntimeSelectorValue;
}

export const SessionPromptRuntimeContext = createContext<SessionPromptRuntimeContextValue | null>(
  null
);
