import type { ReactNode } from "react";

import {
  SessionCreateContext,
  type SessionCreateContextValue,
} from "./session-create-context-value";

interface SessionCreateProviderProps {
  value: SessionCreateContextValue;
  children: ReactNode;
}

export function SessionCreateProvider({ value, children }: SessionCreateProviderProps) {
  return <SessionCreateContext.Provider value={value}>{children}</SessionCreateContext.Provider>;
}

export type { SessionCreateContextValue };
