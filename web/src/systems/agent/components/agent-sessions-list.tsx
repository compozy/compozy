import { Link } from "@tanstack/react-router";
import { ChevronRight, MessageSquare } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import {
  Empty,
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Eyebrow,
  Pill,
  Skeleton,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  cn,
  formatDuration as formatCanonicalDuration,
} from "@compozy/ui";

import { getAgentSessionStatus } from "../lib/session-status";
import {
  getSessionDisplayTitle,
  isSessionRunning,
  SessionRowActions,
  type SessionLifecycleActionHandlers,
  type SessionPayload,
} from "@/systems/session";

const RELATIVE_TIME_REFRESH_MS = 30_000;

export interface AgentSessionsListProps {
  agentName: string;
  sessions: SessionPayload[];
  archivedSessions?: SessionPayload[];
  archivedTotal?: number;
  status: "loading" | "error" | "ready";
  archivedPaginationStatus?: "available" | "loading";
  onLoadMoreArchived?: () => void;
  paginationStatus?: "available" | "loading";
  onLoadMore?: () => void;
  sessionActions?: SessionLifecycleActionHandlers;
  emptyTitle?: ReactNode;
  emptyDescription?: ReactNode;
  emptyAction?: ReactNode;
}

export function AgentSessionsList({
  agentName,
  sessions,
  archivedSessions = [],
  archivedTotal,
  status,
  archivedPaginationStatus,
  onLoadMoreArchived,
  paginationStatus,
  onLoadMore,
  sessionActions,
  emptyTitle = "No sessions yet",
  emptyDescription,
  emptyAction,
}: AgentSessionsListProps) {
  const resolvedEmptyDescription =
    emptyDescription === undefined
      ? `Start a new session for ${agentName} from the toolbar above.`
      : emptyDescription;
  return (
    <div className="flex flex-col gap-3">
      {status === "loading" ? <AgentSessionsSkeleton /> : null}
      {status === "error" ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
          <Empty
            icon={MessageSquare}
            title="Couldn't load sessions"
            description="The session list failed to load. Try refreshing the page."
            data-testid="agent-sessions-error"
            fill={false}
          />
        </div>
      ) : null}
      {status === "ready" && sessions.length === 0 && archivedSessions.length === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
          <Empty
            icon={MessageSquare}
            title={emptyTitle}
            description={resolvedEmptyDescription}
            action={emptyAction}
            data-testid="agent-sessions-empty"
            fill={false}
          />
        </div>
      ) : null}
      {status === "ready" && sessions.length > 0 ? (
        <AgentSessionsTable
          agentName={agentName}
          sessions={sessions}
          paginationStatus={paginationStatus}
          onLoadMore={onLoadMore}
          sessionActions={sessionActions}
        />
      ) : null}
      <ArchivedSessionsSection
        agentName={agentName}
        sessions={archivedSessions}
        total={archivedTotal}
        paginationStatus={archivedPaginationStatus}
        onLoadMore={onLoadMoreArchived}
        sessionActions={sessionActions}
      />
    </div>
  );
}

interface AgentSessionsTableProps {
  agentName: string;
  sessions: SessionPayload[];
  paginationStatus?: "available" | "loading";
  onLoadMore?: () => void;
  sessionActions?: SessionLifecycleActionHandlers;
}

