"use client";

import { destroyTerminalInstances, type TerminalEngineLoader } from "@compozy/ui";
import { useEffect, useState } from "react";

import type { TerminalSocketFactory } from "../adapters/terminal-socket";
import { useTerminalStore } from "../hooks/use-terminal-store";
import {
  isTerminalPaneKey,
  terminalInstanceKey,
  terminalInstanceKeyInScope,
  terminalScopeKey,
} from "../lib/terminal-scope-key";
import type { TerminalInfo, TerminalInputRequest } from "../types";
import { TerminalEmptyState, TerminalExecuteOnlyState } from "./terminal-empty-states";
import type { TerminalRecordingState } from "./terminal-header";
import { TerminalLimitDialog } from "./terminal-limit-dialog";
import { TERMINAL_JOURNAL_TAB, TerminalTabs, type TerminalTabId } from "./terminal-tabs";
import type { TerminalWindowActions } from "./terminal-window-actions";
import { TerminalWindowBody } from "./terminal-window-body";

export type { TerminalWindowActions } from "./terminal-window-actions";

/** No terminal is selected, because the project has none yet. */
const TERMINAL_NO_TERMINALS = "";

function hasTerminal(terminals: readonly TerminalInfo[], id: string): boolean {
  return terminals.some(terminal => terminal.id === id);
}

export interface TerminalWindowAppProps {
  workspaceId: string;
  profile: string;
  /** This browser's operator identity, exactly as the daemon names it. */
  viewerId: string | null;
  terminals: readonly TerminalInfo[];
  inputRequests: readonly TerminalInputRequest[];
  /** The per-project cap, from `[terminal].max_per_workspace`. */
  limit: number;
  /** `[terminal].exit_retention` in milliseconds. Omit when unknown. */
  exitRetentionMs?: number;
  /** False where the platform cannot host an interactive terminal at all. */
  interactiveAvailable: boolean;
  auditBlockedIds?: ReadonlySet<string>;
  recordings?: Readonly<Record<string, TerminalRecordingState>>;
  /** Pipe terminals render captured output rather than a live stream. */
  pipeOutput?: Readonly<Record<string, { lines: readonly string[]; firstLineNumber: number }>>;
  journal: React.ReactNode;
  actions: TerminalWindowActions;
  socketFactory?: TerminalSocketFactory;
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
  profile,
  viewerId,
  terminals,
  inputRequests,
  limit,
  exitRetentionMs,
  interactiveAvailable,
  auditBlockedIds,
  recordings,
  pipeOutput,
  journal,
  actions,
  socketFactory,
  engineLoader,
}: TerminalWindowAppProps) {
  // An empty project opens on the terminals view, not on the journal: with
  // nothing running, the honest first offer is opening a terminal.
  const [selectedTab, setSelectedTab] = useState<TerminalTabId | null>(null);
  const [limitOpen, setLimitOpen] = useState(false);
  const attentionIds = new Set(inputRequests.map(request => request.terminal_id));
  const selected =
    selectedTab !== null &&
    (selectedTab === TERMINAL_JOURNAL_TAB || hasTerminal(terminals, selectedTab))
      ? selectedTab
      : (terminals[0]?.id ?? null);
  const activeTab = selected ?? TERMINAL_NO_TERMINALS;
  const active = terminals.find(terminal => terminal.id === activeTab) ?? null;
  const setActiveTab = setSelectedTab;
  const atLimit = terminals.length >= limit;
  const store = useTerminalStore();
  const scopeKey = terminalScopeKey(workspaceId, profile);
  // Serialized as JSON rather than joined by a delimiter: a free delimiter is
  // exactly the collision the length-prefixed keys exist to prevent.
  const liveKeySignature = JSON.stringify(
    terminals.map(terminal => terminalInstanceKey(workspaceId, profile, terminal.id))
  );

  // A buffer is the only copy of what a viewer already saw, so it survives a tab
  // switch — but not the terminal itself. When a terminal leaves the catalog, or
  // the scope changes, its emulator, its WebGL context and its observers go with
  // it. Panes still on screen keep theirs, and buffers other surfaces own are
  // never touched.
  useEffect(() => {
    const live = new Set(JSON.parse(liveKeySignature) as string[]);
    store.trigger.scopeBound({ scopeKey });
    destroyTerminalInstances(key => {
      if (!isTerminalPaneKey(key)) return false;
      if (!terminalInstanceKeyInScope(key, scopeKey)) return true;
      return !live.has(key);
    });
  }, [liveKeySignature, scopeKey, store]);

  const openTerminal = actions.onOpenTerminal
    ? () => {
        if (atLimit) {
          setLimitOpen(true);
          return;
        }
        actions.onOpenTerminal?.();
      }
    : undefined;

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
        limit={limit}
        onCloseTerminal={actions.onCloseTerminal}
        onOpenTerminal={openTerminal}
        onSelect={setActiveTab}
        terminals={terminals}
      />
      {activeTab === TERMINAL_JOURNAL_TAB ? (
        journal
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
          // different profile is a different terminal, and must not inherit the
          // previous one's in-flight confirmation.
          key={terminalInstanceKey(workspaceId, profile, active.id)}
          onViewJournal={() => setActiveTab(TERMINAL_JOURNAL_TAB)}
          pipeOutput={pipeOutput?.[active.id]}
          profile={profile}
          recording={recordings?.[active.id] ?? null}
          socketFactory={socketFactory}
          terminal={active}
          viewerId={viewerId}
          workspaceId={workspaceId}
        />
      )}
      <TerminalLimitDialog
        limit={limit}
        onCloseTerminal={actions.onCloseTerminal}
        onOpenChange={setLimitOpen}
        onOpenSettings={actions.onOpenSettings}
        open={limitOpen}
        terminals={terminals}
      />
    </div>
  );
}
