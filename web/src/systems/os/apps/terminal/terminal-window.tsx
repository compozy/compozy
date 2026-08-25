import type { TerminalSelectionRange } from "@compozy/ui";
import { Button, Empty, Spinner } from "@compozy/ui";
import { AlertCircle, FolderOpen } from "lucide-react";
import { toast } from "sonner";

import { stageSessionTerminalQuote } from "@/systems/session";
import {
  terminalSelectionLines,
  TerminalJournalFilterDialog,
  TerminalJournalPanel,
  TerminalRecordingPlayer,
  TerminalStoreProvider,
  TerminalWindowApp,
  type TerminalInfo,
  type TerminalJournalFilters,
} from "@/systems/terminal";

import type { OsDesktopRuntimeStore } from "../../lib/os-types";
import { useTerminalWindowControllerState } from "./hooks/use-terminal-window-controller-state";

function terminalIdFromPath(pathname: string): string | null {
  const match = /^\/terminal\/([^/]+)$/.exec(pathname);
  return match ? decodeURIComponent(match[1]) : null;
}

function durationMilliseconds(value: string | undefined): number | undefined {
  const match = value?.trim().match(/^(\d+)(ms|s|m|h)$/);
  if (!match) return undefined;
  const amount = Number(match[1]);
  const unit = match[2];
  if (unit === "h") return amount * 3_600_000;
  if (unit === "m") return amount * 60_000;
  if (unit === "s") return amount * 1_000;
  return amount;
}

function orderedTerminals(terminals: readonly TerminalInfo[], requestedId: string | null) {
  if (requestedId === null) return terminals;
  const requested = terminals.find(terminal => terminal.id === requestedId);
  return requested
    ? [requested, ...terminals.filter(terminal => terminal.id !== requestedId)]
    : terminals;
}

function mostRecentSession(state: OsDesktopRuntimeStore, currentWindowId: string) {
  for (const id of state.client?.focusOrder ?? []) {
    const candidate = state.windows[id];
    if (id !== currentWindowId && candidate?.app === "session" && candidate.instanceKey) {
      return { id, sessionId: candidate.instanceKey };
    }
  }
  return null;
}

function terminalJournalFilterChips(filters: TerminalJournalFilters) {
  return [
    ...(filters.actor ? [{ key: "actor" as const, label: "who", value: filters.actor }] : []),
    ...(filters.since ? [{ key: "since" as const, label: "since", value: filters.since }] : []),
    ...(filters.failed ? [{ key: "failed" as const, label: "result", value: "failed" }] : []),
    ...(filters.terminalId
      ? [{ key: "terminalId" as const, label: "terminal", value: filters.terminalId }]
      : []),
  ];
}

export function TerminalWindow({ windowId }: { windowId: string }) {
  return (
    <TerminalStoreProvider>
      <TerminalWindowController windowId={windowId} />
    </TerminalStoreProvider>
  );
}

