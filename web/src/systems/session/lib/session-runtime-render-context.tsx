import type { ReactNode } from "react";

import {
  SessionRuntimeRenderContext,
  type SessionRuntimeRenderContextValue,
} from "./session-runtime-render-context-value";
import type { SessionInteractionRecord } from "../types";

const noop = () => {};
const noDurableMessageIds: ReadonlySet<string> = new Set();
const noExpiredInteractions: ReadonlyMap<string, SessionInteractionRecord> = new Map();
const noResolvedInteractions: ReadonlyMap<string, SessionInteractionRecord> = new Map();

export function SessionRuntimeRenderProvider({
  children,
  durableMessageIds = noDurableMessageIds,
  expiredInteractions = noExpiredInteractions,
  resolvedInteractions = noResolvedInteractions,
  resetRuntime = noop,
  rewindBlocked = false,
  sessionId,
  workspaceId,
}: Partial<
  Pick<
    SessionRuntimeRenderContextValue,
    "durableMessageIds" | "expiredInteractions" | "resolvedInteractions"
  >
> &
  Omit<
    SessionRuntimeRenderContextValue,
    "durableMessageIds" | "expiredInteractions" | "resolvedInteractions"
  > & {
    children: ReactNode;
  }) {
  const contextValue = {
    durableMessageIds,
    expiredInteractions,
    resolvedInteractions,
    resetRuntime,
    rewindBlocked,
    sessionId,
    workspaceId,
  };
  return <SessionRuntimeRenderContext value={contextValue}>{children}</SessionRuntimeRenderContext>;
}
