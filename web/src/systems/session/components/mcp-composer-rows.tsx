import { ChevronRight } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  cn,
  Disclosure,
  SkeletonRegion,
  SkeletonText,
  StatusDot,
} from "@compozy/ui";

import { ProviderLogo } from "@/systems/settings/components/provider-logo";

import type { SessionMCPServerRow, SessionRuntimeMCPRow } from "../adapters/session-mcp-api";
import {
  sessionMCPRuntimeProvider,
  sessionMCPRuntimeStateView,
  sessionMCPStateView,
  sessionMCPToolCount,
  type SessionMCPControlView,
  type SessionMCPRowAction,
} from "../lib/session-mcp-view";

// One row geometry for CompozyOS rows, runtime rows and their placeholders. Rows line up with the
// disclosure arrows, so nothing indents deeper than the section toggles.
const ROW = "flex min-h-8 items-center gap-2 rounded-md px-2 py-1";
const NAME = "min-w-0 flex-1 truncate text-small-body text-fg";
const SIDE_TEXT = "shrink-0 text-meta text-faint tabular-nums";
const PLACEHOLDER_WIDTHS = ["w-28", "w-20", "w-32"] as const;

export type MCPComposerRowActionHandler = (
  row: SessionMCPServerRow,
  action: SessionMCPRowAction,
  trigger: HTMLElement
) => void;

export interface MCPComposerRowsProps {
  view: SessionMCPControlView;
  onAction?: MCPComposerRowActionHandler;
  /** The server whose action is in flight. */
  actingName?: string | null;
  /** Actions wait until their target can be resolved. */
  actionsDisabled?: boolean;
  /** The server whose row panel is open beside it. */
  activeName?: string | null;
  /** The coding agent's own servers start folded. */
  runtimeOpen?: boolean;
}

function RowAction({
  row,
  action,
  disabled,
  onAction,
}: {
  row: SessionMCPServerRow;
  action: SessionMCPRowAction;
  disabled: boolean;
  onAction: MCPComposerRowActionHandler;
}) {
  const press = (event: { currentTarget: HTMLElement }) =>
    onAction(row, action, event.currentTarget);
  if (action === "check") {
    return (
      <Button
        type="button"
        size="xs"
        variant="quiet"
        className="shrink-0 text-muted-foreground"
        aria-label={`${row.name} is not working`}
        disabled={disabled}
        onClick={press}
      >
        Not working
        <ChevronRight data-icon="inline-end" />
      </Button>
    );
  }
  const label = action === "sign_in" ? "Sign in" : "Review";
  return (
    <Button
      type="button"
      size="xs"
      variant="neutral"
      className="shrink-0"
      aria-label={`${label} ${row.name}`}
      disabled={disabled}
      onClick={press}
    >
      {label}
    </Button>
  );
}

function CompozyRow({
  row,
  onAction,
  acting,
  active,
}: {
  row: SessionMCPServerRow;
  onAction?: MCPComposerRowActionHandler;
  acting: boolean;
  active: boolean;
}) {
  const view = sessionMCPStateView(row.state);
  let side: ReactNode = <span className={SIDE_TEXT}>{view.label}</span>;
  if (row.state === "ready") {
    side = <span className={SIDE_TEXT}>{sessionMCPToolCount(row.tool_count)}</span>;
  } else if (row.state === "applies_next_message") {
    side = (
      <span className="flex shrink-0 items-center gap-1.5 text-meta text-muted-foreground">
        <StatusDot tone="info" size="sm" />
        {view.label}
      </span>
    );
  } else if (view.action) {
    side = onAction ? (
      <RowAction row={row} action={view.action} disabled={acting} onAction={onAction} />
    ) : (
      <span className={SIDE_TEXT}>{view.label}</span>
    );
  }
  return (
    <li
      className={cn(ROW, active && "bg-surface-2", view.quiet && "opacity-60")}
      data-testid={`composer-mcp-${row.name}`}
      data-state={row.state}
    >
      {view.attention ? <StatusDot tone={view.attention} size="sm" /> : null}
      <span className={NAME}>{row.name}</span>
      {side}
    </li>
  );
}

