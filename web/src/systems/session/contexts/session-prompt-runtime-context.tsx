import type { ReactNode } from "react";

import type { SessionPayload } from "../types";
import { useSessionPromptRuntime } from "../hooks/use-session-prompt-runtime";
import { SessionPromptRuntimeContext } from "./session-prompt-runtime-context-value";

export interface SessionPromptRuntimeProviderProps {
  session: SessionPayload;
  canPrompt: boolean;
  children: ReactNode;
}

export function SessionPromptRuntimeProvider({
  session,
  canPrompt,
  children,
}: SessionPromptRuntimeProviderProps) {
  const value = useSessionPromptRuntime(session, canPrompt);
  return <SessionPromptRuntimeContext value={value}>{children}</SessionPromptRuntimeContext>;
}
