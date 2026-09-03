"use client";

import { useEffect, useEffectEvent, useRef, useState } from "react";

import type { TerminalWindowActions } from "../lib/terminal-window-actions";
import type { TerminalInfo } from "../types";
import { useTerminalScopeCleanup } from "./use-terminal-scope-cleanup";
import { useTerminalStore } from "./use-terminal-store";

const EMPTY_WINDOWED: ReadonlySet<string> = new Set();

function hasTerminal(terminals: readonly TerminalInfo[], id: string): boolean {
  return terminals.some(terminal => terminal.id === id);
}

export interface UseTerminalWindowAppStateOptions {
  workspaceId: string;
  profile: string;
  terminals: readonly TerminalInfo[];
  /** The per-project cap, from `[terminal].max_per_workspace`. Absent until loaded. */
  limit?: number;
  readOnly: boolean;
  /** False where the platform cannot host an interactive terminal at all. */
  interactiveAvailable: boolean;
  actions: TerminalWindowActions;
  /**
   * The PTY the route named. `undefined` is an isolated window with no host
   * route. `null` is `/terminal` with no id — the resolver's territory.
   */
  requestedTerminalId?: string | null;
  /** Terminal ids already on screen in another OS window. Never adopted. */
  windowedTerminalIds?: ReadonlySet<string>;
  /** The route arrived asking for a fresh terminal; the resolver never adopts. */
  createIntent?: boolean;
  /**
   * The catalog has been fetched for this mount. The resolver must decide on
   * current truth — a cached list from before this window opened could miss a
   * running terminal and open a duplicate instead of adopting it.
   */
  resolveReady?: boolean;
  /** Reveals the journal. The host unlocks its fetch on first open. */
  onViewJournal?: (() => void) | undefined;
  /** The journal overlay is no longer the surface being read. */
  onLeaveJournal?: (() => void) | undefined;
}

export interface TerminalWindowAppState {
  active: TerminalInfo | null;
  activeProfile: string;
  journalOpen: boolean;
  openJournal: () => void;
  closeJournal: () => void;
  /** Terminals that count toward the cap: running, under this profile. */
  destinationTerminals: TerminalInfo[];
  limitOpen: boolean;
  /** The route named a PTY that is not in this catalog. */
  missingRequested: boolean;
  /** The id-less route is still deciding which terminal this window shows. */
  resolving: boolean;
  openTerminal: (() => void) | undefined;
  setLimitOpen: (open: boolean) => void;
}

/**
 * One OS window, one terminal.
 *
 * The route is the only selection truth: `/terminal/:id` names the PTY this
 * window shows, and the journal is a local overlay on top of it. The id-less
 * `/terminal` route resolves itself — it adopts the most recent running
 * terminal no other window is showing, or opens a fresh one — so launching the
 * app always lands in a working terminal rather than a picker.
 */
