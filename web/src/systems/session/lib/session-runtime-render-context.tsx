import type { ReactNode } from "react";

import {
  SessionRuntimeRenderContext,
  type SessionRuntimeRenderContextValue,
} from "./session-runtime-render-context-value";

const noop = () => {};

export function SessionRuntimeRenderProvider({
  children,
  resetRuntime = noop,
  rewindBlocked = false,
  sessionId,
  workspaceId,
}: SessionRuntimeRenderContextValue & { children: ReactNode }) {
  const contextValue = { resetRuntime, rewindBlocked, sessionId, workspaceId };
  return <SessionRuntimeRenderContext value={contextValue}>{children}</SessionRuntimeRenderContext>;
}
