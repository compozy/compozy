import type { ReactElement } from "react";

import {
  Alert,
  EmptyState,
  SectionHeading,
  SkeletonRow,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { WorkflowSummary } from "../types";

export interface MemoryIndexViewProps {
  workflows: WorkflowSummary[];
  isLoading: boolean;
  isRefetching: boolean;
  error?: string | null;
  workspaceName: string;
}

export function MemoryIndexView(props: MemoryIndexViewProps): ReactElement {
  const { workflows, isLoading, isRefetching, error, workspaceName } = props;
  return (
    <div className="space-y-6" data-testid="memory-index-view">
      <SectionHeading
        description={`Each workflow in ${workspaceName} keeps its own memory store — a shared MEMORY.md and per-task notebooks written by agents.`}
        eyebrow="Across workspace"
        title="Memory"
      />

      {error ? (
        <Alert data-testid="memory-index-error" variant="error">
          {error}
        </Alert>
      ) : null}

      {isLoading ? (
        <div
          className="grid gap-3 md:grid-cols-2 xl:grid-cols-3"
          data-testid="memory-index-loading"
        >
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}

      {!isLoading && workflows.length === 0 && !error ? (
        <EmptyState
          data-testid="memory-index-empty"
          description="No workflows are registered in this workspace, so there are no memory notebooks to browse yet."
          title="No workflows yet"
        />
      ) : null}

      {workflows.length > 0 ? (
        <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3" data-testid="memory-index-list">
          {workflows.map(workflow => (
            <li key={workflow.id}>
              <SurfaceCard
                data-interactive="true"
                data-testid={`memory-index-card-${workflow.slug}`}
              >
                <SurfaceCardHeader>
                  <div>
                    <SurfaceCardEyebrow>Workflow</SurfaceCardEyebrow>
                    <SurfaceCardTitle>{workflow.slug}</SurfaceCardTitle>
                    <SurfaceCardDescription>
                      last synced {formatTimestamp(workflow.last_synced_at)}
                    </SurfaceCardDescription>
                  </div>
                  <StatusBadge tone={workflow.archived_at ? "neutral" : "success"}>
                    {workflow.archived_at ? "archived" : "live"}
                  </StatusBadge>
                </SurfaceCardHeader>
                <SurfaceCardBody>
                  <p className="text-sm text-muted-foreground">
                    Shared MEMORY.md plus per-task notebooks for{" "}
                    <code className="font-mono">{workflow.slug}</code>.
                  </p>
                </SurfaceCardBody>
                <SurfaceCardFooter>
                  <Link
                    className="text-xs font-semibold uppercase tracking-[0.12em] text-primary transition-colors hover:text-foreground"
                    data-testid={`memory-index-open-${workflow.slug}`}
                    params={{ slug: workflow.slug }}
                    to="/memory/$slug"
                  >
                    Open memory →
                  </Link>
                </SurfaceCardFooter>
              </SurfaceCard>
            </li>
          ))}
        </ul>
      ) : null}

      {isRefetching ? (
        <p className="text-xs text-muted-foreground" data-testid="memory-index-refreshing">
          refreshing…
        </p>
      ) : null}
    </div>
  );
}

function formatTimestamp(raw: string | undefined): string {
  if (!raw) {
    return "unknown";
  }
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
