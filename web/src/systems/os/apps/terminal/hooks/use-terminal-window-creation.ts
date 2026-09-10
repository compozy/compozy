import {
  useMutation,
  useMutationState,
  useQueryClient,
  type Mutation,
} from "@tanstack/react-query";

import { toast } from "@compozy/ui";
import {
  createTerminal,
  terminalKeys,
  type TerminalProfileQueryScope,
  type TerminalQueryScope,
  type TerminalViewerIdentity,
} from "@/systems/terminal";

import type { RoutingCoordinator } from "../../../lib/routing-coordinator";
import { terminalWindowCreateKey } from "../../../lib/terminal-window-close";
import { windowManagerStore } from "../../../stores/window-manager-store";

interface TerminalWindowCreationScope {
  windowId: string;
  workspaceId: string;
  catalogScope: TerminalQueryScope;
  destinationScope: TerminalProfileQueryScope;
  coordinator: RoutingCoordinator;
}

interface TerminalWindowCreation extends TerminalWindowCreationScope {
  identity: TerminalViewerIdentity;
  binding: ReturnType<typeof windowManagerStore.getSnapshot>["context"]["binding"];
}

/** A pending creation retains its request scope even when the shell rebinds. */
export function useTerminalWindowCreation(scope: TerminalWindowCreationScope) {
  const queryClient = useQueryClient();
  const completed = useMutationState({
    filters: {
      mutationKey: terminalWindowCreateKey(scope.windowId),
      exact: true,
      status: "success",
    },
    select: (
      candidate: Mutation<Awaited<ReturnType<typeof createTerminal>>, Error, TerminalWindowCreation>
    ) => {
      const { data, variables } = candidate.state;
      return data && variables
        ? {
            workspaceId: variables.workspaceId,
            profileKey: variables.catalogScope.key.profileKey,
            destinationProfile: variables.destinationScope.params.profile,
            terminal: data,
          }
        : null;
    },
  });
  const mutation = useMutation({
    mutationKey: terminalWindowCreateKey(scope.windowId),
    mutationFn: (input: TerminalWindowCreation) =>
      createTerminal(input.workspaceId, {}, input.destinationScope.params, input.identity),
    onSuccess: async (terminal, input) => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: terminalKeys.catalog(input.catalogScope.key),
          exact: true,
        }),
        queryClient.invalidateQueries({
          queryKey: terminalKeys.inputRequests(input.catalogScope.key),
          exact: true,
        }),
        queryClient.invalidateQueries({
          queryKey: terminalKeys.journalScope(input.catalogScope.key),
        }),
      ]);
      // The coordinator targets the currently bound desktop. A terminal created
      // for a previous binding remains available in its owner's catalog instead.
      if (
        input.binding === null ||
        windowManagerStore.getSnapshot().context.binding !== input.binding
      ) {
        return;
      }
      await input.coordinator.userRetarget(input.windowId, {
        app: "terminal",
        instanceKey: terminal.id,
        route: { pathname: `/terminal/${encodeURIComponent(terminal.id)}`, search: {} },
      });
    },
    onError: error =>
      toast.error(error instanceof Error ? error.message : "Failed to open terminal"),
  });
  const capture = (identity: TerminalViewerIdentity): TerminalWindowCreation => ({
    ...scope,
    identity,
    binding: windowManagerStore.getSnapshot().context.binding,
  });
  return {
    completedTerminal:
      completed
        .filter(
          candidate =>
            candidate?.workspaceId === scope.workspaceId &&
            candidate.profileKey === scope.catalogScope.key.profileKey &&
            candidate.destinationProfile === scope.destinationScope.params.profile
        )
        .at(-1)?.terminal ?? null,
    mutate: (identity: TerminalViewerIdentity) => mutation.mutate(capture(identity)),
    mutateAsync: (identity: TerminalViewerIdentity) => mutation.mutateAsync(capture(identity)),
    isPending: mutation.isPending,
  };
}
