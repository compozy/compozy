import type { TerminalSelectionRange } from "@compozy/ui";
import { Button, Empty, Spinner } from "@compozy/ui";
import { AlertCircle, FolderOpen } from "lucide-react";
import { toast } from "sonner";

import { stageSessionTerminalQuote, useSessionCreateActions } from "@/systems/session";
import { parsePositiveDurationMilliseconds } from "@/systems/settings";
import {
  buildTerminalQuote,
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
import { useDesktop } from "../../hooks/use-desktop";
import { matchTerminalInstance } from "../../lib/app-catalog";
import { useTerminalWindowControllerState } from "./hooks/use-terminal-window-controller-state";

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
  const sessionCreate = useSessionCreateActions();
  const activeSessionWindowId = useDesktop(state => mostRecentSession(state, windowId)?.id ?? null);
  const activeSessionId = useDesktop(
    state => mostRecentSession(state, windowId)?.sessionId ?? null
  );
  const activeSession =
    activeSessionWindowId && activeSessionId
      ? { id: activeSessionWindowId, sessionId: activeSessionId }
      : null;
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
    viewerToken,
    workspace,
    workspaceId,
  } = useTerminalWindowControllerState(windowId);

  if (workspaceId === "") {
    return <Empty icon={FolderOpen} title="Choose a project to use Terminal" />;
  }
  if (catalog.isPending || inputRequests.isPending) {
    return <Spinner className="m-auto size-5 text-subtle" />;
  }
  const error = catalog.error ?? inputRequests.error;
  if (error) {
    const retry = catalog.error
      ? catalog.refetch
      : inputRequests.error
        ? inputRequests.refetch
        : journal.refetch;
    return (
      <Empty
        action={
          <Button onClick={() => void retry()} size="sm" type="button" variant="outline">
            Retry
          </Button>
        }
        icon={AlertCircle}
        title={error instanceof Error ? error.message : "Failed to load Terminal"}
      />
    );
  }

  const requestedId = matchTerminalInstance(pathname);
  if (requestedId && !(catalog.data ?? []).some(terminal => terminal.id === requestedId)) {
    const fallbackTerminal = catalog.data?.[0];
    return (
      <Empty
        action={
          <Button
            onClick={() => {
              if (!fallbackTerminal) {
                create.mutate();
                return;
              }
              void coordinator.userRetarget(windowId, {
                app: "terminal",
                instanceKey: fallbackTerminal.id,
                route: {
                  pathname: `/terminal/${encodeURIComponent(fallbackTerminal.id)}`,
                  search: {},
                },
              });
            }}
            size="sm"
            type="button"
            variant="outline"
          >
            {fallbackTerminal ? "View terminals" : "Open terminal"}
          </Button>
        }
        icon={AlertCircle}
        title="Terminal not found"
      />
    );
  }
  const terminals = orderedTerminals(catalog.data ?? [], requestedId);
  const terminalSettings = settings.data?.config.terminal;
  const interactiveAvailable = !workspace.runtimeWorkspace?.sandbox_ref;
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
  ) : journal.isPending ? (
    <Spinner className="m-auto size-5 text-subtle" />
  ) : journal.error ? (
    <Empty
      action={
        <Button onClick={() => void journal.refetch()} size="sm" type="button" variant="outline">
          Retry
        </Button>
      }
      icon={AlertCircle}
      title={journal.error instanceof Error ? journal.error.message : "Failed to load the journal"}
    />
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
          onStartSession: (terminalId, selection) => {
            const quote = buildTerminalQuote({
              terminalId,
              fromLine: selection.startLine,
              lines: terminalSelectionLines(selection.text),
            });
            sessionCreate.openWithPrompt(quote.text);
          },
          hasActiveSession: activeSession !== null,
          onOpenSettings: () => {
            void coordinator.userOpen({
              app: "settings",
              route: { pathname: "/settings/general", search: {} },
            });
          },
        }}
        exitRetentionMs={parsePositiveDurationMilliseconds(terminalSettings?.exit_retention)}
        inputRequests={inputRequests.data ?? []}
        interactiveAvailable={interactiveAvailable}
        journal={journalContent}
        limit={terminalSettings?.max_per_workspace ?? 8}
        onViewJournal={() => void journal.refetch()}
        profile={profile.destination}
        readOnly={profile.aggregate}
        terminals={terminals}
        viewerId={viewerId}
        viewerToken={viewerToken}
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
