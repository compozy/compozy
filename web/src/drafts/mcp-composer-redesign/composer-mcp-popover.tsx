/**
 * Proposed composer MCP popover. Calm by default: plain server names, no pill on a healthy row,
 * actions only where the person is needed, and the coding agent's own servers folded behind one
 * closed row. Row actions open a small panel beside the popover (nested popover), never a modal.
 */
import { ChevronRight, Plug, Settings2 } from "@compozy/ui/icons";
import { useRef, type ReactNode, type RefObject } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  cn,
  Disclosure,
  InputGroupButton,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  StatusDot,
} from "@compozy/ui";

import type {
  SessionMCPResponse,
  SessionMCPServerRow,
  SessionRuntimeMCPRow,
} from "@/systems/session/adapters/session-mcp-api";
import { ProviderLogo } from "@/systems/settings/components/provider-logo";
import { sessionMCPRuntimeProvider } from "@/systems/session/lib/session-mcp-view";

import { MCPAllowPanel, MCPIssuePanel } from "../mcp-redesign-shared/mcp-row-panels";
import {
  MCP_PANEL_CLASS,
  MCPSignInPanel,
  type MCPSignInPhase,
  type MCPSignInScope,
} from "../mcp-redesign-shared/mcp-sign-in-panel";

export type ComposerNested =
  | { kind: "sign-in"; server: string; phase: MCPSignInPhase; error?: string; scopes?: MCPSignInScope[] }
  | { kind: "allow"; server: string }
  | { kind: "issue"; server: string };

export interface ComposerMCPPopoverProps {
  data: SessionMCPResponse | null;
  open?: boolean;
  loading?: boolean;
  stale?: boolean;
  /** The coding agent's own servers start folded; a state can pin them open. */
  runtimeOpen?: boolean;
  /** The row panel pinned open beside the popover. */
  nested?: ComposerNested;
}

const ROW = "group/row flex min-h-8 items-center gap-2 rounded-md px-2 py-1";
const NAME = "min-w-0 flex-1 truncate text-small-body text-fg";
const SIDE_TEXT = "shrink-0 text-meta text-faint tabular-nums";

const ATTENTION = new Set(["needs_sign_in", "waiting_approval", "failed"]);

function RowPanel({
  anchor,
  open,
  trigger,
  children,
}: {
  anchor: RefObject<HTMLLIElement | null>;
  open: boolean;
  trigger: ReactNode;
  children: ReactNode;
}) {
  return (
    <Popover defaultOpen={open} modal={false}>
      {trigger}
      <PopoverContent
        side="right"
        align="start"
        sideOffset={14}
        alignOffset={-6}
        anchor={anchor}
        className={MCP_PANEL_CLASS}
      >
        {children}
      </PopoverContent>
    </Popover>
  );
}

