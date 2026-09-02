import { createContext, use } from "react";

import type { useTerminalWindowControllerState } from "./use-terminal-window-controller-state";

export type TerminalWindowControllerState = ReturnType<typeof useTerminalWindowControllerState>;

export const TerminalWindowControllerContext = createContext<TerminalWindowControllerState | null>(
  null
);

export function useTerminalWindowControllerContext(): TerminalWindowControllerState {
  const state = use(TerminalWindowControllerContext);
  if (state === null) throw new Error("Terminal window controller is unavailable");
  return state;
}
