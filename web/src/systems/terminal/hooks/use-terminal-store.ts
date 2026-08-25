"use client";

import { use } from "react";

import { TerminalStoreContext, type TerminalStore } from "../contexts/terminal-store-handle";

/**
 * The terminal store handle for the surrounding provider.
 *
 * Panes take the handle, never a snapshot, and subscribe to the slice they
 * render — a snapshot through context would re-render every pane on every frame
 * of every other terminal.
 */
export function useTerminalStore(): TerminalStore {
  const store = use(TerminalStoreContext);
  if (!store) {
    throw new Error("useTerminalStore must be used inside a TerminalStoreProvider.");
  }
  return store;
}
