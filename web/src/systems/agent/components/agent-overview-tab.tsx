import { Link } from "@tanstack/react-router";
import { ChevronRight, MessageSquare } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  Empty,
  MetadataList,
  Panel,
  Pill,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  StateGlyph,
  cn,
} from "@compozy/ui";

import { permissionLabel } from "../lib/agent-permissions";
import type { AgentPayload } from "../types";
import { AgentStatsGrid, AgentStatsGridSkeleton } from "./agent-stats-grid";
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
        <SkeletonRegion data-testid="agent-overview-metrics-skeleton" label="Loading agent metrics">
          <AgentStatsGridSkeleton />
        </SkeletonRegion>
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
            // Optical: the trailing chevron lands on the card's content edge.
            className="-mr-2"
            data-testid="agent-overview-edit-runtime"
            onClick={onEditRuntime}
            size="sm"
            type="button"
            variant="link"
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
            // Optical: the trailing chevron lands on the card's content edge.
            className="-mr-2"
            data-testid="agent-overview-view-all-sessions"
            onClick={onViewAllSessions}
            size="sm"
            type="button"
            variant="link"
          >
            View all
            <ChevronRight aria-hidden="true" data-icon="inline-end" />
          </Button>
        }
        title="Live sessions"
      >
        {sessionsLoading ? (
          <SkeletonRegion data-testid="agent-overview-sessions-skeleton" label="Loading sessions">
            <LiveSessionRowsSkeleton />
          </SkeletonRegion>
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
            size="compact"
            className="px-4 py-6"
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
                    className={cn(LIVE_SESSION_ROW_CLASS, "hover:bg-hover")}
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
                    <Pill form="plain">
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

const LIVE_SESSION_ROW_CLASS = "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-4 py-3";
// One row: most agents have a single live session or none, so the panel is not overshot.
const LIVE_SESSION_SKELETON_IDS = ["live-session-skeleton-1"] as const;

/** A live-session row in the real row grid; the same list renders while the page loads. */
function LiveSessionRowsSkeleton() {
  return (
    <ul>
      {LIVE_SESSION_SKELETON_IDS.map(id => (
        <li
          aria-hidden="true"
          className="animate-skeleton-reveal border-t border-line-soft first:border-t-0"
          key={id}
        >
          <div className={LIVE_SESSION_ROW_CLASS}>
            <span className="min-w-0">
              <span className="block text-body font-medium">
                <SkeletonText className="w-48" />
              </span>
              <span className="mt-1 block text-small-body">
                <SkeletonText className="w-10" />
              </span>
            </span>
            <Skeleton className="h-pill-sm w-16 rounded-pill" />
          </div>
        </li>
      ))}
    </ul>
  );
}

/**
 * Overview while the agent itself loads: the real tile grid and the two panels, with the head
 * actions and values held by placeholders. Wrap in one `SkeletonRegion`.
 */
export function AgentOverviewSkeleton() {
  return (
    <div className="flex flex-col gap-6" data-testid="agent-overview-skeleton">
      <AgentStatsGridSkeleton />
      <Panel bodyClassName="p-0" right={<Skeleton className="h-button-sm w-12" />} title="Setup">
        <MetadataList className="gap-0">
          <MetadataList.Row className={metadataRowClassName}>
            <MetadataList.Term>Model</MetadataList.Term>
            <MetadataList.Value className="w-full">
              <Skeleton className="h-button-lg w-56" />
            </MetadataList.Value>
          </MetadataList.Row>
          <MetadataList.Row className={metadataRowClassName}>
            <MetadataList.Term>Permissions</MetadataList.Term>
            <MetadataList.Value>
              <SkeletonText className="w-32" />
            </MetadataList.Value>
          </MetadataList.Row>
        </MetadataList>
      </Panel>
      <Panel
        bodyClassName="p-0"
        right={<Skeleton className="h-button-sm w-16" />}
        title="Live sessions"
      >
        <LiveSessionRowsSkeleton />
      </Panel>
    </div>
  );
}

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
