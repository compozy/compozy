"use client";

import { useStore } from "@xstate/store-react";
import type { ReactNode } from "react";

import { terminalStoreLogic } from "../stores/terminal-store";
import { TerminalStoreContext } from "./terminal-store-handle";

/**
 * Injects the terminal store handle, never a snapshot.
 *
 * One store per Terminal app: panes come and go as tabs switch, and the control
 * state they read has to outlive any one of them. Panes subscribe to the slice
 * they render, so one terminal's output never re-renders another's pane.
 */
export function TerminalStoreProvider({ children }: { children: ReactNode }) {
  const store = useStore(terminalStoreLogic);
  return <TerminalStoreContext value={store}>{children}</TerminalStoreContext>;
}
