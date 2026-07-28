import type { ReactNode } from "react";

import {
  SessionRuntimeRenderContext,
  type SessionRuntimeRenderContextValue,
} from "./session-runtime-render-context-value";

export function SessionRuntimeRenderProvider({
  children,
  sessionId,
  workspaceId,
}: SessionRuntimeRenderContextValue & { children: ReactNode }) {
  const contextValue = { sessionId, workspaceId };
  return <SessionRuntimeRenderContext value={contextValue}>{children}</SessionRuntimeRenderContext>;
}
