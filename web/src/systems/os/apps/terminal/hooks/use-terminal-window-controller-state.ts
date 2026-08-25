import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";

import { useProfileReadScope } from "@/systems/profiles";
import { useSettingsGeneral } from "@/systems/settings";
import {
  answerTerminalInputRequest,
  closeTerminal,
  controlTerminalRecording,
  createTerminal,
  rejectTerminalInputRequest,
  signalTerminal,
  terminalCatalogQuery,
  terminalInputRequestsQuery,
  terminalJournalQuery,
  terminalKeys,
  terminalRecordingQuery,
  terminalScope,
  useTerminalCatalogStream,
  type TerminalInputRequest,
  type TerminalJournalFilters,
} from "@/systems/terminal";
import { useActiveWorkspace } from "@/systems/workspace";

import { useDesktop } from "../../../hooks/use-desktop";
import { useOsShell } from "../../../hooks/use-os-shell";

const DEFAULT_ROUTE = "/terminal";

interface TerminalReplaySelection {
  id: string;
  profile: string;
  title: string;
}

function terminalIdFromPath(pathname: string): string | null {
  const match = /^\/terminal\/([^/]+)$/.exec(pathname);
  return match ? decodeURIComponent(match[1]) : null;
}

export function useTerminalWindowControllerState(windowId: string) {
  const [journalFilters, setJournalFilters] = useState<TerminalJournalFilters>({});
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [replay, setReplay] = useState<TerminalReplaySelection | null>(null);
  const { coordinator, manager } = useOsShell();
  const queryClient = useQueryClient();
  const workspace = useActiveWorkspace();
  const profile = useProfileReadScope();
  const settings = useSettingsGeneral();
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? DEFAULT_ROUTE);
  const viewerId = useDesktop(state => state.client?.clientId ?? null);
  const workspaceId = workspace.runtimeWorkspaceId ?? "";
  const catalogScope = terminalScope(workspaceId, profile.destination, profile.aggregate);
  const destinationScope = terminalScope(workspaceId, profile.destination);
  const journalScope = catalogScope;
  const catalog = useQuery({
    ...terminalCatalogQuery(catalogScope),
    enabled: workspaceId !== "",
  });
  const inputRequests = useQuery({
    ...terminalInputRequestsQuery(catalogScope),
    enabled: workspaceId !== "",
  });
  const journal = useInfiniteQuery({
    ...terminalJournalQuery(journalScope, journalFilters),
    enabled: workspaceId !== "",
  });
  const recordingScope = terminalScope(workspaceId, replay?.profile ?? profile.destination);
  const recording = useQuery({
    ...terminalRecordingQuery(recordingScope, replay?.id ?? ""),
    enabled: workspaceId !== "" && replay !== null,
  });

  useTerminalCatalogStream({
    workspaceId,
    profileKey: catalogScope.key.profileKey,
    allProfiles: profile.aggregate,
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
    mutationFn: () => createTerminal(workspaceId, {}, destinationScope.params),
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
      const next = (catalog.data ?? []).find(terminal => terminal.id !== closedId);
      if (terminalIdFromPath(pathname) !== closedId) return;
      if (next) {
        await coordinator.userRetarget(windowId, {
          app: "terminal",
          instanceKey: next.id,
          route: { pathname: `/terminal/${encodeURIComponent(next.id)}`, search: {} },
        });
      } else {
        await coordinator.userClose(windowId);
      }
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
  const answer = useMutation({
    mutationFn: ({ request, value }: { request: TerminalInputRequest; value: string }) =>
      answerTerminalInputRequest(workspaceId, request.terminal_id, request.id, value, {
        profile: request.profile_name,
      }),
    onSuccess: invalidateTerminalReads,
    onError: error => toast.error(error instanceof Error ? error.message : "Failed to answer"),
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
    onError: error => toast.error(error instanceof Error ? error.message : "Failed to reject"),
  });
  const stopRecording = useMutation({
    mutationFn: (terminalId: string) =>
      controlTerminalRecording(workspaceId, terminalId, "stop", terminalSelector(terminalId)),
    onSuccess: invalidateTerminalReads,
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to stop recording"),
  });

  return {
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
  };
}
