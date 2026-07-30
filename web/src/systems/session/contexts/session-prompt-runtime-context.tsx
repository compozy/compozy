import { useEffect, type ReactNode } from "react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import type { SessionPayload } from "../types";
import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
} from "../stores/session-prompt-runtime-store";
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
  const input = sessionPromptRuntimeInput(session, canPrompt);
  const { store } = useStoreBinding(session.id, () =>
    sessionPromptRuntimeStoreLogic.createStore(input)
  );

  useEffect(() => {
    store.trigger.inputUpdated(input);
  }, [input, store]);

  return <SessionPromptRuntimeContext value={store}>{children}</SessionPromptRuntimeContext>;
}
