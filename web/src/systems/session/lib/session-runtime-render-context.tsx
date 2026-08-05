import type { ReactNode } from "react";

import {
  SessionRuntimeRenderContext,
  type SessionRuntimeRenderContextValue,
} from "./session-runtime-render-context-value";

const noop = () => {};
const noDurableMessageIds: ReadonlySet<string> = new Set();

export function SessionRuntimeRenderProvider({
  children,
  durableMessageIds = noDurableMessageIds,
  resetRuntime = noop,
  rewindBlocked = false,
  sessionId,
  workspaceId,
}: Partial<Pick<SessionRuntimeRenderContextValue, "durableMessageIds">> &
  Omit<SessionRuntimeRenderContextValue, "durableMessageIds"> & { children: ReactNode }) {
  const contextValue = { durableMessageIds, resetRuntime, rewindBlocked, sessionId, workspaceId };
  return <SessionRuntimeRenderContext value={contextValue}>{children}</SessionRuntimeRenderContext>;
}
