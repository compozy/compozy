"use client";

import { Activity } from "react";
import { BlockLoading, type TerminalEngineLoader } from "@compozy/ui";

import type { TerminalAttachmentSocketFactory } from "../hooks/use-terminal-attachment";
import { useCompactTerminalWindow } from "../hooks/use-compact-terminal-window";
import { useTerminalWindowAppState } from "../hooks/use-terminal-window-app-state";
import { terminalInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalInfo, TerminalInputRequest, TerminalResolvedInputRequest } from "../types";
import {
  TerminalEmptyState,
  TerminalExecuteOnlyState,
  TerminalNotFoundState,
} from "./terminal-empty-states";
import { TerminalJournalHostChrome, type TerminalRecordingState } from "./terminal-header";
import { TerminalJournalHead } from "./terminal-journal-panel";
import { TerminalLimitDialog } from "./terminal-limit-dialog";
import type { TerminalWindowActions } from "./terminal-window-actions";
import { TerminalWindowBody } from "./terminal-window-body";

export type { TerminalWindowActions } from "./terminal-window-actions";

const EMPTY_RESOLVED: readonly TerminalResolvedInputRequest[] = [];

export interface TerminalWindowAppProps {
  workspaceId: string;
  /** The project's display name, for the journal's identity row. */
  projectLabel?: string;
  profile: string;
  /** This browser's operator identity, exactly as the daemon names it. */
  viewerId: string | null;
  /** Token proving this browser owns `viewerId`. */
  viewerToken?: string | null;
  terminals: readonly TerminalInfo[];
  inputRequests: readonly TerminalInputRequest[];
  resolvedInputRequests?: readonly TerminalResolvedInputRequest[];
  /** Terminal id → title, for stacked questions that must name their origin. */
  inputRequestTitles?: ReadonlyMap<string, string>;
  /** The per-project cap, from `[terminal].max_per_workspace`. Absent until loaded. */
  limit?: number;
  /** `[terminal].exit_retention` in milliseconds. Omit when unknown. */
  exitRetentionMs?: number;
  /** `[terminal].detached_ttl`, already phrased when known. */
  detachedTtl?: string;
  /** False where the platform cannot host an interactive terminal at all. */
  interactiveAvailable: boolean;
  /** Aggregate profile reads cannot mutate terminals owned by another profile. */
  readOnly?: boolean;
  recordings?: Readonly<Record<string, TerminalRecordingState>>;
  /** Pipe terminals render captured output rather than a live stream. */
  pipeOutput?: Readonly<Record<string, { lines: readonly string[]; firstLineNumber: number }>>;
  journal: React.ReactNode;
  /** The PTY the host route named. Isolated windows omit this. */
  requestedTerminalId?: string | null;
  /** Terminal ids other OS windows already show; the resolver never adopts them. */
  windowedTerminalIds?: ReadonlySet<string>;
  /** The route arrived asking for a fresh terminal; the resolver never adopts. */
  createIntent?: boolean;
  /** The catalog is fetched for this mount; the resolver waits for it. */
  resolveReady?: boolean;
  /** Retargets the host to `/terminal/:id`. Isolated windows omit this. */
  onSelectTerminal?: (terminalId: string) => void;
  /** Reveals the journal. The host unlocks its fetch on first open. */
  onViewJournal?: () => void;
  /** The journal overlay is no longer the surface being read. */
  onLeaveJournal?: () => void;
  actions: TerminalWindowActions;
  socketFactory?: TerminalAttachmentSocketFactory;
  /** Replaces the emulator. Tests and playback harnesses only. */
  engineLoader?: TerminalEngineLoader;
  /**
   * The OS window already hosts identity in its topbar. Isolated tests leave
   * this off so the in-app head remains the assertion surface.
   */
  hostChrome?: boolean;
}

/**
 * The Terminal app's window body: one terminal per OS window.
 *
 * The OS window deck is the only tab strip; this component owns the terminal
 * the route names, the journal overlay on top of it, and the edge states. The
 * journal stays mounted while hidden so its scroll and selection survive.
 */
