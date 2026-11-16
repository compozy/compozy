import { Plug, Settings2 } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Button,
  InputGroupButton,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  StatusDot,
} from "@compozy/ui";

import type { SessionMCPResponse, SessionMCPServerTarget } from "../adapters/session-mcp-api";
import {
  sessionMCPControlLabel,
  sessionMCPControlView,
  sessionMCPHeaderNote,
  sessionMCPReloadFailed,
  type SessionMCPSummary,
} from "../lib/session-mcp-view";
import {
  MCPComposerLoadError,
  MCPComposerLoadingRows,
  MCPComposerRows,
  type MCPComposerRowActionHandler,
} from "./mcp-composer-rows";

export interface MCPComposerControlProps {
  /** The session payload's `mcp_summary`; the control is absent without one. */
  summary: SessionMCPSummary | null;
  /** The session MCP read, or `null` before it answered. */
  data: SessionMCPResponse | null;
  loading?: boolean;
  /** The latest read failed; rows from an earlier read stay, marked as possibly out of date. */
  failed?: boolean;
  onRetry?: () => void;
  /** A row's Sign in / Review / Not working; `trigger` is the button its panel opens beside. */
  onAction?: MCPComposerRowActionHandler;
  /** The server (name and owner) whose action is in flight. */
  actingServer?: SessionMCPServerTarget | null;
  /** Actions wait until their target can be resolved. */
  actionsDisabled?: boolean;
  /** The server (name and owner) whose row panel is open beside it. */
  activeServer?: SessionMCPServerTarget | null;
  onManage?: () => void;
  /** The popover opened or closed; the owner reads the session's servers while it is open. */
  onOpenChange?: (open: boolean) => void;
  defaultOpen?: boolean;
  /** The coding agent's own servers start folded; a story can pin them open. */
  runtimeOpen?: boolean;
  /** Row panels rendered inside the popover, so pressing them keeps the popover open. */
  rowPanel?: ReactNode;
}

function MCPComposerHeader({
  note,
  onManage,
}: {
  note: ReturnType<typeof sessionMCPHeaderNote>;
  onManage?: () => void;
}) {
  return (
    <div className="flex items-center gap-2 border-b border-line-soft py-2 pr-2 pl-3.5">
      <Plug aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
      <PopoverTitle className="shrink-0 text-small-body font-medium whitespace-nowrap text-fg-strong">
        MCP servers
      </PopoverTitle>
      {note === "stale" ? (
        <span className="truncate text-meta text-faint">May be out of date</span>
      ) : null}
      {note === "pending" ? (
        <span className="flex min-w-0 items-center gap-1.5 text-meta text-muted-foreground">
          <StatusDot tone="info" size="sm" />
          <span className="truncate">Updates on your next message</span>
        </span>
      ) : null}
      {onManage ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label="Manage MCP servers in Settings"
          title="Manage in Settings"
          className="ml-auto text-muted-foreground"
          onClick={onManage}
        >
          <Settings2 />
        </Button>
      ) : null}
    </div>
  );
}

function MCPComposerBody({
  summary,
  data,
  loading = false,
  failed = false,
  onRetry,
  runtimeOpen,
  ...rows
}: Omit<MCPComposerControlProps, "onOpenChange" | "onManage" | "defaultOpen" | "rowPanel"> & {
  summary: SessionMCPSummary;
}) {
  if (data === null) {
    // Without an answer there is nothing to call ready: a failed read says so, never a count.
    if (failed) return <MCPComposerLoadError onRetry={onRetry} />;
    if (!loading) return null;
    return <MCPComposerLoadingRows count={summary.usable + summary.attention} />;
  }
  return (
    <MCPComposerRows
      {...rows}
      view={sessionMCPControlView(data)}
      runtimeMCP={data.runtime_servers.runtime_mcp}
      runtimeOpen={runtimeOpen}
    />
  );
}

/**
 * The composer's MCP control. The session payload's summary drives the closed trigger (an
 * attention dot top-right, a pending change bottom-right); the popover lists the CompozyOS servers
 * this session can use with the one action each needs, those its parent turned off, and — folded —
 * the servers the coding agent loads on its own. Row actions open a panel beside the row, never a
 * modal. Absent when the session has no servers.
 */
export function MCPComposerControl({
  onOpenChange,
  onManage,
  defaultOpen,
  rowPanel,
  ...body
}: MCPComposerControlProps) {
  const { summary, data, failed = false } = body;
  if (summary === null) return null;
  const reloadFailed = sessionMCPReloadFailed(summary, data);
  const warn = summary.attention > 0 || reloadFailed;
  const label = sessionMCPControlLabel(summary, { reloadFailed });
  const note = sessionMCPHeaderNote({
    stale: failed && data !== null,
    pendingReload: data?.pending_reload ?? summary.pending_reload,
    reloadFailed,
  });
  return (
    <Popover defaultOpen={defaultOpen} onOpenChange={open => onOpenChange?.(open)}>
      <PopoverTrigger
        render={
          <InputGroupButton
            type="button"
            size="tool-icon"
            aria-label={label}
            title={label}
            data-testid="composer-mcp-button"
            className="relative shrink-0 data-[popup-open]:bg-surface-2"
          />
        }
      >
        <Plug aria-hidden="true" />
        {warn ? (
          <span
            aria-hidden="true"
            data-testid="composer-mcp-attention"
            className="absolute top-1 right-1 size-1.5 rounded-full bg-warning"
          />
        ) : null}
        {summary.pending_reload ? (
          <span
            aria-hidden="true"
            data-testid="composer-mcp-pending"
            className="absolute right-1 bottom-1 size-1.5 rounded-full bg-info"
          />
        ) : null}
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        className="w-80 max-w-[calc(100vw-2rem)] gap-0 p-0"
        data-testid="composer-mcp-popover"
      >
        <MCPComposerHeader note={note} onManage={onManage} />
        <div className="flex flex-col gap-1 p-1.5">
          <MCPComposerBody {...body} summary={summary} />
        </div>
        {rowPanel}
      </PopoverContent>
    </Popover>
  );
}
