import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";
import { useProfileReadScope } from "@/systems/profiles";
import {
  createTerminal,
  terminalAggregateQuery,
  terminalAggregateScope,
  terminalKeys,
  terminalScope,
  useTerminalAggregateStream,
  type TerminalInfo,
  type TerminalViewerIdentity,
} from "@/systems/terminal";
import {
  ownerDataScope,
  useActiveWorkspace,
  useDataScope,
  useWorktreeAggregate,
} from "@/systems/workspace";

import {
  defaultTerminalLauncherDestination,
  runningTerminals,
  terminalLauncherCreateInput,
  terminalLauncherDestinations,
  type TerminalLauncherDestination,
} from "../lib/terminal-launcher-model";
import { useDesktop } from "./use-desktop";
import { useOpenTarget } from "./use-open-target";

const OPEN_FAILURE = "Couldn't open the terminal. Try again.";

function terminalOpenTarget(terminal: TerminalInfo) {
  return {
    app: "terminal" as const,
    instanceKey: terminal.id,
    binding: ownerDataScope(terminal.workspace_id),
    route: { pathname: `/terminal/${encodeURIComponent(terminal.id)}`, search: {} },
  };
}

/**
 * The All projects Terminal launcher: every project (and User) a new terminal
 * can start in, the running terminals to go back to, and the opens that land
 * each one in its own window bound to its scope.
 */
export function useTerminalLauncher({
  enabled,
  onOpened,
}: {
  enabled: boolean;
  onOpened: () => void;
}) {
  const workspace = useActiveWorkspace();
  const profile = useProfileReadScope();
  const openItem = useOpenTarget();
  const queryClient = useQueryClient();
  const viewerId = useDesktop(state => state.client?.clientId ?? null);
  const viewerWorkspaceId = useDesktop(state => state.client?.workspaceId ?? null);
  const viewerToken = useDesktop(state => state.clientAttachmentToken);
  const viewer: TerminalViewerIdentity | null =
    viewerId && viewerToken && viewerWorkspaceId
      ? { id: viewerId, attachmentToken: viewerToken, workspaceId: viewerWorkspaceId }
      : null;
  // The same scope the dock's live mark reads, so the launcher opens on a warm cache.
  const scope = terminalAggregateScope([], profile.destination, profile.aggregate);
  const catalog = useQuery({ ...terminalAggregateQuery(scope), enabled });
  useTerminalAggregateStream({ scope, enabled });
  // Every project's worktrees in one store read; the catalog stream keeps it fresh.
  const worktrees = useWorktreeAggregate([], { enabled });
  const destinations = terminalLauncherDestinations(
    workspace.workspaces,
    worktrees.data?.worktrees,
    profile.destination
  );
  const create = useMutation({
    mutationFn: ({
      destination,
      identity,
    }: {
      destination: TerminalLauncherDestination;
      identity: TerminalViewerIdentity;
    }) =>
      createTerminal(
        destination.workspaceId,
        terminalLauncherCreateInput(destination),
        { profile: profile.destination },
        identity
      ),
    onSuccess: async (terminal, { destination }) => {
      // The window that opens next reads its project's catalog; neither read may miss the new row.
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: terminalKeys.aggregate(scope.selection, scope.profileKey),
          exact: true,
        }),
        queryClient.invalidateQueries({
          queryKey: terminalKeys.catalog(
            terminalScope(destination.workspaceId, profile.destination, profile.aggregate).key
          ),
          exact: true,
        }),
      ]);
      openItem(terminalOpenTarget(terminal), OPEN_FAILURE);
      onOpened();
    },
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to open terminal"),
  });
  return {
    destinations,
    defaultDestination: defaultTerminalLauncherDestination(
      destinations,
      workspace.rememberedWorkspace?.id
    ),
    running: runningTerminals(catalog.data ?? []),
    projectNames: new Map(workspace.workspaces.map(item => [item.id, item.name])),
    readOnly: profile.aggregate,
    busy: create.isPending || viewer === null,
    runningError: catalog.error ? "Couldn't load running terminals." : null,
    onOpenNew: (destination: TerminalLauncherDestination) => {
      if (viewer === null || create.isPending) return;
      create.mutate({ destination, identity: viewer });
    },
    onAttach: (terminal: TerminalInfo) => {
      openItem(terminalOpenTarget(terminal), OPEN_FAILURE);
      onOpened();
    },
  };
}

/**
 * Whether the rail's Terminal item opens the launcher: only under All projects,
 * where a terminal needs a project first. Elsewhere the rail opens one directly.
 */
export function useTerminalLauncherEntry() {
  const [open, setOpen] = useState(false);
  const allProjects = useDataScope()?.kind === "all";
  // Leaving All projects closes the launcher for good, so returning never reopens it.
  const [wasAllProjects, setWasAllProjects] = useState(allProjects);
  if (wasAllProjects !== allProjects) {
    setWasAllProjects(allProjects);
    setOpen(false);
  }
  return {
    open: open && allProjects,
    setOpen,
    onOpen: allProjects ? () => setOpen(true) : undefined,
  };
}
