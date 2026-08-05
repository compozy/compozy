import { createContext } from "react";

export interface SessionRuntimeRenderContextValue {
  resetRuntime?: () => void;
  rewindBlocked?: boolean;
  sessionId: string;
  workspaceId: string;
}

export const SessionRuntimeRenderContext = createContext<SessionRuntimeRenderContextValue | null>(
  null
);
