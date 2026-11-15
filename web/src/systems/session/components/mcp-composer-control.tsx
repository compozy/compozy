import { Plug } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  cn,
  Disclosure,
  InputGroupButton,
  Pill,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@compozy/ui";

import { ProviderLogo } from "@/systems/settings";

import type {
  SessionMCPResponse,
  SessionMCPServerRow,
  SessionRuntimeMCPRow,
} from "../adapters/session-mcp-api";
import {
  sessionMCPControlLabel,
  sessionMCPControlView,
  sessionMCPHeaderCaption,
  sessionMCPRowDetail,
  sessionMCPRuntimeProvider,
  sessionMCPStateView,
  type SessionMCPRowAction,
  type SessionMCPSummary,
} from "../lib/session-mcp-view";
import { ComposerPopoverRowsSkeleton } from "./composer-popover-rows-skeleton";

// The row's geometry, shared by the live rows and their placeholders so the two cannot drift.
const ROW_FRAME = "flex items-center gap-2 rounded-md px-0.5 py-1";
const ROW_TEXT_FRAME = "flex min-w-0 flex-1 flex-col gap-0.5";
const TITLE_WIDTHS = ["w-24", "w-32", "w-20"] as const;
const META_WIDTHS = ["w-16", "w-20", "w-14"] as const;

const ACTION_LABELS: Record<SessionMCPRowAction, string> = {
  sign_in: "Sign in",
  allow: "Allow",
};

export interface MCPComposerControlProps {
  /** The session payload's `mcp_summary`; the control is absent without one. */
  summary: SessionMCPSummary | null;
  /** The session MCP read, or `null` before it answered. */
  data: SessionMCPResponse | null;
  loading?: boolean;
  /** The latest read failed; rows from an earlier read stay, marked as possibly out of date. */
  failed?: boolean;
  onRetry?: () => void;
  onAction?: (row: SessionMCPServerRow, action: SessionMCPRowAction) => void;
  /** The server whose action is in flight. */
  actingName?: string | null;
  /** Actions wait until their target can be resolved. */
  actionsDisabled?: boolean;
  onManage?: () => void;
  /** The popover opened or closed; the owner reads the session's servers while it is open. */
  onOpenChange?: (open: boolean) => void;
  defaultOpen?: boolean;
}

interface RowProps {
  name: string;
  state: string;
  runtime: string;
  detail?: string;
  action?: ReactNode;
}

function ServerRow({ name, state, runtime, detail, action }: RowProps) {
  const view = sessionMCPStateView(state, runtime);
  return (
    <li className={ROW_FRAME} data-testid={`composer-mcp-${name}`} data-state={state}>
      <span className={cn(ROW_TEXT_FRAME, view.quiet && "opacity-60")}>
        <span className="truncate font-mono text-transcript-meta text-fg">{name}</span>
        {detail ? (
          <span className="truncate text-transcript-caption text-muted-foreground">{detail}</span>
        ) : null}
      </span>
      {action}
      <Pill form="plain" size="sm" tone={view.tone}>
        <Pill.Dot />
        {view.label}
      </Pill>
    </li>
  );
}

function CompozyRow({
  row,
  runtime,
  onAction,
  acting,
}: {
  row: SessionMCPServerRow;
  runtime: string;
  onAction?: MCPComposerControlProps["onAction"];
  acting: boolean;
}) {
  const action = sessionMCPStateView(row.state, runtime).action;
  return (
    <ServerRow
      name={row.name}
      state={row.state}
      runtime={runtime}
      detail={sessionMCPRowDetail(row)}
      action={
        action && onAction ? (
          <Button
            size="xs"
            type="button"
            variant="neutral"
            disabled={acting}
            aria-label={`${ACTION_LABELS[action]} ${row.name}`}
            onClick={() => onAction(row, action)}
          >
            {ACTION_LABELS[action]}
          </Button>
        ) : null
      }
    />
  );
}

function MCPControlSection({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-0.5">
      <span className="flex items-center gap-1.5 px-0.5 pb-0.5 text-eyebrow text-faint">
        {label}
      </span>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </section>
  );
}

