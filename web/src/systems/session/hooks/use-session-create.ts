import { use } from "react";

import {
  SessionCreateContext,
  type SessionCreateContextValue,
} from "../contexts/session-create-context-value";

export function useSessionCreate(): SessionCreateContextValue {
  const value = use(SessionCreateContext);
  if (!value) {
    throw new Error("useSessionCreate must be used inside <SessionCreateProvider>");
  }
  return value;
}
