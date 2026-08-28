import { BlockLoading, Button, Empty, toast } from "@compozy/ui";
import { AlertCircle, FolderOpen } from "lucide-react";

import { parsePositiveDurationMilliseconds } from "@/systems/settings";
import {
  TerminalJournalPanel,
  TerminalRecordingPlayer,
  TerminalStoreProvider,
  TerminalWindowApp,
} from "@/systems/terminal";

import type { OsDesktopRuntimeStore } from "../../lib/os-types";
import { useDesktop } from "../../hooks/use-desktop";
import { matchTerminalInstance } from "../../lib/app-catalog";
import { useTerminalWindowControllerState } from "./hooks/use-terminal-window-controller-state";
import { useTerminalWindowHostActions } from "./hooks/use-terminal-window-host-actions";

function mostRecentSession(state: OsDesktopRuntimeStore, currentWindowId: string) {
  for (const id of state.client?.focusOrder ?? []) {
    const candidate = state.windows[id];
    if (id !== currentWindowId && candidate?.app === "session" && candidate.instanceKey) {
      return { id, sessionId: candidate.instanceKey };
    }
  }
  return null;
}

function sessionWindowId(state: OsDesktopRuntimeStore, sessionId: string): string | null {
  for (const [id, window] of Object.entries(state.windows)) {
    if (window.app === "session" && window.instanceKey === sessionId) return id;
  }
  return null;
}

export function TerminalWindow({ windowId }: { windowId: string }) {
  return (
    <TerminalStoreProvider>
      <TerminalWindowController windowId={windowId} />
    </TerminalStoreProvider>
  );
}

