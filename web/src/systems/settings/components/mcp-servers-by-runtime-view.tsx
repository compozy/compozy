import { AlertCircle, Cable, Plug, Plus, SearchX } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, Empty, ListingRowSkeleton, Pill, Skeleton, SkeletonRegion } from "@compozy/ui";

import {
  isRuntimeOffered,
  isRuntimeUnreadable,
  isMCPApprovalWaiting,
  MCP_FILTER_ALL,
  MCP_FILTER_COMPOZY,
  resolveRuntimeFilter,
  runtimeFilterOptions,
  searchRuntimeSnapshot,
} from "../lib/mcp-runtime-view-model";
import type { RuntimeMCPServer, RuntimeMCPSnapshot, SettingsMCPServerEntry } from "../types";
import { MCPRuntimeFilter } from "./mcp-runtime-filter";
import { MCPRuntimeGroup } from "./mcp-runtime-group";
import {
  MCPServersTable,
  MCPServersTableSkeleton,
  type MCPServersTableProps,
} from "./mcp-servers-table";

type LoadState = "pending" | "error" | "ready";

export interface MCPServersByRuntimeViewProps {
  compozy: {
    state: LoadState;
    errorMessage?: string;
    onRetry?: () => void;
    /** Every CompozyOS server of the lens (counts, empty state). */
    servers: readonly SettingsMCPServerEntry[];
    /** The servers left after the search. */
    visibleServers: SettingsMCPServerEntry[];
    table: Omit<MCPServersTableProps, "servers" | "heading">;
    onCreate: () => void;
  };
  runtimes: {
    state: LoadState;
    errorMessage?: string;
    onRetry?: () => void;
    snapshots: readonly RuntimeMCPSnapshot[];
  };
  query: string;
  onClearQuery: () => void;
  filter: string;
  onFilterChange: (filter: string) => void;
  onCopy: (snapshot: RuntimeMCPSnapshot, server: RuntimeMCPServer, trigger: HTMLElement) => void;
  onShowFile?: (path: string) => void;
  renderModeSwitch?: (snapshot: RuntimeMCPSnapshot) => ReactNode;
  /** Start every runtime group unfolded. */
  expandedGroups?: boolean;
}

/**
 * Settings › MCP servers by owner: the runtime filter, the CompozyOS group (today's table with the
 * approval cell), then one read-only group per runtime. Loading · error · empty · no match come
 * one at a time; a runtime read failure never hides the CompozyOS group. Pure: the management hook
 * (or a prototype) supplies data and callbacks.
 */
export function MCPServersByRuntimeView(props: MCPServersByRuntimeViewProps) {
  const { compozy, runtimes, query } = props;
  if (compozy.state === "pending") return <MCPServersByRuntimeSkeleton />;
  if (compozy.state === "error" && compozy.servers.length === 0) {
    return <MCPServersLoadError compozy={compozy} />;
  }
  const offered = runtimes.snapshots.filter(isRuntimeOffered);
  const searched = offered.map(snapshot => searchRuntimeSnapshot(snapshot, query));
  if (compozy.servers.length === 0 && runtimes.state === "ready" && offered.length === 0) {
    return <MCPServersNothingYet onCreate={compozy.onCreate} />;
  }
  if (runtimes.state !== "pending" && matchesNothing(query, compozy.visibleServers, searched)) {
    return <MCPServersNoMatch onClearQuery={props.onClearQuery} query={query} />;
  }
  return (
    <MCPServersByRuntimeGroups {...props} hasRuntimes={offered.length > 0} searched={searched} />
  );
}

function matchesNothing(
  query: string,
  visibleServers: readonly SettingsMCPServerEntry[],
  searched: readonly RuntimeMCPSnapshot[]
): boolean {
  return (
    query.trim() !== "" &&
    visibleServers.length === 0 &&
    searched.every(snapshot => snapshot.servers.length === 0)
  );
}

function MCPServersLoadError({ compozy }: Pick<MCPServersByRuntimeViewProps, "compozy">) {
  return (
    <Empty
      action={
        <Button onClick={compozy.onRetry} size="sm" type="button">
          Try again
        </Button>
      }
      cause={compozy.errorMessage}
      data-testid="settings-page-mcp-error"
      description="Check that CompozyOS is running, then try again."
      framed
      icon={AlertCircle}
      title="Couldn't load MCP servers"
      titleAs="h3"
    />
  );
}

function MCPServersNothingYet({ onCreate }: { onCreate: () => void }) {
  return (
    <Empty
      action={
        <Button
          data-testid="settings-page-mcp-empty-create"
          onClick={onCreate}
          size="sm"
          type="button"
        >
          <Plus aria-hidden="true" />
          Add MCP server
        </Button>
      }
      data-testid="settings-page-mcp-empty"
      description="Servers you add here, the ones your extensions add, and the ones your coding agents load all show up in this list."
      icon={Cable}
      title="No MCP servers yet"
    />
  );
}

function MCPServersNoMatch({ query, onClearQuery }: { query: string; onClearQuery: () => void }) {
  return (
    <Empty
      action={
        <Button onClick={onClearQuery} size="sm" type="button" variant="secondary">
          Clear search
        </Button>
      }
      data-testid="settings-page-mcp-query-empty"
      description={`Nothing matches "${query}".`}
      icon={SearchX}
      title="No MCP servers match"
    />
  );
}