function AgentSessionsTable({
  agentName,
  sessions,
  paginationStatus,
  onLoadMore,
  sessionActions,
}: AgentSessionsTableProps) {
  const [now, setNow] = useState(Date.now);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), RELATIVE_TIME_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, []);

  return (
    <div className="flex flex-col" data-testid="agent-sessions-table-wrapper">
      <div className="overflow-x-auto">
        <Table data-testid="agent-sessions-table">
          <TableHeader>
            <TableRow>
              <TableHead className="w-2/5">Session</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Duration</TableHead>
              <TableHead className="text-right">Iterations</TableHead>
              <TableHead className="text-right">Last activity</TableHead>
              <TableHead className="w-10">
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map(session => (
              <AgentSessionRow
                key={session.id}
                agentName={agentName}
                session={session}
                now={now}
                sessionActions={sessionActions}
              />
            ))}
          </TableBody>
        </Table>
      </div>
      {paginationStatus ? (
        <div className="flex justify-center border-t border-line px-4 py-3">
          <Button
            type="button"
            variant="neutral"
            size="sm"
            disabled={paginationStatus === "loading"}
            aria-busy={paginationStatus === "loading"}
            onClick={onLoadMore}
            data-testid="agent-sessions-load-more"
          >
            {paginationStatus === "loading" ? <Spinner aria-hidden="true" /> : null}
            {paginationStatus === "loading" ? "Loading more sessions" : "Load more sessions"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

interface AgentSessionRowProps {
  agentName: string;
  session: SessionPayload;
  now: number;
  sessionActions?: SessionLifecycleActionHandlers;
}

function AgentSessionRow({ agentName, session, now, sessionActions }: AgentSessionRowProps) {
  const status = getAgentSessionStatus(session);
  const running = isSessionRunning(session);
  const title = getSessionDisplayTitle(session);
  return (
    <TableRow data-testid={`agent-session-row-${session.id}`} data-state={status.kind}>
      <TableCell>
        <Link
          to="/agents/$name/sessions/$id"
          params={{ name: agentName, id: session.id }}
          className={cn(
            "text-item-title flex flex-col gap-0.5 text-fg",
            "transition-colors hover:text-accent"
          )}
          data-testid={`agent-session-link-${session.id}`}
        >
          <span className="truncate font-medium">{title}</span>
          <Eyebrow className="text-subtle">{session.runtime.effective?.provider}</Eyebrow>
        </Link>
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap justify-end gap-1">
          <Pill mono tone={status.tone} data-testid={`agent-session-status-${session.id}`}>
            {running ? <Spinner className="size-3" /> : null}
            {status.label}
          </Pill>
          {session.archived_at !== null ? (
            <Pill mono tone="neutral">
              ARCHIVED
            </Pill>
          ) : null}
        </div>
      </TableCell>
      <TableCell className="text-small-body text-right font-mono text-muted">
        {formatDuration(session.activity?.elapsed_seconds)}
      </TableCell>
      <TableCell className="text-small-body text-right font-mono text-muted">
        {formatIterations(session.activity?.iteration_current, session.activity?.iteration_max)}
      </TableCell>
      <TableCell className="text-small-body text-right font-mono text-muted">
        {formatRelativeTime(session.activity?.last_activity_at ?? session.updated_at, now)}
      </TableCell>
      <TableCell className="text-right">
        {sessionActions ? <SessionRowActions session={session} actions={sessionActions} /> : null}
      </TableCell>
    </TableRow>
  );
}

function ArchivedSessionsSection({
  agentName,
  sessions,
  total,
  paginationStatus,
  onLoadMore,
  sessionActions,
}: {
  agentName: string;
  sessions: SessionPayload[];
  total?: number;
  paginationStatus?: "available" | "loading";
  onLoadMore?: () => void;
  sessionActions?: SessionLifecycleActionHandlers;
}) {
  if (sessions.length === 0) return null;

  return (
    <Collapsible data-testid="agent-sessions-archived">
      <CollapsibleTrigger
        className="group/agent-sessions-archived"
        render={
          <Button
            type="button"
            variant="ghost"
            size="sm"
            aria-label={total === undefined ? "Archived sessions" : `Archived sessions (${total})`}
            className="w-full justify-start gap-2 px-2 text-small-body text-subtle"
          />
        }
      >
        <ChevronRight className="size-3 transition-transform group-data-panel-open/agent-sessions-archived:rotate-90" />
        Archived
        {total === undefined ? null : (
          <span className="font-mono text-micro text-faint">{total}</span>
        )}
      </CollapsibleTrigger>
      <CollapsibleContent className="pt-1">
        <AgentSessionsTable
          agentName={agentName}
          sessions={sessions}
          paginationStatus={paginationStatus}
          onLoadMore={onLoadMore}
          sessionActions={sessionActions}
        />
      </CollapsibleContent>
    </Collapsible>
  );
}

function AgentSessionsSkeleton() {
  return (
    <div
      className="flex flex-col gap-2 px-1 py-2"
      data-testid="agent-sessions-loading"
      role="status"
      aria-live="polite"
    >
      {AGENT_SESSION_SKELETON_IDS.map(id => (
        <Skeleton key={id} className="h-9 w-full rounded-md" />
      ))}
    </div>
  );
}

const AGENT_SESSION_SKELETON_IDS = [
  "agent-session-skeleton-1",
  "agent-session-skeleton-2",
  "agent-session-skeleton-3",
  "agent-session-skeleton-4",
];

function formatDuration(seconds: number | undefined | null): string {
  if (typeof seconds !== "number" || !Number.isFinite(seconds) || seconds < 0) return "--";
  return formatCanonicalDuration(Math.round(seconds) * 1_000);
}

function formatIterations(current: number | undefined, max: number | undefined): string {
  if (typeof current !== "number" || !Number.isFinite(current)) return "--";
  if (typeof max === "number" && Number.isFinite(max) && max > 0) {
    return `${current}/${max}`;
  }
  return `${current}`;
}

function formatRelativeTime(value: string | null | undefined, now: number): string {
  if (!value) return "--";
  const ts = new Date(value).getTime();
  if (!Number.isFinite(ts)) return "--";
  const diffMs = now - ts;
  if (diffMs < 0) return "just now";
  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 45) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;
  return new Date(ts).toLocaleDateString();
}
