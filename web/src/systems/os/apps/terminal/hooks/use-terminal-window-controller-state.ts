import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast, type Filter } from "@compozy/ui";

import { useProfileReadScope, useProfiles } from "@/systems/profiles";
import { useSettingsGeneral } from "@/systems/settings";
import {
  closeTerminal,
  controlTerminalRecording,
  createTerminal,
  rejectTerminalInputRequest,
  signalTerminal,
  waitTerminal,
  terminalCatalogQuery,
  terminalInputRequestsQuery,
  terminalJournalQuery,
  terminalKeys,
  terminalRecordingQuery,
  terminalJournalFiltersFromChips,
  terminalScope,
  useTerminalCatalogStream,
  useTerminalInputAnswer,
  useTerminalRecordings,
  applyRecordingStopSuccess,
  unlockTerminalJournal,
  type TerminalInputRequest,
  type TerminalRecordingMap,
  type TerminalViewerIdentity,
  useTerminalJournalUnlocked,
} from "@/systems/terminal";
import { useActiveWorkspace } from "@/systems/workspace";

import { useDesktop } from "../../../hooks/use-desktop";
import { useOsShell } from "../../../hooks/use-os-shell";
import { matchTerminalInstance } from "../../../lib/app-catalog";
import { decideTerminalCloseHost } from "../lib/terminal-window-close";
import { terminalJournalQueryEnabled } from "../lib/terminal-window-journal";

const DEFAULT_ROUTE = "/terminal";

interface TerminalReplaySelection {
  id: string;
  profile: string;
  title: string;
}

