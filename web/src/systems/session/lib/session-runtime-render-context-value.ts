import { createContext } from "react";

export interface SessionRuntimeRenderContextValue {
  sessionId: string;
  workspaceId: string;
}

export const SessionRuntimeRenderContext = createContext<SessionRuntimeRenderContextValue | null>(
  null
);