function RuntimeSection({
  view,
  runtimeMCP,
  defaultOpen,
}: {
  view: SessionMCPControlView;
  runtimeMCP: string;
  defaultOpen: boolean;
}) {
  const runtime = view.runtimeName;
  const rows = view.runtimeServers;
  const off = runtimeMCP === "off";
  const waiting = rows.some(row => row.state === "waiting_in_runtime");
  return (
    <Disclosure
      defaultOpen={defaultOpen}
      size="md"
      className="border-t border-line-soft pt-1"
      triggerProps={{
        className: "w-full rounded-md px-2 py-1.5 hover:bg-surface-2",
        "data-testid": "composer-mcp-runtime-toggle",
      }}
      contentProps={{ className: "pt-0.5" }}
      label={
        <span className="flex min-w-0 flex-1 items-center gap-2">
          <ProviderLogo provider={sessionMCPRuntimeProvider(view.runtimeId)} className="size-3.5" />
          <span className="truncate text-meta font-medium text-muted-foreground">
            {runtime}'s own servers
          </span>
          {waiting ? <StatusDot tone="warning" size="sm" label="One needs you" /> : null}
          <span className={cn(SIDE_TEXT, "ml-auto")}>{off ? "Off" : rows.length}</span>
        </span>
      }
    >
      <ul className="flex flex-col">
        {rows.map(row => (
          <RuntimeRow key={row.name} row={row} runtime={runtime} />
        ))}
      </ul>
      <p className="px-2 pt-1.5 pb-1 text-meta text-faint">
        {off
          ? "Turned off for CompozyOS sessions. Turn them on in Settings."
          : `${runtime} loads these by itself. Change them in ${runtime}.`}
      </p>
    </Disclosure>
  );
}

function RuntimeRow({ row, runtime }: { row: SessionRuntimeMCPRow; runtime: string }) {
  const view = sessionMCPRuntimeStateView(row, runtime);
  return (
    <li
      className={cn(ROW, view?.quiet && "opacity-60")}
      data-testid={`composer-mcp-${row.name}`}
      data-state={row.state}
    >
      <span className={NAME}>{row.name}</span>
      {view?.attention ? (
        <span className="flex shrink-0 items-center gap-1.5 text-meta text-muted-foreground">
          <StatusDot tone={view.attention} size="sm" />
          {view.label}
        </span>
      ) : view ? (
        <span className={SIDE_TEXT}>{view.label}</span>
      ) : null}
    </li>
  );
}

/** Rows of a popover whose session MCP read answered. */
export function MCPComposerRows({
  view,
  runtimeMCP,
  onAction,
  actingName = null,
  actionsDisabled = false,
  activeName = null,
  runtimeOpen = false,
}: MCPComposerRowsProps & { runtimeMCP: string }) {
  const runtime = view.runtimeName;
  return (
    <>
      {view.reloadFailed ? (
        <Alert variant="warning" className="mx-0.5">
          <AlertDescription>
            {runtime} couldn't start with the new servers. Your message wasn't sent; send it again
            to retry.
          </AlertDescription>
        </Alert>
      ) : null}
      {view.compozy.length > 0 ? (
        <ul className="flex max-h-80 flex-col overflow-y-auto">
          {view.compozy.map(row => (
            <CompozyRow
              key={`${row.owner}:${row.name}`}
              row={row}
              onAction={onAction}
              acting={actionsDisabled || actingName === row.name}
              active={activeName === row.name}
            />
          ))}
        </ul>
      ) : null}
      {view.unavailable.length > 0 ? (
        <Disclosure
          label={`Off in this session (${view.unavailable.length})`}
          className="px-2"
          contentProps={{ className: "pt-0.5" }}
        >
          <p className="pb-1 text-meta text-faint">
            The session that started this one limits them.
          </p>
          <ul className="-mx-2 flex flex-col">
            {view.unavailable.map(row => (
              <CompozyRow
                key={`${row.owner}:${row.name}`}
                row={row}
                acting={false}
                active={false}
              />
            ))}
          </ul>
        </Disclosure>
      ) : null}
      {view.runtimeServers.length > 0 ? (
        <RuntimeSection view={view} runtimeMCP={runtimeMCP} defaultOpen={runtimeOpen} />
      ) : null}
    </>
  );
}

/** Placeholder rows while the first read is in flight, in the live rows' geometry. */
export function MCPComposerLoadingRows({ count }: { count: number }) {
  const rows = PLACEHOLDER_WIDTHS.slice(0, Math.min(Math.max(count, 1), PLACEHOLDER_WIDTHS.length));
  return (
    <SkeletonRegion label="Loading MCP servers" data-testid="composer-mcp-loading">
      <ul aria-hidden="true" className="flex flex-col">
        {rows.map(width => (
          <li key={width} className={cn(ROW, "justify-between")}>
            <SkeletonText className={cn("text-small-body", width)} />
            <SkeletonText className="w-10 text-meta" />
          </li>
        ))}
      </ul>
    </SkeletonRegion>
  );
}

/** The first read failed: nothing is called ready, and the read can be retried. */
export function MCPComposerLoadError({ onRetry }: { onRetry?: () => void }) {
  return (
    <div className="flex items-center justify-between gap-2 px-2 py-1">
      <p className="text-meta text-muted-foreground">Couldn't load MCP servers.</p>
      {onRetry ? (
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}