function CompozyRow({ row, nested }: { row: SessionMCPServerRow; nested?: ComposerNested }) {
  const ref = useRef<HTMLLIElement>(null);
  const pinned = nested?.server === row.name ? nested : undefined;
  const active = pinned !== undefined;
  let side: ReactNode = null;
  if (row.state === "ready") {
    side = <span className={SIDE_TEXT}>{row.tool_count} tools</span>;
  } else if (row.state === "applies_next_message") {
    side = (
      <span className="flex shrink-0 items-center gap-1.5 text-meta text-muted-foreground">
        <StatusDot tone="info" size="sm" />
        Next message
      </span>
    );
  } else if (row.state === "needs_sign_in") {
    side = (
      <RowPanel
        anchor={ref}
        open={pinned?.kind === "sign-in"}
        trigger={
          <PopoverTrigger
            render={<Button type="button" size="xs" variant="neutral" aria-label={`Sign in to ${row.name}`} />}
          >
            Sign in
          </PopoverTrigger>
        }
      >
        <MCPSignInPanel
          server={row.name}
          phase={pinned?.kind === "sign-in" ? pinned.phase : "waiting"}
          error={pinned?.kind === "sign-in" ? pinned.error : undefined}
          scopes={pinned?.kind === "sign-in" ? pinned.scopes : undefined}
          toolCount={18}
          toolPreview={["Revenue report", "Orders", "Campaigns", "CRM deals"]}
        />
      </RowPanel>
    );
  } else if (row.state === "waiting_approval") {
    side = (
      <RowPanel
        anchor={ref}
        open={pinned?.kind === "allow"}
        trigger={
          <PopoverTrigger render={<Button type="button" size="xs" variant="neutral" />}>
            Review
          </PopoverTrigger>
        }
      >
        <MCPAllowPanel server={row.name} />
      </RowPanel>
    );
  } else if (row.state === "failed") {
    side = (
      <RowPanel
        anchor={ref}
        open={pinned?.kind === "issue"}
        trigger={
          <PopoverTrigger
            render={
              <Button type="button" size="xs" variant="quiet" className="text-muted-foreground" />
            }
          >
            Not working
            <ChevronRight data-icon="inline-end" />
          </PopoverTrigger>
        }
      >
        <MCPIssuePanel server={row.name} reason={row.reason} />
      </RowPanel>
    );
  } else if (row.state === "not_available") {
    side = <span className={SIDE_TEXT}>Off here</span>;
  }
  return (
    <li
      ref={ref}
      className={cn(ROW, active && "bg-surface-2", row.state === "not_available" && "opacity-60")}
      data-state={row.state}
    >
      {ATTENTION.has(row.state) ? (
        <StatusDot tone={row.state === "failed" ? "destructive" : "warning"} size="sm" />
      ) : null}
      <span className={NAME}>{row.name}</span>
      {side}
    </li>
  );
}

function runtimeSide(row: SessionRuntimeMCPRow, runtime: string): ReactNode {
  switch (row.state) {
    case "loaded_by_runtime":
      return null;
    case "waiting_in_runtime":
      return (
        <span className="flex shrink-0 items-center gap-1.5 text-meta text-muted-foreground">
          <StatusDot tone="warning" size="sm" />
          Approve in {runtime}
        </span>
      );
    default:
      return <span className={SIDE_TEXT}>Off</span>;
  }
}

function RuntimeSection({
  data,
  open,
}: {
  data: SessionMCPResponse;
  open: boolean;
}) {
  const runtime = data.runtime_display_name;
  const rows = data.runtime_servers.servers;
  const off = data.runtime_servers.runtime_mcp === "off";
  const waiting = rows.some(row => row.state === "waiting_in_runtime");
  return (
    <Disclosure
      defaultOpen={open}
      className="border-t border-line-soft pt-1"
      triggerProps={{ className: "w-full rounded-md px-2 py-1.5 hover:bg-surface-2" }}
      contentProps={{ className: "pt-0.5" }}
      size="md"
      label={
        <span className="flex min-w-0 flex-1 items-center gap-2">
          <ProviderLogo provider={sessionMCPRuntimeProvider(data.runtime)} className="size-3.5" />
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
          <li
            key={row.name}
            className={cn(ROW, row.state.startsWith("off") && "opacity-60")}
          >
            <span className={NAME}>{row.name}</span>
            {runtimeSide(row, runtime)}
          </li>
        ))}
      </ul>
      <p className="px-2 pt-1.5 pb-1 text-meta text-faint">
        {off
          ? `Turned off for CompozyOS sessions. Turn them on in Settings.`
          : `${runtime} loads these by itself. Change them in ${runtime}.`}
      </p>
    </Disclosure>
  );
}

