import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast, type Filter } from "@compozy/ui";

import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { remoteRejectionMessage } from "@/lib/remote-error-copy";
import { findComputerById, useComputers } from "@/systems/computers";
import { useProfileReadScope, useProfiles } from "@/systems/profiles";
import { useSettingsGeneral } from "@/systems/settings";
import {
  closeTerminal,
  controlTerminalRecording,
  rejectTerminalInputRequest,
  signalTerminal,
  TerminalApiError,
  waitTerminal,
  terminalAggregateQuery,
  terminalAggregateScope,
  terminalCatalogQuery,
  terminalInputRequestsQuery,
  terminalJournalQuery,
  terminalKeys,
  terminalRecordingQuery,
  terminalJournalFiltersFromChips,
  terminalScope,
  useTerminalAggregateStream,
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
import {
  focusedWorkspaceSelector,
  useActiveWorkspace,
  useWindowDataWorkspace,
  useWorktreeAggregate,
  WINDOW_BINDING_SEARCH_KEY,
} from "@/systems/workspace";

import { useDesktop } from "../../../hooks/use-desktop";
import { useOsShell } from "../../../hooks/use-os-shell";
import { matchTerminalInstance } from "../../../lib/app-catalog";
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

function formatTerminalMutationError(error: unknown, fallback: string) {
  if (error instanceof TerminalApiError) {
    return remoteRejectionMessage(error.code, error.message, error.details);
  }
  return error instanceof Error ? error.message : fallback;
}

function useTerminalWindowBinding(windowId: string) {
  const workspace = useActiveWorkspace();
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? DEFAULT_ROUTE);
  const computerId: ComputerId = useDesktop(state => {
    const value = state.windows[windowId]?.route.search.computer;
    return normalizeComputerId(typeof value === "string" ? value : undefined);
  });
  const isRemote = isRemoteComputerId(computerId);
  const computers = useComputers({ enabled: isRemote });
  const remoteComputer = isRemote ? findComputerById(computers.computers, computerId) : undefined;
  // The `new` search key is the deliberate-creation mark: a window opened by
  // the head's New must open a fresh terminal, never adopt a windowless one.
  const createRequested = useDesktop(
    state => state.windows[windowId]?.route.search.new !== undefined
  );
  const windowedKeys = useDesktop(state => windowedTerminalKeys(state, windowId));
  const viewerId = useDesktop(state => state.client?.clientId ?? null);
  const viewerWorkspaceId = useDesktop(state => state.client?.workspaceId ?? null);
  const viewerToken = useDesktop(state => state.clientAttachmentToken);
  const viewer: TerminalViewerIdentity | null =
    !isRemote && viewerId && viewerToken && viewerWorkspaceId
      ? { id: viewerId, attachmentToken: viewerToken, workspaceId: viewerWorkspaceId }
      : null;
  // The window's scope: its binding (a project or `@user`), else its desktop. An
  // unbound window on the All projects desktop lists every project's terminals.
  const windowScope = useWindowDataWorkspace();
  const workspaceId = focusedWorkspaceSelector(windowScope);
  const binding = useDesktop(
    state => state.windows[windowId]?.route.search[WINDOW_BINDING_SEARCH_KEY]
  );
  const retargetSearch: Record<string, unknown> = {
    ...(typeof binding === "string" ? { [WINDOW_BINDING_SEARCH_KEY]: binding } : {}),
    ...(isRemote ? { computer: computerId } : {}),
  };
  const projectLabel =
    windowScope?.kind === "project"
      ? (workspace.registeredWorkspaces.find(item => item.id === windowScope.workspaceId)?.name ??
        windowScope.workspaceId)
      : undefined;

  return {
    aggregateMode: !isRemote && windowScope?.kind === "all",
    computerId,
    createRequested,
    isRemote,
    pathname,
    projectLabel,
    remoteComputer,
    retargetSearch,
    viewer,
    viewerId,
    viewerToken,
    viewerWorkspaceId,
    windowedTerminalIds: new Set(windowedKeys === "" ? [] : windowedKeys.split("\n")),
    workspace,
    workspaceId,
  };
}