export function useTerminalWindowAppState({
  workspaceId,
  profile,
  terminals,
  limit,
  readOnly,
  interactiveAvailable,
  actions,
  requestedTerminalId,
  windowedTerminalIds = EMPTY_WINDOWED,
  createIntent = false,
  resolveReady = true,
  onViewJournal,
  onLeaveJournal,
}: UseTerminalWindowAppStateOptions): TerminalWindowAppState {
  const [journalOpen, setJournalOpen] = useState(false);
  const [limitOpen, setLimitOpen] = useState(false);
  // Interaction state belongs to one `(workspace, profile)`: a dialog opened
  // for one project must not survive into another. Compared during render so
  // there is never a frame where the stale dialog shows over the new scope.
  const scope = `${workspaceId} ${profile}`;
  const [previousScope, setPreviousScope] = useState(scope);
  if (previousScope !== scope) {
    setPreviousScope(scope);
    setLimitOpen(false);
    setJournalOpen(false);
  }
  const routeOwned = requestedTerminalId !== undefined;
  const missingRequested =
    routeOwned &&
    requestedTerminalId !== null &&
    !hasTerminal(terminals, requestedTerminalId) &&
    !journalOpen;
  const active = resolveActiveTerminal({ requestedTerminalId, routeOwned, terminals });
  // Exited terminals stay readable through retention but no longer occupy a
  // slot; only running ones count toward the cap.
  const destinationTerminals = terminals.filter(
    terminal => terminal.profile_name === profile && terminal.state === "running"
  );
  const atLimit = limit !== undefined && destinationTerminals.length >= limit;
  const store = useTerminalStore();
  const activeProfile = active?.profile_name ?? profile;
  useTerminalScopeCleanup({ workspaceId, profile: activeProfile, terminals, store });

  // The daemon refuses a create at the cap, so the offer becomes the way out.
  const openTerminal =
    actions.onOpenTerminal && !readOnly
      ? () => {
          if (atLimit) {
            setLimitOpen(true);
            return;
          }
          actions.onOpenTerminal?.();
        }
      : undefined;

  const resolving = routeOwned && requestedTerminalId === null && !readOnly;
  // One resolution per arrival at the id-less route in a scope: re-renders and
  // StrictMode's doubled effect land on the same key and are ignored, while
  // navigating away and back produces a new arrival.
  const resolutionKey = `${scope}::${requestedTerminalId ?? "auto"}`;
  const lastResolutionKeyRef = useRef<string | null>(null);
  const resolveDestination = useEffectEvent(() => {
    const adoptable = createIntent
      ? null
      : latestUnwindowedRunning(terminals, profile, windowedTerminalIds);
    if (adoptable && actions.retargetTerminal) {
      actions.retargetTerminal(adoptable.id);
      return;
    }
    // Execute-only platforms cannot host a PTY; the state surface says so.
    if (!interactiveAvailable) return;
    openTerminal?.();
  });
  useEffect(() => {
    // Adopting or creating a terminal because the window landed on `/terminal`
    // is a synchronization with the daemon, keyed to that arrival — exactly
    // what an effect is for.
    if (!resolving) {
      lastResolutionKeyRef.current = resolutionKey;
      return;
    }
    // An arrival is consumed only once it can be answered from fresh truth.
    if (!resolveReady) return;
    const previousKey = lastResolutionKeyRef.current;
    lastResolutionKeyRef.current = resolutionKey;
    if (previousKey === resolutionKey) return;
    resolveDestination();
  }, [resolving, resolveReady, resolutionKey]);

  return {
    active,
    activeProfile,
    journalOpen,
    openJournal: () => {
      setJournalOpen(true);
      onViewJournal?.();
    },
    closeJournal: () => {
      setJournalOpen(false);
      onLeaveJournal?.();
    },
    destinationTerminals,
    limitOpen,
    missingRequested,
    resolving,
    openTerminal,
    setLimitOpen,
  };
}

function resolveActiveTerminal({
  requestedTerminalId,
  routeOwned,
  terminals,
}: {
  requestedTerminalId: string | null | undefined;
  routeOwned: boolean;
  terminals: readonly TerminalInfo[];
}): TerminalInfo | null {
  if (routeOwned) {
    if (typeof requestedTerminalId !== "string") return null;
    return terminals.find(terminal => terminal.id === requestedTerminalId) ?? null;
  }
  // An isolated window has no route; it shows what it was given.
  return terminals[0] ?? null;
}

/** The catalog arrives oldest-first, so the last match is the most recent. */
function latestUnwindowedRunning(
  terminals: readonly TerminalInfo[],
  profile: string,
  windowedTerminalIds: ReadonlySet<string>
): TerminalInfo | null {
  for (let index = terminals.length - 1; index >= 0; index -= 1) {
    const terminal = terminals[index];
    if (
      terminal.state === "running" &&
      terminal.profile_name === profile &&
      !windowedTerminalIds.has(terminal.id)
    ) {
      return terminal;
    }
  }
  return null;
}