function LoadError({ onRetry }: { onRetry?: () => void }) {
  return (
    <div className="flex items-center justify-between gap-2 px-0.5">
      <p className="text-transcript-caption text-muted-foreground">Couldn't load MCP servers.</p>
      {onRetry ? (
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

function RuntimeRows({
  rows,
  runtime,
}: {
  rows: readonly SessionRuntimeMCPRow[];
  runtime: string;
}) {
  return rows.map(row => (
    <ServerRow key={row.name} name={row.name} state={row.state} runtime={runtime} />
  ));
}

function MCPComposerBody({
  summary,
  data,
  loading = false,
  failed = false,
  onRetry,
  onAction,
  actingName = null,
  actionsDisabled = false,
}: Omit<MCPComposerControlProps, "onOpenChange" | "onManage" | "defaultOpen"> & {
  summary: SessionMCPSummary;
}) {
  if (data === null) {
    // Without an answer there is nothing to call ready: a failed read says so, never a pill.
    if (failed) return <LoadError onRetry={onRetry} />;
    if (!loading) return null;
    return (
      <ComposerPopoverRowsSkeleton
        label="Loading MCP servers"
        data-testid="composer-mcp-loading"
        count={Math.max(summary.usable + summary.attention, 1)}
        rowClassName={ROW_FRAME}
        textClassName={ROW_TEXT_FRAME}
        titleWidths={TITLE_WIDTHS}
        metaWidths={META_WIDTHS}
      />
    );
  }
  const view = sessionMCPControlView(data);
  const runtime = view.runtimeName;
  return (
    <div className="flex max-h-96 flex-col gap-3 overflow-y-auto">
      {view.reloadFailed ? (
        <Alert variant="warning">
          <AlertDescription>
            Couldn't start {runtime} with the new MCP servers. Your message wasn't sent; send it
            again to retry.
          </AlertDescription>
        </Alert>
      ) : null}
      {view.compozy.length > 0 ? (
        <MCPControlSection label="CompozyOS">
          {view.compozy.map(row => (
            <CompozyRow
              key={`${row.owner}:${row.name}`}
              row={row}
              runtime={runtime}
              onAction={onAction}
              acting={actionsDisabled || actingName === row.name}
            />
          ))}
        </MCPControlSection>
      ) : null}
      {view.unavailable.length > 0 ? (
        <Disclosure label={`Not available in this session (${view.unavailable.length})`}>
          <ul className="flex flex-col gap-0.5 pt-1">
            {view.unavailable.map(row => (
              <CompozyRow
                key={`${row.owner}:${row.name}`}
                row={row}
                runtime={runtime}
                acting={false}
              />
            ))}
          </ul>
        </Disclosure>
      ) : null}
      {view.runtimeServers.length > 0 ? (
        <MCPControlSection
          label={
            <>
              <ProviderLogo
                className="size-3.5"
                provider={sessionMCPRuntimeProvider(view.runtimeId)}
              />
              {runtime}'s own servers
            </>
          }
        >
          <RuntimeRows rows={view.runtimeServers} runtime={runtime} />
        </MCPControlSection>
      ) : null}
    </div>
  );
}

/**
 * The composer's MCP control (`mcp-extension-hot-activation` S3). The session payload's summary
 * drives the trigger at no cost; the popover lists what this session can use, what needs the
 * operator, and what the coding agent loads on its own. Absent when the session has no servers.
 */
export function MCPComposerControl({
  onOpenChange,
  onManage,
  defaultOpen,
  ...body
}: MCPComposerControlProps) {
  const { summary, data, failed = false } = body;
  if (summary === null) return null;
  const reloadFailed = data?.last_reload?.outcome === "failed";
  const warn = summary.attention > 0 || reloadFailed;
  const label = sessionMCPControlLabel(summary, { reloadFailed });
  // Icon-only and neutral like its toolbar neighbours; the name and tooltip carry the counts and
  // states. Two independent dots repeat them visually: attention top-right, a pending change
  // bottom-right.
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
      <PopoverContent side="top" align="start" className="w-96 max-w-[calc(100vw-2rem)] gap-2">
        <div className="flex items-baseline gap-2 px-0.5">
          <PopoverTitle className="text-transcript-meta font-semibold text-fg-strong">
            MCP servers
          </PopoverTitle>
          <span className="ml-auto text-transcript-caption text-faint">
            {sessionMCPHeaderCaption({
              stale: failed && data !== null,
              pendingReload: data?.pending_reload ?? summary.pending_reload,
            })}
          </span>
        </div>
        <MCPComposerBody {...body} summary={summary} />
        {onManage ? (
          <div className="flex justify-end border-t border-line-soft pt-2">
            <Button size="xs" type="button" variant="ghost" onClick={onManage}>
              Manage in Settings
            </Button>
          </div>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}
