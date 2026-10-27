import { useEffect, useState } from "react";
import { AlertCircle, ListFilter, TerminalSquare } from "lucide-react";

import {
  Button,
  Empty,
  FiltersWithSearch,
  Item,
  ItemActions,
  ItemContent,
  ItemTitle,
  toast,
} from "@compozy/ui";
import {
  createTerminal,
  TerminalAggregateSkeleton,
  terminalDisplayTitle,
  type TerminalInfo,
  type TerminalViewerIdentity,
} from "@/systems/terminal";
import {
  CreateDestinationPicker,
  DESTINATION_COPY,
  ownerDataScope,
  projectFilterChip,
  projectFilterValuesFromChips,
  USER_SCOPE_SELECTOR,
  WINDOW_BINDING_SEARCH_KEY,
  WorkspaceBadge,
  workspaceBadgeIdentity,
  windowBindingValue,
  type CreateDestinationOption,
  type ProjectFilterModel,
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
  /** The listing Project filter: which projects (and User) the list reads. */
  projectFilter: Pick<ProjectFilterModel, "field" | "values" | "setValues">;
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
    })),
    { value: USER_SCOPE_SELECTOR, label: DESTINATION_COPY.user },
  ];
}

function TerminalAggregateProjectFilter({
  projectFilter,
}: {
  projectFilter: TerminalAggregateLocationProps["projectFilter"];
}) {
  if (!projectFilter.field) return null;
  const chip = projectFilterChip(projectFilter.values);
  return (
    <div data-testid="terminal-aggregate-filters">
      <FiltersWithSearch<string>
        allowMultiple={false}
        fields={[projectFilter.field]}
        filters={chip ? [chip] : []}
        onChange={next => projectFilter.setValues(projectFilterValuesFromChips(next))}
        size="sm"
        trigger={
          <Button aria-label="Filter terminals" size="sm" type="button" variant="ghost">
            <ListFilter aria-hidden="true" data-icon="inline-start" />
            Filter
          </Button>
        }
      />
    </div>
  );
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
  projectFilter,
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

  const startNew = () => {
    if (!viewer || creating) return;
    setCreating(true);
    void createTerminal(target, {}, { profile }, viewer)
      .then(
        terminal => openBound(terminal.id, terminal.workspace_id),
        (error: unknown) =>
          toast.error(error instanceof Error ? error.message : "Failed to open terminal")
      )
      .then(() => setCreating(false));
  };

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

  if (catalog.isPending || requested) {
    // The filter and the new-terminal row do not depend on the list, so they stay live and in
    // place (a filter edit re-reads the list without closing its own editor).
    return (
      <TerminalAggregateSkeleton
        filter={<TerminalAggregateProjectFilter projectFilter={projectFilter} />}
        footer={newTerminal}
      />
    );
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

  const filtered = projectFilter.values.length > 0;
  if (terminals.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 flex-col px-5 py-4">
        <TerminalAggregateProjectFilter projectFilter={projectFilter} />
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
          <Empty
            action={newTerminal}
            data-testid="terminal-aggregate-empty"
            description={
              filtered
                ? "No terminal runs in the selected projects."
                : "A terminal runs commands on this computer. Agents can open one too."
            }
            icon={TerminalSquare}
            title={filtered ? "No matching terminals" : "No terminals yet"}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">
      <TerminalAggregateProjectFilter projectFilter={projectFilter} />
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
