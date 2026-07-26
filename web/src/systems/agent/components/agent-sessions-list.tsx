import { Link } from "@tanstack/react-router";
import { MessageSquare } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import {
  Empty,
  Button,
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
} from "@agh/ui";

import { getAgentSessionStatus } from "../lib/session-status";
import { getSessionDisplayTitle, isSessionRunning, type SessionPayload } from "@/systems/session";

const RELATIVE_TIME_REFRESH_MS = 30_000;

export interface AgentSessionsListProps {
  agentName: string;
  sessions: SessionPayload[];
  status: "loading" | "error" | "ready";
  paginationStatus?: "available" | "loading";
  onLoadMore?: () => void;
  emptyTitle?: ReactNode;
  emptyDescription?: ReactNode;
  emptyAction?: ReactNode;
}

export function AgentSessionsList({
  agentName,
  sessions,
  status,
  paginationStatus,
  onLoadMore,
  emptyTitle = "No sessions yet",
  emptyDescription,
  emptyAction,
}: AgentSessionsListProps) {
  const resolvedEmptyDescription =
    emptyDescription === undefined
      ? `Start a new session for ${agentName} from the toolbar above.`
      : emptyDescription;
  if (status === "loading") {
    return <AgentSessionsSkeleton />;
  }

  if (status === "error") {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
        <Empty
          icon={MessageSquare}
          title="Couldn't load sessions"
          description="The session list failed to load. Try refreshing the page."
          data-testid="agent-sessions-error"
          fill={false}
        />
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
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
    );
  }

  return (
    <AgentSessionsTable
      agentName={agentName}
      sessions={sessions}
      paginationStatus={paginationStatus}
      onLoadMore={onLoadMore}
    />
  );
}

interface AgentSessionsTableProps {
  agentName: string;
  sessions: SessionPayload[];
  paginationStatus?: "available" | "loading";
  onLoadMore?: () => void;
}

function AgentSessionsTable({
  agentName,
  sessions,
  paginationStatus,
  onLoadMore,
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
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map(session => (
              <AgentSessionRow key={session.id} agentName={agentName} session={session} now={now} />
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
}

function AgentSessionRow({ agentName, session, now }: AgentSessionRowProps) {
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
          <Eyebrow className="text-subtle">{session.provider}</Eyebrow>
        </Link>
      </TableCell>
      <TableCell>
        <Pill mono tone={status.tone} data-testid={`agent-session-status-${session.id}`}>
          {running ? <Spinner className="size-3" /> : null}
          {status.label}
        </Pill>
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
    </TableRow>
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
