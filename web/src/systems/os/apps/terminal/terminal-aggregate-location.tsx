import { useEffect, useState } from "react";

import { AlertCircle } from "lucide-react";

import { Button, Empty, toast } from "@compozy/ui";
import {
  createTerminal,
  TerminalAggregateSkeleton,
  type TerminalInfo,
  type TerminalViewerIdentity,
} from "@/systems/terminal";
import { ownerDataScope, WINDOW_BINDING_SEARCH_KEY, windowBindingValue } from "@/systems/workspace";

import { TerminalLauncher } from "../../components/terminal-launcher";
import type { RoutingCoordinator } from "../../lib/routing-coordinator";
import {
  defaultTerminalLauncherDestination,
  runningTerminals,
  terminalLauncherDestinations,
} from "../../lib/terminal-launcher-model";

export interface TerminalAggregateLocationProps {
  windowId: string;
  catalog: {
    data?: TerminalInfo[];
    error: Error | null;
    isPending: boolean;
    refetch: () => unknown;
  };
  coordinator: Pick<RoutingCoordinator, "userRetarget">;
  /** The terminal this window was asked to show before it knew the terminal's project. */
  requestedId: string | null;
  workspaces: readonly { id: string; name: string; root_missing?: boolean }[];
  /** The project a new terminal starts in unless the operator picks another. */
  rememberedWorkspaceId?: string | null;
  /** The profile a new terminal opens in. */
  profile: string;
  /** All profiles is read-only: running rows open, nothing new starts. */
  readOnly: boolean;
  viewer: TerminalViewerIdentity | null;
}

/**
 * The Terminal app on the All projects desktop before it shows one terminal:
 * the launcher, in place. Opening a running terminal (or a new one in the
 * chosen project) binds this window to that terminal's project — the desktop
 * never switches.
 */
export function TerminalAggregateLocation({
  windowId,
  catalog,
  coordinator,
  requestedId,
  workspaces,
  rememberedWorkspaceId,
  profile,
  readOnly,
  viewer,
}: TerminalAggregateLocationProps) {
  const [creating, setCreating] = useState(false);
  const terminals = catalog.data ?? [];
  const destinations = terminalLauncherDestinations(workspaces);
  const defaultDestination = defaultTerminalLauncherDestination(
    destinations,
    rememberedWorkspaceId
  );

  const openBound = (terminalId: string, workspaceId: string) =>
    void coordinator.userRetarget(windowId, {
      app: "terminal",
      instanceKey: terminalId,
      route: {
        pathname: `/terminal/${encodeURIComponent(terminalId)}`,
        search: { [WINDOW_BINDING_SEARCH_KEY]: windowBindingValue(ownerDataScope(workspaceId)) },
      },
    });

  // A terminal opened by id without its project (a transcript block, a palette
  // row) binds as soon as the aggregate names that terminal's project.
  const requested = requestedId ? terminals.find(terminal => terminal.id === requestedId) : null;
  useEffect(() => {
    if (requested) openBound(requested.id, requested.workspace_id);
  });

  const startNew = (destination: string) => {
    if (!viewer || creating) return;
    setCreating(true);
    void createTerminal(destination, {}, { profile }, viewer)
      .then(
        terminal => openBound(terminal.id, terminal.workspace_id),
        (error: unknown) =>
          toast.error(error instanceof Error ? error.message : "Failed to open terminal")
      )
      .then(() => setCreating(false));
  };

  if (requested || (requestedId && catalog.isPending)) return <TerminalAggregateSkeleton />;
  // A terminal asked for by id cannot bind until the catalog names its project.
  if (requestedId && catalog.error && terminals.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          action={
            <Button
              onClick={() => void catalog.refetch()}
              size="sm"
              type="button"
              variant="secondary"
            >
              Retry
            </Button>
          }
          className="max-w-md"
          description={catalog.error.message}
          icon={AlertCircle}
          title="Couldn't load Terminal"
        />
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 justify-center overflow-y-auto px-6 pt-[12vh] pb-6">
      <TerminalLauncher
        busy={creating || viewer === null}
        className="w-full max-w-lg self-start rounded-lg border border-line"
        defaultDestination={defaultDestination}
        destinations={destinations}
        onAttach={terminal => openBound(terminal.id, terminal.workspace_id)}
        onOpenNew={startNew}
        projectNames={new Map(workspaces.map(workspace => [workspace.id, workspace.name]))}
        readOnly={readOnly}
        running={runningTerminals(terminals)}
        runningError={catalog.error ? "Couldn't load running terminals." : null}
      />
    </div>
  );
}
