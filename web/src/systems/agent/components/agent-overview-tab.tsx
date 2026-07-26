import { Link } from "@tanstack/react-router";
import { MessageSquare } from "lucide-react";
import type { ReactNode } from "react";

import { Empty, MetadataList, Pill, Section, Skeleton } from "@agh/ui";

import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";

import {
  formatAbsentOverride,
  formatPromptWordCount,
  formatSkillsPolicyLine,
} from "../lib/agent-absent-value";
import type { AgentPayload } from "../types";
import { AgentPanelBox } from "./agent-panel-box";
import { AgentStatsGrid } from "./agent-stats-grid";
import { formatAgentRuntimeDuration } from "../lib/format-agent-runtime-duration";

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
  const mcpNames = (agent.mcp_servers ?? []).map(server => server.name);
  const toolsCount = agent.tools?.length ?? 0;
  const denyCount = agent.deny_tools?.length ?? 0;
  const toolsetsCount = agent.toolsets?.length ?? 0;

  return (
    <div className="flex flex-col gap-6" data-testid="agent-overview-tab">
      {sessionsError ? (
        <p
          className="text-small-body text-muted"
          role="status"
          data-testid="agent-overview-sessions-notice"
        >
          Session status unavailable
        </p>
      ) : null}
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

      <div className="grid gap-7 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,20rem)]">
        <div className="flex flex-col gap-6">
          <Section
            className="gap-0"
            label="Runtime"
            right={
              <button
                type="button"
                className="text-small-body text-muted hover:text-fg"
                onClick={onEditRuntime}
                data-testid="agent-overview-edit-runtime"
              >
                Edit ›
              </button>
            }
            data-testid="agent-overview-runtime"
          >
            <AgentPanelBox>
              <MetadataList className="gap-0">
                <MetadataList.Row className={metadataRowClassName}>
                  <MetadataList.Term id="agent-overview-model-label">Model</MetadataList.Term>
                  <MetadataList.Value className="w-full text-fg">
                    {runtimeControl}
                  </MetadataList.Value>
                </MetadataList.Row>
                <MetadataList.Row className={metadataRowClassName}>
                  <MetadataList.Term>Command</MetadataList.Term>
                  <MetadataList.Value className={agent.command ? "font-mono" : "text-muted"}>
                    {formatAbsentOverride(agent.command)}
                  </MetadataList.Value>
                </MetadataList.Row>
                <MetadataList.Row className={metadataRowClassName}>
                  <MetadataList.Term>Permissions</MetadataList.Term>
                  <MetadataList.Value
                    className={agent.permissions?.trim() ? "font-mono" : "text-muted"}
                  >
                    {agent.permissions?.trim() || "Default"}
                  </MetadataList.Value>
                </MetadataList.Row>
              </MetadataList>
            </AgentPanelBox>
          </Section>

          <Section
            className="gap-0"
            label="Live sessions"
            data-testid="agent-overview-live-sessions"
            right={
              <button
                type="button"
                className="text-small-body text-muted hover:text-fg"
                onClick={onViewAllSessions}
                data-testid="agent-overview-view-all-sessions"
              >
                View all ›
              </button>
            }
          >
            <AgentPanelBox>
              {sessionsLoading ? (
                <div className="flex flex-col gap-2 p-4">
                  <Skeleton className="h-10 rounded-md" />
                  <Skeleton className="h-10 rounded-md" />
                </div>
              ) : sessionsError ? (
                <p className="p-4 text-small-body text-muted">Session status unavailable</p>
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
                    const iter =
                      session.activity?.iteration_current != null &&
                      session.activity?.iteration_max != null
                        ? `${session.activity.iteration_current}/${session.activity.iteration_max} iter`
                        : null;
                    const elapsed =
                      typeof session.activity?.elapsed_seconds === "number"
                        ? formatElapsed(session.activity.elapsed_seconds)
                        : null;
                    const detail = [iter, elapsed].filter(Boolean).join(" · ");
                    return (
                      <li key={session.id} className="border-t border-line-soft first:border-t-0">
                        <Link
                          to="/agents/$name/sessions/$id"
                          params={{ name: agent.name, id: session.id }}
                          className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-4 py-3 hover:bg-hover"
                          data-testid={`agent-overview-live-${session.id}`}
                        >
                          <span className="min-w-0">
                            <span className="block truncate text-body font-medium text-fg-strong">
                              {getSessionDisplayTitle(session)}
                            </span>
                            {detail ? (
                              <span className="mt-1 block font-mono text-badge tracking-mono text-muted">
                                {detail}
                              </span>
                            ) : null}
                          </span>
                          <Pill size="sm" tone="success">
                            <Pill.Dot size="sm" tone="success" />
                            Active
                          </Pill>
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              )}
            </AgentPanelBox>
          </Section>
        </div>

        <Section className="gap-0" label="At a glance" data-testid="agent-overview-glance">
          <AgentPanelBox>
            <MetadataList className="gap-0">
              <MetadataList.Row className={metadataRowClassName}>
                <MetadataList.Term>MCP servers</MetadataList.Term>
                <MetadataList.Value>
                  {mcpNames.length === 0 ? (
                    <span className="text-muted">None</span>
                  ) : (
                    <span className="inline-flex flex-wrap items-center gap-x-1.5 gap-y-1">
                      {mcpNames.map((name, index) => (
                        <span key={name} className="inline-flex items-center gap-1.5">
                          {index > 0 ? <span className="text-muted">·</span> : null}
                          <code className="font-mono text-badge tracking-mono text-fg">{name}</code>
                        </span>
                      ))}
                    </span>
                  )}
                </MetadataList.Value>
              </MetadataList.Row>
              <MetadataList.Row className={metadataRowClassName}>
                <MetadataList.Term>Tools</MetadataList.Term>
                <MetadataList.Value>
                  {toolsCount} allow · {denyCount} deny
                </MetadataList.Value>
              </MetadataList.Row>
              <MetadataList.Row className={metadataRowClassName}>
                <MetadataList.Term>Toolsets</MetadataList.Term>
                <MetadataList.Value className={toolsetsCount === 0 ? "text-muted" : undefined}>
                  {toolsetsCount === 0 ? "None" : String(toolsetsCount)}
                </MetadataList.Value>
              </MetadataList.Row>
              <MetadataList.Row className={metadataRowClassName}>
                <MetadataList.Term>Prompt</MetadataList.Term>
                <MetadataList.Value>{formatPromptWordCount(agent.prompt)}</MetadataList.Value>
              </MetadataList.Row>
              <MetadataList.Row className={metadataRowClassName}>
                <MetadataList.Term>Skills</MetadataList.Term>
                <MetadataList.Value data-testid="agent-overview-skills">
                  {formatSkillsPolicyLine(agent.skills)}
                </MetadataList.Value>
              </MetadataList.Row>
            </MetadataList>
          </AgentPanelBox>
        </Section>
      </div>
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
