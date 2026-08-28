"use client";

import { Activity } from "react";
import type { TerminalEngineLoader } from "@compozy/ui";

import type { TerminalAttachmentSocketFactory } from "../hooks/use-terminal-attachment";
import { useCompactTerminalWindow } from "../hooks/use-compact-terminal-window";
import {
  TERMINAL_NO_TERMINALS,
  useTerminalWindowAppState,
} from "../hooks/use-terminal-window-app-state";
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
import { TERMINAL_JOURNAL_TAB, terminalPanelDomId, terminalTabDomId } from "./terminal-tab-id";
import { TerminalTabs } from "./terminal-tabs";
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
  /** The per-project cap, from `[terminal].max_per_workspace`. */
  limit: number;
  /** `[terminal].exit_retention` in milliseconds. Omit when unknown. */
  exitRetentionMs?: number;
  /** `[terminal].detached_ttl`, already phrased when known. */
  detachedTtl?: string;
  /** False where the platform cannot host an interactive terminal at all. */
  interactiveAvailable: boolean;
  /** Aggregate profile reads cannot mutate terminals owned by another profile. */
  readOnly?: boolean;
  auditBlockedIds?: ReadonlySet<string>;
  recordings?: Readonly<Record<string, TerminalRecordingState>>;
  /** Pipe terminals render captured output rather than a live stream. */
  pipeOutput?: Readonly<Record<string, { lines: readonly string[]; firstLineNumber: number }>>;
  journal: React.ReactNode;
  /** The PTY the host route named. Isolated windows omit this. */
  requestedTerminalId?: string | null;
  /** Retargets the host to `/terminal/:id`. Isolated windows omit this. */
  onSelectTerminal?: (terminalId: string) => void;
  /** Refreshes the journal when the operator reveals it. */
  onViewJournal?: () => void;
  /** The journal tab is no longer the surface being read. */
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
 * The Terminal app's window body.
 *
 * Composes the tab strip, the identity head and one pane, and owns the scope:
 * which `(workspace, profile)` these terminals belong to, and which buffers may
 * still exist. The journal stays mounted across tab changes so its scroll and
 * selection survive. Execute-only still hosts the journal tab and empty chrome.
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
  auditBlockedIds,
  recordings,
  pipeOutput,
  journal,
  requestedTerminalId,
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
    activeTab,
    attentionIds,
    destinationTerminals,
    limitOpen,
    missingRequested,
    openTerminal,
    setActiveTab,
    setLimitOpen,
    tabsIdBase,
  } = useTerminalWindowAppState({
    workspaceId,
    profile,
    terminals,
    inputRequests,
    limit,
    readOnly,
    actions,
    requestedTerminalId,
    onSelectTerminal,
    onViewJournal,
    onLeaveJournal,
  });
  const showJournal = activeTab === TERMINAL_JOURNAL_TAB;
  const openJournal = () => setActiveTab(TERMINAL_JOURNAL_TAB);

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="terminal-window" ref={rootRef}>
      {compact ? null : (
        <TerminalTabs
          activeTab={activeTab}
          attentionIds={attentionIds}
          idBase={tabsIdBase}
          limit={limit}
          onCloseTerminal={readOnly ? undefined : actions.onCloseTerminal}
          onOpenTerminal={interactiveAvailable ? openTerminal : undefined}
          onSelect={setActiveTab}
          showOwner={readOnly}
          terminals={terminals}
        />
      )}
      <div
        aria-labelledby={
          activeTab === TERMINAL_NO_TERMINALS ? undefined : terminalTabDomId(tabsIdBase, activeTab)
        }
        className="flex min-h-0 flex-1 flex-col bg-terminal-bg"
        id={terminalPanelDomId(tabsIdBase)}
        role="tabpanel"
      >
        <Activity mode={showJournal ? "visible" : "hidden"}>
          <div className="flex min-h-0 flex-1 flex-col">
            <TerminalJournalHostChrome
              hostChrome={hostChrome && showJournal}
              projectLabel={projectLabel}
            />
            {hostChrome ? null : <TerminalJournalHead projectLabel={projectLabel} />}
            {journal}
          </div>
        </Activity>
        {showJournal ? null : missingRequested ? (
          <TerminalNotFoundState onOpenTerminal={openTerminal} onViewJournal={openJournal} />
        ) : !interactiveAvailable && active === null ? (
          <TerminalExecuteOnlyState onViewJournal={openJournal} />
        ) : active === null ? (
          <TerminalEmptyState onOpenTerminal={openTerminal} />
        ) : (
          <TerminalWindowBody
            actions={{ ...actions, onOpenTerminal: openTerminal }}
            auditBlocked={auditBlockedIds?.has(active.id) ?? false}
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
            terminalCount={terminals.length}
            limit={limit}
            viewerId={viewerId}
            viewerToken={viewerToken}
            workspaceId={workspaceId}
          />
        )}
      </div>
      {!readOnly ? (
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