function MCPRuntimesReadError({ runtimes }: Pick<MCPServersByRuntimeViewProps, "runtimes">) {
  return (
    <Empty
      action={
        <Button onClick={runtimes.onRetry} size="sm" type="button" variant="secondary">
          Try again
        </Button>
      }
      cause={runtimes.errorMessage}
      data-testid="settings-mcp-runtime-error"
      description="CompozyOS servers above are unaffected."
      framed
      icon={AlertCircle}
      title="Couldn't read your coding agents' MCP servers"
      titleAs="h3"
    />
  );
}

/** The populated layout: filter, the CompozyOS group, then the runtime groups the filter keeps. */
function MCPServersByRuntimeGroups({
  compozy,
  runtimes,
  hasRuntimes,
  searched,
  ...props
}: MCPServersByRuntimeViewProps & {
  hasRuntimes: boolean;
  searched: readonly RuntimeMCPSnapshot[];
}) {
  const options = runtimeFilterOptions(compozy.servers.length, runtimes.snapshots);
  const filter = resolveRuntimeFilter(props.filter, options);
  const showCompozy = filter === MCP_FILTER_ALL || filter === MCP_FILTER_COMPOZY;
  const groups = searched.filter(
    snapshot =>
      (filter === MCP_FILTER_ALL || filter === snapshot.runtime) &&
      (snapshot.servers.length > 0 || isRuntimeUnreadable(snapshot))
  );
  // The search can empty just the picked group while other groups still match: say so under the
  // filter instead of leaving the pills alone on the page.
  const compozyShown =
    showCompozy && (compozy.servers.length === 0 || compozy.visibleServers.length > 0);
  const noMatch =
    props.query.trim() !== "" &&
    (runtimes.state === "ready" || filter === MCP_FILTER_COMPOZY) &&
    !compozyShown &&
    groups.length === 0;
  return (
    <div className="flex flex-col gap-4">
      {hasRuntimes ? (
        <MCPRuntimeFilter onChange={props.onFilterChange} options={options} value={filter} />
      ) : null}
      {noMatch ? <MCPServersNoMatch onClearQuery={props.onClearQuery} query={props.query} /> : null}
      {showCompozy ? <CompozyGroup {...compozy} /> : null}
      {runtimes.state === "pending" && filter === MCP_FILTER_ALL ? <RuntimeGroupsSkeleton /> : null}
      {runtimes.state === "error" ? <MCPRuntimesReadError runtimes={runtimes} /> : null}
      {groups.map(snapshot => (
        <MCPRuntimeGroup
          collapsible={filter === MCP_FILTER_ALL}
          defaultExpanded={props.expandedGroups}
          key={snapshot.runtime}
          onCopy={props.onCopy}
          onShowFile={props.onShowFile}
          renderModeSwitch={props.renderModeSwitch}
          snapshot={snapshot}
        />
      ))}
    </div>
  );
}

function CompozyGroup({
  servers,
  visibleServers,
  table,
  onCreate,
}: MCPServersByRuntimeViewProps["compozy"]) {
  if (servers.length === 0) {
    return (
      <Empty
        action={
          <Button
            data-testid="settings-page-mcp-empty-create"
            onClick={onCreate}
            size="sm"
            type="button"
          >
            <Plus aria-hidden="true" />
            Add MCP server
          </Button>
        }
        data-testid="settings-mcp-compozy-empty"
        description="Add one, or copy a server your coding agent already uses."
        framed
        icon={Cable}
        title="No CompozyOS servers yet"
        titleAs="h3"
      />
    );
  }
  if (visibleServers.length === 0) return null;
  const waiting = servers.filter(isMCPApprovalWaiting).length;
  return (
    <MCPServersTable
      {...table}
      heading={<CompozyGroupHeading count={servers.length} waiting={waiting} />}
      servers={visibleServers}
    />
  );
}

function CompozyGroupHeading({ count, waiting }: { count: number; waiting: number }) {
  return (
    <header className="flex min-h-12 items-center gap-3 border-b border-line px-4 py-2.5">
      <span
        aria-hidden="true"
        className="grid size-7 place-items-center rounded-md bg-brand-tint text-brand-strong"
      >
        <Plug className="size-4" />
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="flex items-center gap-2">
          <span className="text-small-body font-medium text-fg">CompozyOS</span>
          <Pill mono>{count}</Pill>
        </span>
        <span className="truncate text-eyebrow text-subtle">
          {waiting > 0
            ? `Every CompozyOS session can use these · ${waiting} waiting for approval`
            : "Every CompozyOS session can use these from its next message"}
        </span>
      </div>
    </header>
  );
}

const RUNTIME_SKELETON_GROUPS = ["first", "second"] as const;

function RuntimeGroupsSkeleton() {
  return (
    <>
      {RUNTIME_SKELETON_GROUPS.map(id => (
        <div aria-hidden="true" className="overflow-hidden rounded-lg bg-card shadow-card" key={id}>
          <div className="flex min-h-12 items-center gap-3 border-b border-line px-4">
            <Skeleton className="size-7 rounded-md" />
            <Skeleton className="h-3.5 w-32 rounded-pill" />
          </div>
          <ListingRowSkeleton count={2} trail="pill" />
        </div>
      ))}
    </>
  );
}

/** The CompozyOS table and two runtime groups in skeleton form. */
export function MCPServersByRuntimeSkeleton() {
  return (
    <SkeletonRegion
      className="flex flex-col gap-4"
      data-testid="settings-page-mcp-loading"
      label="Loading MCP servers"
    >
      <MCPServersTableSkeleton />
      <RuntimeGroupsSkeleton />
    </SkeletonRegion>
  );
}
