import { useEffect, useState } from "react";
import { AlertCircle, TerminalSquare } from "lucide-react";

import {
  BlockLoading,
  Button,
  Empty,
  Item,
  ItemActions,
  ItemContent,
  ItemTitle,
  toast,
} from "@compozy/ui";
import {
  createTerminal,
  terminalDisplayTitle,
  type TerminalInfo,
  type TerminalViewerIdentity,
} from "@/systems/terminal";
import {
  CreateDestinationPicker,
  DESTINATION_COPY,
  ownerDataScope,
  USER_SCOPE_SELECTOR,
  WINDOW_BINDING_SEARCH_KEY,
  WorkspaceBadge,
  workspaceBadgeIdentity,
  workspaceMonogram,
  windowBindingValue,
  type CreateDestinationOption,
} from "@/systems/workspace";

import type { RoutingCoordinator } from "../../lib/routing-coordinator";

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
  workspaces: readonly { id: string; name: string }[];
  /** The profile a new terminal opens in. */
  profile: string;
  /** All profiles is read-only: rows open, nothing new starts. */
  readOnly: boolean;
  viewer: TerminalViewerIdentity | null;
}

function destinationOptions(
  workspaces: readonly { id: string; name: string }[]
): CreateDestinationOption[] {
  return [
    ...workspaces.map(workspace => ({
      value: workspace.id,
      label: workspace.name,
      monogram: workspaceMonogram(workspace.name),
    })),
    {
      value: USER_SCOPE_SELECTOR,
      label: DESTINATION_COPY.user,
      monogram: DESTINATION_COPY.userMonogram,
    },
  ];
}

/**
 * The Terminal app on the All projects desktop before it shows one terminal:
 * every project's terminals, each named by its project. Opening one (or a
 * new one in the chosen project) binds this window to that terminal's project
 * in place — the desktop never switches.
 */
export function TerminalAggregateLocation({
  windowId,
  catalog,
  coordinator,
  requestedId,
  workspaces,
  profile,
  readOnly,
  viewer,
}: TerminalAggregateLocationProps) {
  const options = destinationOptions(workspaces);
  const [target, setTarget] = useState(options[0]?.value ?? USER_SCOPE_SELECTOR);
  const [creating, setCreating] = useState(false);
  const names = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  const terminals = catalog.data ?? [];

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

  const startNew = async () => {
    if (!viewer || creating) return;
    setCreating(true);
    try {
      const terminal = await createTerminal(target, {}, { profile }, viewer);
      openBound(terminal.id, terminal.workspace_id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to open terminal");
    } finally {
      setCreating(false);
    }
  };

  if (catalog.isPending || requested) {
    return <BlockLoading className="flex-1" label="Loading terminals" surface="bare" />;
  }
  if (catalog.error && terminals.length === 0) {
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

  const newTerminal = readOnly ? null : (
    <div
      className="flex flex-wrap items-center gap-2 text-small-body text-muted"
      data-testid="terminal-aggregate-new"
    >
      <span>
        New terminal in{" "}
        <CreateDestinationPicker
          axis="Project"
          onChange={setTarget}
          options={options}
          testId="terminal-aggregate-project"
          value={target}
        />
      </span>
      <Button
        disabled={!viewer || creating}
        onClick={() => void startNew()}
        size="sm"
        type="button"
        variant="secondary"
      >
        Open
      </Button>
    </div>
  );

  if (terminals.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          action={newTerminal}
          data-testid="terminal-aggregate-empty"
          description="A terminal runs commands on this computer. Agents can open one too."
          icon={TerminalSquare}
          title="No terminals yet"
        />
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">
      {catalog.error ? (
        <p className="text-small-body text-danger" role="alert">
          Couldn&apos;t refresh terminals.{" "}
          <button className="underline" onClick={() => void catalog.refetch()} type="button">
            Retry
          </button>
        </p>
      ) : null}
      <div className="flex flex-col" data-testid="terminal-aggregate-list">
        {terminals.map(terminal => (
          <Item
            as="button"
            className="rounded-md"
            data-testid={`terminal-aggregate-row-${terminal.id}`}
            key={terminal.id}
            onClick={() => openBound(terminal.id, terminal.workspace_id)}
            size="sm"
          >
            <ItemContent className="flex-row items-center gap-2">
              <WorkspaceBadge identity={workspaceBadgeIdentity(terminal.workspace_id, names)} />
              <ItemTitle className="min-w-0 truncate font-normal">
                {terminalDisplayTitle(terminal)}
              </ItemTitle>
            </ItemContent>
            <ItemActions className="text-micro text-subtle">
              {readOnly ? <span>{terminal.profile_name}</span> : null}
              <span>{terminal.state}</span>
            </ItemActions>
          </Item>
        ))}
      </div>
      {newTerminal}
    </div>
  );
}
