"use client";

import type { TerminalEngineLoader } from "@compozy/ui";

import type { TerminalAttachmentSocketFactory } from "../hooks/use-terminal-attachment";
import {
  TERMINAL_NO_TERMINALS,
  useTerminalWindowAppState,
} from "../hooks/use-terminal-window-app-state";
import { terminalInstanceKey } from "../lib/terminal-scope-key";
import type { TerminalInfo, TerminalInputRequest } from "../types";
import { TerminalEmptyState, TerminalExecuteOnlyState } from "./terminal-empty-states";
import type { TerminalRecordingState } from "./terminal-header";
import { TerminalJournalHead } from "./terminal-journal-panel";
import { TerminalLimitDialog } from "./terminal-limit-dialog";
import { TERMINAL_JOURNAL_TAB, terminalPanelDomId, terminalTabDomId } from "./terminal-tab-id";
import { TerminalTabs } from "./terminal-tabs";
import type { TerminalWindowActions } from "./terminal-window-actions";
import { TerminalWindowBody } from "./terminal-window-body";

export type { TerminalWindowActions } from "./terminal-window-actions";

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
  /** The per-project cap, from `[terminal].max_per_workspace`. */
  limit: number;
  /** `[terminal].exit_retention` in milliseconds. Omit when unknown. */
  exitRetentionMs?: number;
  /** False where the platform cannot host an interactive terminal at all. */
  interactiveAvailable: boolean;
  /** Aggregate profile reads cannot mutate terminals owned by another profile. */
  readOnly?: boolean;
  auditBlockedIds?: ReadonlySet<string>;
  recordings?: Readonly<Record<string, TerminalRecordingState>>;
  /** Pipe terminals render captured output rather than a live stream. */
  pipeOutput?: Readonly<Record<string, { lines: readonly string[]; firstLineNumber: number }>>;
  journal: React.ReactNode;
  /** Refreshes the journal when the operator reveals it. */
  onViewJournal?: () => void;
  actions: TerminalWindowActions;
  socketFactory?: TerminalAttachmentSocketFactory;
  /** Replaces the emulator. Tests and playback harnesses only. */
  engineLoader?: TerminalEngineLoader;
}

/**
 * The Terminal app's window body.
 *
 * Composes the tab strip, the identity head and one pane, and owns the scope:
 * which `(workspace, profile)` these terminals belong to, and which buffers may
 * still exist. It offers no affordance the daemon cannot honour — on an
 * execute-only platform the whole interactive half is absent rather than
 * disabled.
 */
export function TerminalWindowApp({
  workspaceId,
  projectLabel,
  profile,
  viewerId,
  viewerToken,
  terminals,
  inputRequests,
  limit,
  exitRetentionMs,
  interactiveAvailable,
  readOnly = false,
  auditBlockedIds,
  recordings,
  pipeOutput,
  journal,
  onViewJournal,
  actions,
  socketFactory,
  engineLoader,
}: TerminalWindowAppProps) {
  const {
    active,
    activeProfile,
    activeTab,
    attentionIds,
    destinationTerminals,
    limitOpen,
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
    onViewJournal,
  });

  if (!interactiveAvailable) {
    return (
      <div className="flex min-h-0 flex-1 flex-col" data-testid="terminal-window">
        <TerminalExecuteOnlyState onViewJournal={() => setActiveTab(TERMINAL_JOURNAL_TAB)} />
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="terminal-window">
      <TerminalTabs
        activeTab={activeTab}
        attentionIds={attentionIds}
        idBase={tabsIdBase}
        limit={limit}
        onCloseTerminal={readOnly ? undefined : actions.onCloseTerminal}
        onOpenTerminal={openTerminal}
        onSelect={setActiveTab}
        showOwner={readOnly}
        terminals={terminals}
      />
      <div
        aria-labelledby={
          activeTab === TERMINAL_NO_TERMINALS ? undefined : terminalTabDomId(tabsIdBase, activeTab)
        }
        className="flex min-h-0 flex-1 flex-col"
        id={terminalPanelDomId(tabsIdBase)}
        role="tabpanel"
      >
        {activeTab === TERMINAL_JOURNAL_TAB ? (
          <>
            <TerminalJournalHead projectLabel={projectLabel} />
            {journal}
          </>
        ) : active === null ? (
          <TerminalEmptyState onOpenTerminal={openTerminal} />
        ) : (
          <TerminalWindowBody
            actions={actions}
            auditBlocked={auditBlockedIds?.has(active.id) ?? false}
            engineLoader={engineLoader}
            exitRetentionMs={exitRetentionMs}
            inputRequests={inputRequests.filter(request => request.terminal_id === active.id)}
            // Keyed by the full scoped identity: the same terminal id under a
            // different profile is a different terminal, and must not inherit
            // the previous one's in-flight confirmation.
            key={terminalInstanceKey(workspaceId, activeProfile, active.id)}
            onViewJournal={() => setActiveTab(TERMINAL_JOURNAL_TAB)}
            pipeOutput={pipeOutput?.[active.id]}
            profile={activeProfile}
            readOnly={readOnly}
            recording={recordings?.[active.id] ?? null}
            socketFactory={socketFactory}
            terminal={active}
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