function useTerminalWindowMutations(input: {
  windowId: string;
  workspaceId: string;
  computerId: ComputerId;
  catalogScope: ReturnType<typeof terminalScope>;
  destinationScope: ReturnType<typeof terminalScope>;
  coordinator: ReturnType<typeof useOsShell>["coordinator"];
  retargetSearch: Record<string, unknown>;
  terminalSelector: (terminalId: string) => { profile: string };
  invalidateTerminalReads: () => Promise<void>;
}) {
  const {
    windowId,
    workspaceId,
    computerId,
    catalogScope,
    destinationScope,
    coordinator,
    retargetSearch,
    terminalSelector,
    invalidateTerminalReads,
  } = input;
  const queryClient = useQueryClient();
  const scopedTerminalKeys = terminalKeys.forComputer(computerId);
  const isRemote = isRemoteComputerId(computerId);

  const create = useTerminalWindowCreation({
    windowId,
    workspaceId,
    computerId,
    catalogScope,
    destinationScope,
    coordinator,
    retargetSearch,
  });
  const close = useMutation({
    mutationFn: (terminalId: string) =>
      isRemote
        ? closeTerminal(
            workspaceId,
            terminalId,
            terminalSelector(terminalId),
            "HUP",
            undefined,
            computerId
          )
        : closeTerminal(workspaceId, terminalId, terminalSelector(terminalId), "HUP"),
    // The terminal-limit dialog frees a slot without closing its launcher.
    // Normal window close is owned by the shell's terminal close guard.
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(formatTerminalMutationError(error, "Failed to close terminal")),
  });
  const stop = useMutation({
    mutationFn: (terminalId: string) =>
      isRemote
        ? signalTerminal(
            workspaceId,
            terminalId,
            "TERM",
            terminalSelector(terminalId),
            undefined,
            computerId
          )
        : signalTerminal(workspaceId, terminalId, "TERM", terminalSelector(terminalId)),
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(formatTerminalMutationError(error, "Failed to stop command")),
  });
  const answer = useTerminalInputAnswer(workspaceId, {
    computerId,
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(formatTerminalMutationError(error, "Failed to answer")),
  });
  const reject = useMutation({
    mutationFn: (request: TerminalInputRequest) =>
      isRemote
        ? rejectTerminalInputRequest(
            workspaceId,
            request.terminal_id,
            request.id,
            "Rejected by the operator",
            { profile: request.profile_name },
            undefined,
            computerId
          )
        : rejectTerminalInputRequest(
            workspaceId,
            request.terminal_id,
            request.id,
            "Rejected by the operator",
            { profile: request.profile_name }
          ),
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(formatTerminalMutationError(error, "Failed to decline")),
  });
  const wait = useMutation({
    mutationFn: (terminalId: string) =>
      isRemote
        ? waitTerminal(
            workspaceId,
            terminalId,
            { until: "exit" },
            terminalSelector(terminalId),
            undefined,
            computerId
          )
        : waitTerminal(workspaceId, terminalId, { until: "exit" }, terminalSelector(terminalId)),
    onSuccess: invalidateTerminalReads,
    onError: error =>
      toast.error(formatTerminalMutationError(error, "Failed to wait for the command")),
  });
  const stopRecording = useMutation({
    mutationFn: (terminalId: string) =>
      isRemote
        ? controlTerminalRecording(
            workspaceId,
            terminalId,
            "stop",
            terminalSelector(terminalId),
            undefined,
            computerId
          )
        : controlTerminalRecording(workspaceId, terminalId, "stop", terminalSelector(terminalId)),
    onSuccess: async recording => {
      queryClient.setQueryData<TerminalRecordingMap>(
        scopedTerminalKeys.recordings(catalogScope.key),
        current => applyRecordingStopSuccess(current ?? {}, recording)
      );
      await invalidateTerminalReads();
    },
    onError: error => toast.error(formatTerminalMutationError(error, "Failed to stop recording")),
  });

  return { answer, close, create, reject, stop, stopRecording, wait };
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
  const profile = useProfileReadScope();
  const profiles = useProfiles();
  const settings = useSettingsGeneral();
  const bound = useTerminalWindowBinding(windowId);
  const { aggregateMode, computerId, pathname, workspaceId } = bound;
  const scopedTerminalKeys = terminalKeys.forComputer(computerId);
  // The launcher reads every project's terminals; the same scope keeps the dock's live mark warm.
  const aggregateScope = terminalAggregateScope([], profile.destination, profile.aggregate);
  const aggregateCatalog = useQuery({
    ...terminalAggregateQuery(aggregateScope),
    enabled: aggregateMode,
  });
  useTerminalAggregateStream({ scope: aggregateScope, enabled: aggregateMode });
  // The launcher's worktree rows: one store read across every project.
  const aggregateWorktrees = useWorktreeAggregate([], { enabled: aggregateMode });
  const journalUnlocked = useTerminalJournalUnlocked(workspaceId);
  const catalogScope = terminalScope(
    workspaceId,
    profile.destination,
    profile.aggregate,
    computerId
  );
  const destinationScope = terminalScope(workspaceId, profile.destination, false, computerId);
  // The id-less route resolves only from a catalog read after this window
  // mounted (`isFetchedAfterMount`). A list another window just fetched is still
  // fresh, so without a forced read the resolver would wait forever.
  const resolvesOnMount = matchTerminalInstance(pathname) === null;
  const catalog = useQuery({
    ...terminalCatalogQuery(catalogScope),
    enabled: workspaceId !== "",
    refetchOnMount: resolvesOnMount ? "always" : true,
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
    ...terminalJournalQuery(catalogScope, journalFilters),
    enabled: terminalJournalQueryEnabled(workspaceId, journalUnlocked),
  });
  const recordings = useTerminalRecordings(catalogScope.key, workspaceId !== "", computerId);
  const recordingScope = terminalScope(
    workspaceId,
    replay?.profile ?? profile.destination,
    false,
    computerId
  );
  const recording = useQuery({
    ...terminalRecordingQuery(recordingScope, replay?.id ?? ""),
    enabled: workspaceId !== "" && replay !== null,
  });

  useTerminalCatalogStream({
    workspaceId,
    profileKey: catalogScope.key.profileKey,
    computerId,
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
        queryKey: scopedTerminalKeys.catalog(catalogScope.key),
        exact: true,
      }),
      queryClient.invalidateQueries({
        queryKey: scopedTerminalKeys.inputRequests(catalogScope.key),
        exact: true,
      }),
      queryClient.invalidateQueries({
        queryKey: scopedTerminalKeys.journalScope(catalogScope.key),
      }),
    ]);
  };

  const mutations = useTerminalWindowMutations({
    windowId,
    workspaceId,
    computerId,
    catalogScope,
    destinationScope,
    coordinator,
    retargetSearch: bound.retargetSearch,
    terminalSelector,
    invalidateTerminalReads,
  });

  return {
    /** All projects: every project's terminals; opening one binds this window to it. */
    aggregate: aggregateMode
      ? { catalog: aggregateCatalog, worktrees: aggregateWorktrees.data?.worktrees }
      : null,
    ...mutations,
    catalog,
    coordinator,
    computerId: bound.computerId,
    createRequested: bound.createRequested,
    isRemote: bound.isRemote,
    remoteComputer: bound.remoteComputer,
    windowedTerminalIds: bound.windowedTerminalIds,
    inputRequests,
    resolvedInputRequests,
    journal,
    journalChips,
    manager,
    pathname: bound.pathname,
    profile,
    recording,
    recordings,
    replay,
    selectedCommandId,
    setJournalChips,
    journalUnlocked,
    unlockJournal: () => unlockTerminalJournal(workspaceId),
    setReplay,
    setSelectedCommandId,
    settings,
    viewerId: bound.viewerId,
    viewerToken: bound.viewerToken,
    viewerWorkspaceId: bound.viewerWorkspaceId,
    viewer: bound.viewer,
    workspace: bound.workspace,
    workspaceId: bound.workspaceId,
    /** The terminal's project name: the window's binding, else the focused project. */
    retargetSearch: bound.retargetSearch,
    projectLabel: bound.projectLabel,
  };
}
