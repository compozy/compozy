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
import {
  TerminalWindowControllerContext,
  useTerminalWindowControllerContext,
} from "./hooks/use-terminal-window-context";

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
  const controller = useTerminalWindowControllerState(windowId);
  return (
    <TerminalWindowControllerContext value={controller}>
      <TerminalWindowLoadBoundary windowId={windowId} />
    </TerminalWindowControllerContext>
  );
}

function TerminalWindowLoadBoundary({ windowId }: { windowId: string }) {
  const { catalog, inputRequests, workspaceId } = useTerminalWindowControllerContext();
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
    const retry = catalog.error ? catalog.refetch : inputRequests.refetch;
    return <TerminalWindowError error={error} retry={retry} title="Couldn't load Terminal" />;
  }
  return <TerminalWindowLoaded windowId={windowId} />;
}

function TerminalWindowLoaded({ windowId }: { windowId: string }) {
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
    createRequested,
    windowedTerminalIds,
    inputRequests,
    resolvedInputRequests,
    manager,
    pathname,
    profile,
    recordings,
    reject,
    unlockJournal,
    settings,
    stop,
    stopRecording,
    wait,
    viewerId,
    viewerToken,
    viewer,
    workspace,
    workspaceId,
  } = useTerminalWindowControllerContext();
  const openTerminal = viewer ? () => create.mutate(viewer) : undefined;
  const retargetTerminal = (terminalId: string) => {
    void coordinator.userRetarget(windowId, {
      app: "terminal",
      instanceKey: terminalId,
      route: { pathname: `/terminal/${encodeURIComponent(terminalId)}`, search: {} },
    });
  };
  const actions = useTerminalWindowHostActions({
    coordinator,
    getActiveSessionId: () => mostRecentSession(manager.getState(), windowId)?.sessionId ?? null,
    activateSession: sessionId => {
      const target = sessionWindowId(manager.getState(), sessionId);
      if (target) void coordinator.userActivateWindow(target);
    },
    hasActiveSession: activeSessionWindowId !== null && activeSessionId !== null,
    openTerminal,
    retargetTerminal,
    // Another terminal beside this one: a fresh window joins this frame as a
    // tab. `new` marks deliberate creation, so the resolver there opens a
    // fresh terminal instead of adopting a windowless running one.
    openTerminalTab: () => {
      void coordinator.userOpen({
        app: "terminal",
        forceNewInstance: true,
        stackTargetWindowId: windowId,
        route: { pathname: "/terminal", search: { new: "1" } },
      });
    },
    close: terminalId => close.mutate(terminalId),
    closePending: close.isPending,
    stop: terminalId => stop.mutate(terminalId),
    wait: terminalId => wait.mutate(terminalId),
    stopRecording: terminalId => stopRecording.mutate(terminalId),
    answer: (request, value) => answer.mutate({ request, value }),
    reject: request => reject.mutate(request),
  });

  const requestedId = matchTerminalInstance(pathname);
  const terminals = catalog.data ?? [];
  const terminalSettings = settings.data?.config.terminal;
  const interactiveAvailable = !workspace.runtimeWorkspace?.sandbox_ref;

  return (
    <TerminalWindowApp
      hostChrome
      actions={actions}
      createIntent={createRequested}
      detachedTtl={terminalSettings?.detached_ttl}
      exitRetentionMs={parsePositiveDurationMilliseconds(terminalSettings?.exit_retention)}
      inputRequestTitles={new Map(terminals.map(terminal => [terminal.id, terminal.title]))}
      inputRequests={inputRequests.data ?? []}
      interactiveAvailable={interactiveAvailable}
      resolvedInputRequests={resolvedInputRequests}
      journal={
        <TerminalWindowJournal openTerminal={openTerminal} retargetTerminal={retargetTerminal} />
      }
      limit={terminalSettings?.max_per_workspace}
      recordings={recordings}
      onViewJournal={unlockJournal}
      profile={profile.destination}
      projectLabel={workspace.runtimeWorkspace?.name}
      readOnly={profile.aggregate}
      requestedTerminalId={requestedId}
      resolveReady={catalog.isFetchedAfterMount}
      terminals={terminals}
      viewerId={viewerId}
      viewerToken={viewerToken}
      windowedTerminalIds={windowedTerminalIds}
      workspaceId={workspaceId}
    />
  );
}

function TerminalWindowJournal({
  openTerminal,
  retargetTerminal,
}: {
  openTerminal?: () => void;
  retargetTerminal: (terminalId: string) => void;
}) {
  const {
    journal,
    journalChips,
    profile,
    replay,
    selectedCommandId,
    setJournalChips,
    setReplay,
    setSelectedCommandId,
    settings,
  } = useTerminalWindowControllerContext();
  if (journal.isPending && replay === null) {
    return <BlockLoading className="flex-1" label="Loading the journal" surface="bare" />;
  }
  if (journal.error && replay === null) {
    return (
      <TerminalWindowError
        error={journal.error}
        retry={journal.refetch}
        title="Couldn't load the journal"
      />
    );
  }
  const canCopyCommand =
    typeof navigator !== "undefined" &&
    navigator.clipboard !== undefined &&
    typeof navigator.clipboard.writeText === "function";
  return (
    <TerminalJournalPanel
      chips={journalChips}
      entries={journal.data?.pages.flatMap(page => page.entries) ?? []}
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
      onOpenTerminal={retargetTerminal}
      onReplay={(_recordingId, entry) => {
        if (!entry.recording) return;
        setSelectedCommandId(entry.command_id);
        setReplay({ id: entry.recording, profile: entry.profile_name, title: entry.command });
      }}
      onSelectedCommandIdChange={setSelectedCommandId}
      replay={
        replay === null ? undefined : (
          <TerminalWindowReplay
            retentionDays={settings.data?.config.terminal.recording_retention_days}
          />
        )
      }
      selectedCommandId={selectedCommandId}
      showOwner={profile.aggregate}
    />
  );
}

function TerminalWindowReplay({ retentionDays }: { retentionDays?: number }) {
  const { recording, replay, setReplay } = useTerminalWindowControllerContext();
  if (replay === null) return null;
  if (recording.isPending) {
    return <BlockLoading className="flex-1" label="Loading the recording" surface="bare" />;
  }
  if (recording.error) {
    return (
      <TerminalWindowError
        error={recording.error}
        retry={recording.refetch}
        title="Couldn't load the recording"
      />
    );
  }
  return (
    <TerminalRecordingPlayer
      onOpenJournal={() => setReplay(null)}
      recordingId={replay.id}
      retentionNote={retentionDays === undefined ? undefined : `Kept for ${retentionDays} days`}
      source={recording.data ?? ""}
      title={replay.title}
    />
  );
}

function TerminalWindowError({
  error,
  retry,
  title,
}: {
  error: unknown;
  retry: () => Promise<unknown>;
  title: string;
}) {
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
        title={title}
      />
    </div>
  );
}
