import { Link } from "@tanstack/react-router";
import { ChevronRight, MessageSquare } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Empty, MetadataList, Panel, Pill, Skeleton, StateGlyph } from "@compozy/ui";

import { permissionLabel } from "../lib/agent-permissions";
import type { AgentPayload } from "../types";
import { AgentStatsGrid } from "./agent-stats-grid";
import { formatAgentRuntimeDuration } from "../lib/format-agent-runtime-duration";
import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";

export interface AgentOverviewTabProps {
  agent: AgentPayload;
  sessions: SessionPayload[];
  sessionsTotal: number;
  activeSessionsTotal: number;
  failedSessionsTotal: number | null;
  runtimeSeconds: number | null;
  metricsUnavailable: boolean;
  metricsLoading?: boolean;
  lastSessionActivityAt: string | null;
  sessionsLoading: boolean;
  sessionsError: boolean;
  /** Immediate Provider · Model · Reasoning control, mounted by the route. */
  runtimeControl: ReactNode;
  onEditRuntime: () => void;
  onViewAllSessions: () => void;
}

export function AgentOverviewTab({
  agent,
  sessions,
  sessionsTotal,
  activeSessionsTotal,
  failedSessionsTotal,
  runtimeSeconds,
  metricsUnavailable,
  metricsLoading = false,
  lastSessionActivityAt,
  sessionsLoading,
  sessionsError,
  runtimeControl,
  onEditRuntime,
  onViewAllSessions,
}: AgentOverviewTabProps) {
  const liveSessions = sessions.filter(session => session.state === "active").slice(0, 3);

  return (
    <div className="flex flex-col gap-6" data-testid="agent-overview-tab">
      {metricsLoading ? (
        <div
          className="grid grid-cols-2 gap-3 md:grid-cols-4"
          data-testid="agent-overview-metrics-skeleton"
        >
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-20 rounded-md" />
          ))}
        </div>
      ) : (
        <AgentStatsGrid
          active={activeSessionsTotal}
          runtimeLabel={runtimeSeconds === null ? null : formatAgentRuntimeDuration(runtimeSeconds)}
          failed={failedSessionsTotal}
          lastActivityAt={lastSessionActivityAt}
          sessionsTotal={sessionsTotal}
          metricsAvailable={!metricsUnavailable}
        />
      )}

      <Panel
        bodyClassName="p-0"
        data-testid="agent-overview-runtime"
        right={
          <Button
            data-testid="agent-overview-edit-runtime"
            onClick={onEditRuntime}
            size="sm"
            type="button"
            variant="ghost"
          >
            Edit
            <ChevronRight aria-hidden="true" data-icon="inline-end" />
          </Button>
        }
        title="Setup"
      >
        <MetadataList className="gap-0">
          <MetadataList.Row className={metadataRowClassName}>
            <MetadataList.Term id="agent-overview-model-label">Model</MetadataList.Term>
            <MetadataList.Value className="w-full text-fg">{runtimeControl}</MetadataList.Value>
          </MetadataList.Row>
          <MetadataList.Row className={metadataRowClassName}>
            <MetadataList.Term>Permissions</MetadataList.Term>
            <MetadataList.Value data-testid="agent-overview-permissions">
              {permissionLabel(agent.permissions)}
            </MetadataList.Value>
          </MetadataList.Row>
        </MetadataList>
      </Panel>

      <Panel
        bodyClassName="p-0"
        data-testid="agent-overview-live-sessions"
        right={
          <Button
            data-testid="agent-overview-view-all-sessions"
            onClick={onViewAllSessions}
            size="sm"
            type="button"
            variant="ghost"
          >
            View all
            <ChevronRight aria-hidden="true" data-icon="inline-end" />
          </Button>
        }
        title="Live sessions"
      >
        {sessionsLoading ? (
          <div className="flex flex-col gap-2 p-4">
            <Skeleton className="h-10 rounded-md" />
            <Skeleton className="h-10 rounded-md" />
          </div>
        ) : sessionsError ? (
          <p
            className="p-4 text-small-body text-muted"
            data-testid="agent-overview-sessions-notice"
          >
            Session status unavailable
          </p>
        ) : liveSessions.length === 0 ? (
          <Empty
            icon={MessageSquare}
            title="No active sessions"
            description="Start a session to see live work here."
            data-testid="agent-overview-no-live"
            fill={false}
            className="px-4 py-8"
          />
        ) : (
          <ul>
            {liveSessions.map(session => {
              const elapsed =
                typeof session.activity?.elapsed_seconds === "number"
                  ? formatElapsed(session.activity.elapsed_seconds)
                  : "";
              return (
                <li key={session.id} className="border-t border-line-soft first:border-t-0">
                  <Link
                    to="/agents/$name/sessions/$id"
                    params={{ name: agent.name, id: session.id }}
                    className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-4 py-3 hover:bg-hover"
                    data-testid={`agent-overview-live-${session.id}`}
                  >
                    <span className="min-w-0">
                      <span className="block truncate text-body font-medium text-fg">
                        {getSessionDisplayTitle(session)}
                      </span>
                      {elapsed ? (
                        <span className="mt-1 block text-small-body text-muted">{elapsed}</span>
                      ) : null}
                    </span>
                    <Pill size="sm" tone="neutral">
                      <StateGlyph state="running" size="sm" />
                      Active
                    </Pill>
                  </Link>
                </li>
              );
            })}
          </ul>
        )}
      </Panel>
    </div>
  );
}

const metadataRowClassName =
  "flex-col items-start gap-1.5 border-t border-line-soft px-4 py-3.5 first:border-t-0";

function formatElapsed(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) return "";
  if (totalSeconds < 1) return "0s";
  const total = Math.floor(totalSeconds);
  if (total < 60) return `${total}s`;
  const minutes = Math.floor(total / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainderMinutes = minutes % 60;
  return remainderMinutes === 0 ? `${hours}h` : `${hours}h ${remainderMinutes}m`;
}
