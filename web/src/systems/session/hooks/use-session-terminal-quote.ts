"use client";

import { useSelector } from "@xstate/store-react";

import type { TerminalQuote } from "@/systems/terminal/parts";

import { sessionTerminalQuoteStore } from "../lib/session-terminal-quote";

/** The terminal excerpt staged for one session's composer, or null. */
export function useSessionTerminalQuote(sessionId: string): TerminalQuote | null {
  return useSelector(
    sessionTerminalQuoteStore,
    snapshot => snapshot.context.quotes[sessionId] ?? null
  );
}
