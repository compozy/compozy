import type { TerminalSelectionRange } from "@compozy/ui";
import { BlockLoading, Button, Empty, toast } from "@compozy/ui";
import { AlertCircle, FolderOpen } from "lucide-react";

import { stageSessionTerminalQuote, useSessionCreateActions } from "@/systems/session";
import { parsePositiveDurationMilliseconds } from "@/systems/settings";
import {
  buildTerminalQuote,
  terminalSelectionLines,
  TerminalJournalPanel,
  TerminalNotFoundState,
  TerminalRecordingPlayer,
  TerminalStoreProvider,
  TerminalWindowApp,
  type TerminalInfo,
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
    inputRequests,
    resolvedInputRequests,
    journal,
    journalChips,
    manager,
    pathname,
    profile,
    recording,
    reject,
    replay,
    selectedCommandId,
    setJournalChips,
    setJournalVisible,
    setReplay,
    setSelectedCommandId,
    settings,
    stop,
    stopRecording,
    viewerId,
    viewerToken,
    viewer,
    workspace,
    workspaceId,
  } = useTerminalWindowControllerState(windowId);

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
  const openTerminal = viewer ? () => create.mutate(viewer) : undefined;
  if (requestedId && (catalog.data ?? []).length === 0) {
    return <TerminalNotFoundState onOpenTerminal={openTerminal} />;
  }
  const terminals = orderedTerminals(catalog.data ?? [], requestedId);
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
        hostChrome
        actions={{
          onOpenTerminal: openTerminal,
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
        inputRequestTitles={
          new Map((catalog.data ?? []).map(terminal => [terminal.id, terminal.title]))
        }
        inputRequests={inputRequests.data ?? []}
        interactiveAvailable={interactiveAvailable}
        resolvedInputRequests={resolvedInputRequests}
        journal={journalContent}
        limit={terminalSettings?.max_per_workspace ?? 8}
        onLeaveJournal={() => setJournalVisible(false)}
        onViewJournal={() => {
          setJournalVisible(true);
          void journal.refetch();
        }}
        profile={profile.destination}
        projectLabel={workspace.runtimeWorkspace?.name}
        readOnly={profile.aggregate}
        terminals={terminals}
        viewerId={viewerId}
        viewerToken={viewerToken}
        workspaceId={workspaceId}
      />
    </>
  );
}
