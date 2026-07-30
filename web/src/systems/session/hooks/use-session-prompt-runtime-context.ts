import { use } from "react";

import { SessionPromptRuntimeContext } from "../contexts/session-prompt-runtime-context-value";

export function useSessionPromptRuntimeContext() {
  const context = useOptionalSessionPromptRuntimeContext();
  if (context === null) {
    throw new Error("useSessionPromptRuntimeContext requires SessionPromptRuntimeProvider");
  }
  return context;
}

export function useOptionalSessionPromptRuntimeContext() {
  return use(SessionPromptRuntimeContext);
}