function TerminalWindowController({ windowId }: { windowId: string }) {
  const {
    answer,
    catalog,
    close,
    coordinator,
    create,
    filtersOpen,
    inputRequests,
    journal,
    journalFilters,
    manager,
    pathname,
    profile,
    recording,
    reject,
    replay,
    setFiltersOpen,
    setJournalFilters,
    setReplay,
    settings,
    stop,
    stopRecording,
    viewerId,
    workspace,
    workspaceId,
  } = useTerminalWindowControllerState(windowId);

  if (workspaceId === "") {
    return <Empty icon={FolderOpen} title="Choose a project to use Terminal" />;
  }
  if (catalog.isPending || inputRequests.isPending || journal.isPending) {
    return <Spinner className="m-auto size-5 text-subtle" />;
  }
  const error = catalog.error ?? inputRequests.error ?? journal.error;
  if (error) {
    return (
      <Empty
        action={
          <Button onClick={() => void catalog.refetch()} size="sm" type="button" variant="outline">
            Retry
          </Button>
        }
        icon={AlertCircle}
        title={error instanceof Error ? error.message : "Failed to load Terminal"}
      />
    );
  }

  const requestedId = terminalIdFromPath(pathname);
  const terminals = orderedTerminals(catalog.data ?? [], requestedId);
  const activeSession = mostRecentSession(manager.getState(), windowId);
  const terminalSettings = settings.data?.config.terminal;
  const interactiveAvailable =
    !workspace.runtimeWorkspace?.sandbox_ref &&
    terminals.every(terminal => terminal.capabilities.interactive);
  const journalEntries = journal.data?.pages.flatMap(page => page.entries) ?? [];
  const journalContent = replay ? (
    recording.isPending ? (
      <Spinner className="m-auto size-5 text-subtle" />
    ) : recording.error ? (
      <Empty
        action={
          <Button
            onClick={() => void recording.refetch()}
            size="sm"
            type="button"
            variant="outline"
          >
            Retry
          </Button>
        }
        icon={AlertCircle}
        title={
          recording.error instanceof Error ? recording.error.message : "Failed to load recording"
        }
      />
    ) : (
      <TerminalRecordingPlayer
        onOpenJournal={() => setReplay(null)}
        recordingId={replay.id}
        retentionNote={
          terminalSettings
            ? `Kept for ${terminalSettings.recording_retention_days} days`
            : undefined
        }
        source={recording.data ?? ""}
        title={replay.title}
      />
    )
  ) : (
    <TerminalJournalPanel
      entries={journalEntries}
      filters={terminalJournalFilterChips(journalFilters)}
      hasMore={journal.hasNextPage}
      isLoadingMore={journal.isFetchingNextPage}
      onClearFilter={key =>
        setJournalFilters(current => {
          const next = { ...current };
          delete next[key];
          return next;
        })
      }
      onClearFilters={() => setJournalFilters({})}
      onLoadMore={() => void journal.fetchNextPage()}
      onOpenFilters={() => setFiltersOpen(true)}
      onOpenNewTerminal={() => create.mutate()}
      onOpenTerminal={terminalId => {
        void coordinator.userRetarget(windowId, {
          app: "terminal",
          instanceKey: terminalId,
          route: { pathname: `/terminal/${encodeURIComponent(terminalId)}`, search: {} },
        });
      }}
      onReplay={recordingId => {
        const entry = journalEntries.find(candidate => candidate.recording === recordingId);
        if (!entry) return;
        setReplay({ id: recordingId, profile: entry.profile_name, title: entry.command });
      }}
      showOwner={profile.aggregate}
    />
  );

  const sendSelection = (terminalId: string, selection: TerminalSelectionRange) => {
    const target = mostRecentSession(manager.getState(), windowId);
    if (!target) return;
    stageSessionTerminalQuote({
      sessionId: target.sessionId,
      terminalId,
      fromLine: selection.startLine,
      lines: terminalSelectionLines(selection.text),
    });
    void coordinator.userActivateWindow(target.id);
  };

  return (
    <>
      <TerminalWindowApp
        actions={{
          onOpenTerminal: () => create.mutate(),
          onCloseTerminal: terminalId => close.mutate(terminalId),
          onStop: terminalId => stop.mutate(terminalId),
          onStopRecording: terminalId => stopRecording.mutate(terminalId),
          onAnswerInputRequest: (request, value) => answer.mutate({ request, value }),
          onRejectInputRequest: request => reject.mutate(request),
          onCopySelection: (_terminalId, selection) => {
            void navigator.clipboard
              .writeText(selection.text)
              .catch(() => toast.error("Copy failed"));
          },
          onSendSelection: sendSelection,
          onChooseSession: () => {
            void coordinator.userOpen({
              app: "agents",
              route: { pathname: "/agents", search: {} },
            });
          },
          onStartSession: () => {
            void coordinator.userOpen({
              app: "agents",
              route: { pathname: "/agents", search: {} },
            });
          },
          hasActiveSession: activeSession !== null,
          onOpenSettings: () => {
            void coordinator.userOpen({
              app: "settings",
              route: { pathname: "/settings/general", search: {} },
            });
          },
        }}
        exitRetentionMs={durationMilliseconds(terminalSettings?.exit_retention)}
        inputRequests={inputRequests.data ?? []}
        interactiveAvailable={interactiveAvailable}
        journal={journalContent}
        limit={terminalSettings?.max_per_workspace ?? 8}
        profile={profile.destination}
        terminals={terminals}
        viewerId={viewerId}
        workspaceId={workspaceId}
      />
      {filtersOpen ? (
        <TerminalJournalFilterDialog
          onApply={setJournalFilters}
          onOpenChange={setFiltersOpen}
          open
          value={journalFilters}
        />
      ) : null}
    </>
  );
}
