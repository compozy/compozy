import { useEffect, type ReactNode } from "react";

import { onAccountScopeEvicted } from "@/lib/computer-scope";

import type { SessionCreateStore } from "../stores/session-create-store";
import { SessionCreateContext } from "./session-create-context-value";

interface SessionCreateProviderProps {
  store: SessionCreateStore;
  children: ReactNode;
}

/** Supplies only the per-shell store handle; consumers select their own slices. */
export function SessionCreateProvider({ store, children }: SessionCreateProviderProps) {
  useEffect(() => onAccountScopeEvicted(() => store.trigger.accountScopeEvicted({})), [store]);
  return <SessionCreateContext.Provider value={store}>{children}</SessionCreateContext.Provider>;
}
