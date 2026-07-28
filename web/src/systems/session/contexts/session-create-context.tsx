import type { ReactNode } from "react";

import type { SessionCreateStore } from "../stores/session-create-store";
import { SessionCreateContext } from "./session-create-context-value";

interface SessionCreateProviderProps {
  store: SessionCreateStore;
  children: ReactNode;
}

/** Supplies only the per-shell store handle; consumers select their own slices. */
export function SessionCreateProvider({ store, children }: SessionCreateProviderProps) {
  return <SessionCreateContext.Provider value={store}>{children}</SessionCreateContext.Provider>;
}
