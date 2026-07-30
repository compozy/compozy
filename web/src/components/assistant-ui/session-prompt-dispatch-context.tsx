import { type ReactNode } from "react";

import {
  SessionPromptDispatchContext,
  type SessionPromptDispatchStore,
} from "./session-prompt-dispatch-store";

export function SessionPromptDispatchPendingProvider({
  children,
  store,
}: {
  children: ReactNode;
  store: SessionPromptDispatchStore;
}) {
  return <SessionPromptDispatchContext value={store}>{children}</SessionPromptDispatchContext>;
}