export function TerminalWindowApp({
  workspaceId,
  projectLabel,
  profile,
  viewerId,
  viewerToken,
  terminals,
  inputRequests,
  resolvedInputRequests = EMPTY_RESOLVED,
  inputRequestTitles,
  limit,
  exitRetentionMs,
  detachedTtl,
  interactiveAvailable,
  readOnly = false,
  recordings,
  pipeOutput,
  journal,
  requestedTerminalId,
  windowedTerminalIds,
  createIntent,
  resolveReady,
  onSelectTerminal,
  onViewJournal,
  onLeaveJournal,
  actions,
  socketFactory,
  engineLoader,
  hostChrome = false,
}: TerminalWindowAppProps) {
  const { rootRef, compact } = useCompactTerminalWindow();
  const {
    active,
    activeProfile,
    journalOpen,
    openJournal,
    closeJournal,
    destinationTerminals,
    limitOpen,
    missingRequested,
    resolving,
    openTerminal,
    setLimitOpen,
  } = useTerminalWindowAppState({
    workspaceId,
    profile,
    terminals,
    limit,
    readOnly,
    interactiveAvailable,
    actions,
    requestedTerminalId,
    windowedTerminalIds,
    createIntent,
    resolveReady,
    onSelectTerminal,
    onViewJournal,
    onLeaveJournal,
  });

  return (
    <div
      className="flex min-h-0 flex-1 flex-col bg-terminal-bg"
      data-testid="terminal-window"
      ref={rootRef}
    >
      <Activity mode={journalOpen ? "visible" : "hidden"}>
        <div className="flex min-h-0 flex-1 flex-col">
          <TerminalJournalHostChrome
            hostChrome={hostChrome && journalOpen}
            onBack={closeJournal}
            projectLabel={projectLabel}
          />
          {hostChrome ? null : (
            <TerminalJournalHead onBack={closeJournal} projectLabel={projectLabel} />
          )}
          {journal}
        </div>
      </Activity>
      {journalOpen ? null : missingRequested ? (
        <TerminalNotFoundState onOpenTerminal={openTerminal} onViewJournal={openJournal} />
      ) : !interactiveAvailable && active === null ? (
        <TerminalExecuteOnlyState onViewJournal={openJournal} />
      ) : active === null && resolving ? (
        <BlockLoading className="flex-1" label="Opening a terminal" surface="bare" />
      ) : active === null ? (
        <TerminalEmptyState onOpenTerminal={openTerminal} />
      ) : (
        <TerminalWindowBody
          actions={{ ...actions, onOpenTerminal: openTerminal }}
          compact={compact}
          detachedTtl={detachedTtl}
          engineLoader={engineLoader}
          exitRetentionMs={exitRetentionMs}
          hostChrome={hostChrome}
          inputRequestTitles={inputRequestTitles}
          inputRequests={inputRequests.filter(request => request.terminal_id === active.id)}
          resolvedInputRequests={resolvedInputRequests.filter(
            request => request.terminal_id === active.id
          )}
          // Keyed by the full scoped identity: the same terminal id under a
          // different profile is a different terminal, and must not inherit
          // the previous one's in-flight confirmation.
          key={terminalInstanceKey(workspaceId, activeProfile, active.id)}
          onViewJournal={openJournal}
          pipeOutput={pipeOutput?.[active.id]}
          profile={activeProfile}
          readOnly={readOnly}
          recording={recordings?.[active.id] ?? null}
          socketFactory={socketFactory}
          terminal={active}
          terminalCount={destinationTerminals.length}
          limit={limit}
          viewerId={viewerId}
          viewerToken={viewerToken}
          workspaceId={workspaceId}
        />
      )}
      {!readOnly && limit !== undefined ? (
        <TerminalLimitDialog
          limit={limit}
          onCloseTerminal={actions.onCloseTerminal}
          onOpenChange={setLimitOpen}
          onOpenSettings={actions.onOpenSettings}
          open={limitOpen}
          terminals={destinationTerminals}
        />
      ) : null}
    </div>
  );
}
