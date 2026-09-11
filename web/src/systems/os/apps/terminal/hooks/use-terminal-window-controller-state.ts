import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast, type Filter } from "@compozy/ui";

import { useGatewayCapabilities } from "@/systems/gateway";
import { useProfileReadScope, useProfiles } from "@/systems/profiles";
import { useSettingsGeneral } from "@/systems/settings";
import {
  closeTerminal,
  controlTerminalRecording,
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
import { terminalJournalQueryEnabled } from "../lib/terminal-window-journal";
import { useTerminalWindowCreation } from "./use-terminal-window-creation";

const DEFAULT_ROUTE = "/terminal";

interface TerminalReplaySelection {
  id: string;
  profile: string;
  title: string;
}

interface TerminalWindowRouteState {
  windows: Record<
    string,
    { app: string; instanceKey: string | null; route: { search: Record<string, unknown> } }
  >;
}

/** Terminal ids other OS windows already show, as a stable selector value. */
function windowedTerminalKeys(state: TerminalWindowRouteState, currentWindowId: string): string {
  const keys: string[] = [];
  for (const [id, window] of Object.entries(state.windows)) {
    if (id !== currentWindowId && window.app === "terminal" && window.instanceKey) {
      keys.push(window.instanceKey);
    }
  }
  return keys.sort().join("\n");
}

/** Owns terminal creation, process controls, and retained journal state for one managed window. */
export function useTerminalWindowControllerState(windowId: string) {
  // Chips are the interaction state; the query reads their projection, so a
  // chip still being typed filters nothing until it carries a value.
  const [journalChips, setJournalChips] = useState<Filter<string>[]>([]);
  const journalFilters = terminalJournalFiltersFromChips(journalChips);
  const [replay, setReplay] = useState<TerminalReplaySelection | null>(null);
  const [selectedCommandId, setSelectedCommandId] = useState<string | null>(null);
  const { coordinator, manager } = useOsShell();
  const queryClient = useQueryClient();
  const workspace = useActiveWorkspace();
  const profile = useProfileReadScope();
  const profiles = useProfiles();
  const settings = useSettingsGeneral();
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? DEFAULT_ROUTE);
  // The `new` search key is the deliberate-creation mark: a window opened by
  // the head's New must open a fresh terminal, never adopt a windowless one.
  const createRequested = useDesktop(
    state => state.windows[windowId]?.route.search.new !== undefined
  );
  const windowedKeys = useDesktop(state => windowedTerminalKeys(state, windowId));
  const viewerId = useDesktop(state => state.client?.clientId ?? null);
  const viewerToken = useDesktop(state => state.clientAttachmentToken);
  const viewer: TerminalViewerIdentity | null =
    viewerId && viewerToken ? { id: viewerId, attachmentToken: viewerToken } : null;
  const workspaceId = workspace.runtimeWorkspaceId ?? "";
  // Remote tiers register no terminal routes: the window renders the truthful
  // loopback-only state, and none of these reads even run. Both the journal
  // and the recording read carry readsEnabled themselves: an unlocked journal
  // or a selected replay must not fire requests at routes the tier never
  // registers when the capability flips after render.
  const { localTaskLifecycle } = useGatewayCapabilities();
  const readsEnabled = workspaceId !== "" && localTaskLifecycle;
  const journalUnlocked = useTerminalJournalUnlocked(workspaceId);
  const catalogScope = terminalScope(workspaceId, profile.destination, profile.aggregate);
  const destinationScope = terminalScope(workspaceId, profile.destination);
  const journalScope = catalogScope;
  const catalog = useQuery({
    ...terminalCatalogQuery(catalogScope),
    enabled: readsEnabled,
  });
  const inputRequestProjection = useQuery({
    ...terminalInputRequestsQuery(catalogScope),
    enabled: readsEnabled,
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
    enabled: readsEnabled && terminalJournalQueryEnabled(workspaceId, journalUnlocked),
  });
  const recordings = useTerminalRecordings(catalogScope.key, readsEnabled);
  const recordingScope = terminalScope(workspaceId, replay?.profile ?? profile.destination);
  const recording = useQuery({
    ...terminalRecordingQuery(recordingScope, replay?.id ?? ""),
    enabled: readsEnabled && replay !== null,
  });

  useTerminalCatalogStream({
    workspaceId,
    profileKey: catalogScope.key.profileKey,
    allProfiles: profile.aggregate,
    profiles: profiles.data?.map(candidate => candidate.name) ?? [],
    enabled: readsEnabled,
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

  const create = useTerminalWindowCreation({
    windowId,
    workspaceId,
    catalogScope,
    destinationScope,
    coordinator,
  });
  const close = useMutation({
    mutationFn: (terminalId: string) =>
      closeTerminal(workspaceId, terminalId, terminalSelector(terminalId), "HUP"),
    // The terminal-limit dialog frees a slot without closing its launcher.
    // Normal window close is owned by the shell's terminal close guard.
    onSuccess: invalidateTerminalReads,
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
    createRequested,
    windowedTerminalIds: new Set(windowedKeys === "" ? [] : windowedKeys.split("\n")),
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