function TerminalWindowController({ windowId }: { windowId: string }) {
  const activeSessionWindowId = useDesktop(state => mostRecentSession(state, windowId)?.id ?? null);
  const activeSessionId = useDesktop(
    state => mostRecentSession(state, windowId)?.sessionId ?? null
  );
  const {
    answer,
    catalog,
    close,
    coordinator,
    create,
    inputRequests,
    resolvedInputRequests,
    journal,
    journalChips,
    manager,
    pathname,
    profile,
    recording,
    recordings,
    reject,
    replay,
    selectedCommandId,
    setJournalChips,
    setJournalUnlocked,
    setJournalVisible,
    setReplay,
    setSelectedCommandId,
    settings,
    stop,
    stopRecording,
    wait,
    viewerId,
    viewerToken,
    viewer,
    workspace,
    workspaceId,
  } = useTerminalWindowControllerState(windowId);
  const openTerminal = viewer ? () => create.mutate(viewer) : undefined;
  const actions = useTerminalWindowHostActions({
    coordinator,
    getActiveSessionId: () => mostRecentSession(manager.getState(), windowId)?.sessionId ?? null,
    activateSession: sessionId => {
      const target = sessionWindowId(manager.getState(), sessionId);
      if (target) void coordinator.userActivateWindow(target);
    },
    hasActiveSession: activeSessionWindowId !== null && activeSessionId !== null,
    openTerminal,
    close: terminalId => close.mutate(terminalId),
    stop: terminalId => stop.mutate(terminalId),
    wait: terminalId => wait.mutate(terminalId),
    stopRecording: terminalId => stopRecording.mutate(terminalId),
    answer: (request, value) => answer.mutate({ request, value }),
    reject: request => reject.mutate(request),
  });

  if (workspaceId === "") {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty className="max-w-md" icon={FolderOpen} title="Choose a project to use Terminal" />
      </div>
    );
  }
  if (catalog.isPending || inputRequests.isPending) {
    return <BlockLoading className="flex-1" label="Loading terminals" surface="bare" />;
  }
  const error = catalog.error ?? inputRequests.error;
  if (error) {
    const retry = catalog.error
      ? catalog.refetch
      : inputRequests.error
        ? inputRequests.refetch
        : journal.refetch;
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          action={
            <Button onClick={() => void retry()} size="sm" type="button" variant="outline">
              Retry
            </Button>
          }
          className="max-w-md"
          description={error instanceof Error ? error.message : undefined}
          icon={AlertCircle}
          title="Couldn't load Terminal"
        />
      </div>
    );
  }

  const requestedId = matchTerminalInstance(pathname);
  const terminals = catalog.data ?? [];
  const terminalSettings = settings.data?.config.terminal;
  const interactiveAvailable = !workspace.runtimeWorkspace?.sandbox_ref;
  const journalEntries = journal.data?.pages.flatMap(page => page.entries) ?? [];
  const canCopyCommand =
    typeof navigator !== "undefined" &&
    navigator.clipboard !== undefined &&
    typeof navigator.clipboard.writeText === "function";
  const replayNode =
    replay === null ? undefined : recording.isPending ? (
      <BlockLoading className="flex-1" label="Loading the recording" surface="bare" />
    ) : recording.error ? (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
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
          className="max-w-md"
          description={recording.error instanceof Error ? recording.error.message : undefined}
          icon={AlertCircle}
          title="Couldn't load the recording"
        />
      </div>
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
    );
  const journalContent =
    journal.isPending && replay === null ? (
      <BlockLoading className="flex-1" label="Loading the journal" surface="bare" />
    ) : journal.error && replay === null ? (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          action={
            <Button
              onClick={() => void journal.refetch()}
              size="sm"
              type="button"
              variant="outline"
            >
              Retry
            </Button>
          }
          className="max-w-md"
          description={journal.error instanceof Error ? journal.error.message : undefined}
          icon={AlertCircle}
          title="Couldn't load the journal"
        />
      </div>
    ) : (
      <TerminalJournalPanel
        chips={journalChips}
        entries={journalEntries}
        hasMore={journal.hasNextPage}
        isLoadingMore={journal.isFetchingNextPage}
        onCopyCommand={
          canCopyCommand
            ? command =>
                void navigator.clipboard.writeText(command).catch(() => toast.error("Copy failed"))
            : undefined
        }
        onFiltersChange={setJournalChips}
        onLoadMore={() => void journal.fetchNextPage()}
        onOpenNewTerminal={openTerminal}
        onOpenTerminal={terminalId => {
          void coordinator.userRetarget(windowId, {
            app: "terminal",
            instanceKey: terminalId,
            route: { pathname: `/terminal/${encodeURIComponent(terminalId)}`, search: {} },
          });
        }}
        onReplay={(_recordingId, entry) => {
          const recordingId = entry.recording;
          if (!recordingId) return;
          setSelectedCommandId(entry.command_id);
          setReplay({ id: recordingId, profile: entry.profile_name, title: entry.command });
        }}
        onSelectedCommandIdChange={setSelectedCommandId}
        replay={replayNode}
        selectedCommandId={selectedCommandId}
        showOwner={profile.aggregate}
      />
    );

  return (
    <TerminalWindowApp
      hostChrome
      actions={actions}
      detachedTtl={terminalSettings?.detached_ttl}
      exitRetentionMs={parsePositiveDurationMilliseconds(terminalSettings?.exit_retention)}
      inputRequestTitles={new Map(terminals.map(terminal => [terminal.id, terminal.title]))}
      inputRequests={inputRequests.data ?? []}
      interactiveAvailable={interactiveAvailable}
      resolvedInputRequests={resolvedInputRequests}
      journal={journalContent}
      limit={terminalSettings?.max_per_workspace}
      recordings={recordings}
      onLeaveJournal={() => setJournalVisible(false)}
      onSelectTerminal={terminalId => {
        void coordinator.userRetarget(windowId, {
          app: "terminal",
          instanceKey: terminalId,
          route: { pathname: `/terminal/${encodeURIComponent(terminalId)}`, search: {} },
        });
      }}
      onViewJournal={() => {
        setJournalUnlocked(true);
        setJournalVisible(true);
      }}
      profile={profile.destination}
      projectLabel={workspace.runtimeWorkspace?.name}
      readOnly={profile.aggregate}
      requestedTerminalId={requestedId}
      terminals={terminals}
      viewerId={viewerId}
      viewerToken={viewerToken}
      workspaceId={workspaceId}
    />
  );
}