function Body({ data, nested, runtimeOpen }: Required<Pick<ComposerMCPPopoverProps, "runtimeOpen">> & {
  data: SessionMCPResponse;
  nested?: ComposerNested;
}) {
  const reloadFailed = data.pending_reload && data.last_reload?.outcome === "failed";
  const usable = data.compozy_servers.filter(row => row.state !== "not_available");
  const sorted = [...usable].sort(
    (a, b) => Number(ATTENTION.has(b.state)) - Number(ATTENTION.has(a.state))
  );
  const unavailable = data.compozy_servers.filter(row => row.state === "not_available");
  return (
    <>
      {reloadFailed ? (
        <Alert variant="warning" className="mx-0.5">
          <AlertDescription>
            {data.runtime_display_name} couldn't start with the new servers. Send your message
            again to retry.
          </AlertDescription>
        </Alert>
      ) : null}
      <ul className="flex max-h-80 flex-col overflow-y-auto">
        {sorted.map(row => (
          <CompozyRow key={`${row.owner}:${row.name}`} row={row} nested={nested} />
        ))}
      </ul>
      {unavailable.length > 0 ? (
        <Disclosure
          label={`Off in this session (${unavailable.length})`}
          className="px-2"
          contentProps={{ className: "pt-0.5" }}
        >
          <p className="pb-1 text-meta text-faint">The session that started this one limits them.</p>
          <ul className="-mx-2 flex flex-col">
            {unavailable.map(row => (
              <CompozyRow key={row.name} row={row} />
            ))}
          </ul>
        </Disclosure>
      ) : null}
      {data.runtime_servers.servers.length > 0 ? (
        <RuntimeSection data={data} open={runtimeOpen} />
      ) : null}
    </>
  );
}

function LoadingRows() {
  return (
    <ul className="flex flex-col gap-1 px-2 py-1" aria-label="Loading MCP servers">
      {["w-28", "w-20", "w-32"].map(width => (
        <li key={width} className="flex h-6 items-center justify-between">
          <span className={cn("h-3 animate-pulse rounded-sm bg-surface-2", width)} />
          <span className="h-3 w-10 animate-pulse rounded-sm bg-surface-2" />
        </li>
      ))}
    </ul>
  );
}

export function ComposerMCPPopover({
  data,
  open = true,
  loading = false,
  stale = false,
  runtimeOpen = false,
  nested,
}: ComposerMCPPopoverProps) {
  const attention = data?.compozy_servers.some(row => ATTENTION.has(row.state)) ?? false;
  const pending = data?.pending_reload ?? false;
  const reloadFailed = pending && data?.last_reload?.outcome === "failed";
  return (
    <Popover defaultOpen={open}>
      <PopoverTrigger
        render={
          <InputGroupButton
            type="button"
            size="tool-icon"
            aria-label="MCP servers"
            className="relative shrink-0 data-[popup-open]:bg-surface-2"
          />
        }
      >
        <Plug aria-hidden="true" />
        {attention ? (
          <span className="absolute top-1 right-1 size-1.5 rounded-full bg-warning" />
        ) : null}
        {pending ? (
          <span className="absolute right-1 bottom-1 size-1.5 rounded-full bg-info" />
        ) : null}
      </PopoverTrigger>
      <PopoverContent side="top" align="start" className="w-80 max-w-[calc(100vw-2rem)] gap-0 p-0">
        <div className="flex items-center gap-2 border-b border-line-soft py-2 pr-2 pl-3.5">
          <Plug aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
          <PopoverTitle className="shrink-0 text-small-body font-medium whitespace-nowrap text-fg-strong">
            MCP servers
          </PopoverTitle>
          {stale ? <span className="truncate text-meta text-faint">May be out of date</span> : null}
          {pending && !stale && !reloadFailed ? (
            <span className="flex min-w-0 items-center gap-1.5 text-meta text-muted-foreground">
              <StatusDot tone="info" size="sm" />
              <span className="truncate">Updates on your next message</span>
            </span>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label="Manage MCP servers in Settings"
            title="Manage in Settings"
            className="ml-auto text-muted-foreground"
          >
            <Settings2 />
          </Button>
        </div>
        <div className="flex flex-col gap-1 p-1.5">
          {loading || data === null ? (
            <LoadingRows />
          ) : (
            <Body data={data} nested={nested} runtimeOpen={runtimeOpen} />
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