export function useTerminalWindowControllerState(windowId: string) {
  // Chips are the interaction state; the query reads their projection, so a
  // chip still being typed filters nothing until it carries a value.
  const [journalChips, setJournalChips] = useState<Filter<string>[]>([]);
  const journalFilters = terminalJournalFiltersFromChips(journalChips);
  const [replay, setReplay] = useState<TerminalReplaySelection | null>(null);
  const [selectedCommandId, setSelectedCommandId] = useState<string | null>(null);
  const [journalVisible, setJournalVisible] = useState(false);
  const { coordinator, manager } = useOsShell();
  const queryClient = useQueryClient();
  const workspace = useActiveWorkspace();
  const profile = useProfileReadScope();
  const profiles = useProfiles();
  const settings = useSettingsGeneral();
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? DEFAULT_ROUTE);
  const viewerId = useDesktop(state => state.client?.clientId ?? null);
  const viewerToken = useDesktop(state => state.clientAttachmentToken);
  const viewer: TerminalViewerIdentity | null =
    viewerId && viewerToken ? { id: viewerId, attachmentToken: viewerToken } : null;
  const workspaceId = workspace.runtimeWorkspaceId ?? "";
  const journalUnlocked = useTerminalJournalUnlocked(workspaceId);
  const catalogScope = terminalScope(workspaceId, profile.destination, profile.aggregate);
  const destinationScope = terminalScope(workspaceId, profile.destination);
  const journalScope = catalogScope;
  const catalog = useQuery({
    ...terminalCatalogQuery(catalogScope),
    enabled: workspaceId !== "",
  });
  const inputRequestProjection = useQuery({
    ...terminalInputRequestsQuery(catalogScope),
    enabled: workspaceId !== "",
  });
  const inputRequests = {
    data: inputRequestProjection.data?.pending,
    error: inputRequestProjection.error,
    isPending: inputRequestProjection.isPending,
    refetch: inputRequestProjection.refetch,
  };
  const resolvedInputRequests = inputRequestProjection.data?.resolved ?? [];
  const journal = useInfiniteQuery({
    ...terminalJournalQuery(journalScope, journalFilters),
    enabled: terminalJournalQueryEnabled(workspaceId, journalUnlocked),
  });
  const recordings = useTerminalRecordings(catalogScope.key, workspaceId !== "");
  const recordingScope = terminalScope(workspaceId, replay?.profile ?? profile.destination);
  const recording = useQuery({
    ...terminalRecordingQuery(recordingScope, replay?.id ?? ""),
    enabled: workspaceId !== "" && replay !== null,
  });

  useTerminalCatalogStream({
    workspaceId,
    profileKey: catalogScope.key.profileKey,
    allProfiles: profile.aggregate,
    profiles: profiles.data?.map(candidate => candidate.name) ?? [],
    enabled: workspaceId !== "",
  });

  const terminalSelector = (terminalId: string) => ({
    profile:
      catalog.data?.find(terminal => terminal.id === terminalId)?.profile_name ??
      profile.destination,
  });

  const invalidateTerminalReads = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: terminalKeys.catalog(catalogScope.key),
        exact: true,
      }),
      queryClient.invalidateQueries({
        queryKey: terminalKeys.inputRequests(catalogScope.key),
        exact: true,
      }),
      queryClient.invalidateQueries({
        queryKey: terminalKeys.journalScope(journalScope.key),
      }),
    ]);
  };

  const create = useMutation({
    mutationFn: (identity: TerminalViewerIdentity) =>
      createTerminal(workspaceId, {}, destinationScope.params, identity),
    onSuccess: async terminal => {
      await invalidateTerminalReads();
      await coordinator.userRetarget(windowId, {
        app: "terminal",
        instanceKey: terminal.id,
        route: { pathname: `/terminal/${encodeURIComponent(terminal.id)}`, search: {} },
      });
    },
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to open terminal"),
  });
  const close = useMutation({
    mutationFn: (terminalId: string) =>
      closeTerminal(workspaceId, terminalId, terminalSelector(terminalId), "HUP"),
    onSuccess: async (_exit, closedId) => {
      await invalidateTerminalReads();
      const remaining = (catalog.data ?? []).filter(terminal => terminal.id !== closedId);
      const decision = decideTerminalCloseHost({
        closedId,
        routedId: matchTerminalInstance(pathname),
        remaining,
        journalVisible,
      });
      if (decision.kind === "noop") return;
      if (decision.kind === "retarget") {
        await coordinator.userRetarget(windowId, {
          app: "terminal",
          instanceKey: decision.terminalId,
          route: { pathname: `/terminal/${encodeURIComponent(decision.terminalId)}`, search: {} },
        });
        return;
      }
      if (decision.kind === "keep") {
        // Drop the dead terminal from the route so the journal stays the surface.
        const navigated = manager.navigateWindow(windowId, {
          pathname: DEFAULT_ROUTE,
          search: {},
        });
        await navigated.completion;
        return;
      }
      await coordinator.userClose(windowId);
    },
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to close terminal"),
  });
  const stop = useMutation({
    mutationFn: (terminalId: string) =>
      signalTerminal(workspaceId, terminalId, "TERM", terminalSelector(terminalId)),
    onSuccess: invalidateTerminalReads,
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to stop command"),
  });
  const answer = useTerminalInputAnswer(workspaceId, {
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(error.message),
  });
  const reject = useMutation({
    mutationFn: (request: TerminalInputRequest) =>
      rejectTerminalInputRequest(
        workspaceId,
        request.terminal_id,
        request.id,
        "Rejected by the operator",
        { profile: request.profile_name }
      ),
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(error instanceof Error ? error.message : "Failed to decline"),
  });
  const wait = useMutation({
    mutationFn: (terminalId: string) =>
      waitTerminal(workspaceId, terminalId, { until: "exit" }, terminalSelector(terminalId)),
    onSuccess: invalidateTerminalReads,
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to wait for the command"),
  });
  const stopRecording = useMutation({
    mutationFn: (terminalId: string) =>
      controlTerminalRecording(workspaceId, terminalId, "stop", terminalSelector(terminalId)),
    onSuccess: async recording => {
      queryClient.setQueryData<TerminalRecordingMap>(
        terminalKeys.recordings(catalogScope.key),
        current => applyRecordingStopSuccess(current ?? {}, recording)
      );
      await invalidateTerminalReads();
    },
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to stop recording"),
  });

  return {
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
    unlockJournal: () => unlockTerminalJournal(workspaceId),
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
  };
}
